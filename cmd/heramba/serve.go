package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Armur-Ai/heramba/internal/alert"
	"github.com/Armur-Ai/heramba/internal/config"
	"github.com/Armur-Ai/heramba/internal/edge"
	"github.com/Armur-Ai/heramba/internal/evidence"
	"github.com/Armur-Ai/heramba/internal/intel"
)

// runServe loads config, builds the edge engine, and serves until interrupted.
func runServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	configPath := fs.String("config", ".heramba.yml", "path to config file")
	mode := fs.String("mode", "", "override mode: observe|protect")
	listen := fs.String("listen", "", "override listen address, e.g. :8080")
	upstream := fs.String("upstream", "", "override upstream, e.g. http://127.0.0.1:3000")
	_ = fs.Parse(args)

	cfg := config.Default()
	if c, err := config.Load(*configPath); err == nil {
		cfg = c
	} else if !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "heramba: %v\n", err)
		os.Exit(1)
	}
	if *mode != "" {
		cfg.Mode = config.Mode(*mode)
	}
	if *listen != "" {
		cfg.Listen = *listen
	}
	if *upstream != "" {
		cfg.Upstream = *upstream
	}
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "heramba: %v\n", err)
		os.Exit(1)
	}

	// Fan the live detection stream into the operator's alert output, the
	// intelligence plane (evidence store + campaign reconstruction), and the
	// strategy engine running inline in SHADOW mode (proposes bounded actions,
	// never enforces).
	ingestor := intel.NewIngestor(evidence.NewStore(), "edge")
	advisor := newShadowAdvisor()
	counting := alert.Count(alert.MultiSink{buildSink(cfg), ingestor, advisor})
	engine, err := edge.New(cfg, counting)
	if err != nil {
		fmt.Fprintf(os.Stderr, "heramba: %v\n", err)
		os.Exit(1)
	}

	srv := &http.Server{
		Addr:              cfg.Listen,
		Handler:           engine.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	upstreamNote := cfg.Upstream
	if upstreamNote == "" {
		upstreamNote = "(honeypot-only, no upstream)"
	}
	fmt.Printf("heramba edge — mode=%s listen=%s upstream=%s\n", cfg.Mode, cfg.Listen, upstreamNote)
	if cfg.Mode == config.ModeObserve {
		fmt.Println("  OBSERVE mode: detection only, real traffic passes through untouched.")
	} else {
		fmt.Println("  PROTECT mode: inline — response normalization + tarpit on flagged sources.")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Fprintf(os.Stderr, "heramba: serve error: %v\n", err)
			stop()
		}
	}()
	<-ctx.Done()
	fmt.Println("\nheramba: shutting down…")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)

	if counting.Total() == 0 {
		fmt.Println("heramba: no detections this session.")
		return
	}
	fmt.Printf("heramba: %d detection(s) this session:\n", counting.Total())
	for _, t := range counting.Snapshot() {
		fmt.Printf("  %-18s %d\n", t.Kind, t.Count)
	}

	// Intelligence rollup: what the correlator made of the session.
	if s := ingestor.Summarize(); s.Events > 0 {
		fmt.Printf("heramba intelligence: %d event(s) → %d session(s), %d actor(s), %d campaign(s)\n",
			s.Events, s.Sessions, s.Actors, s.Campaigns)
		for class, n := range s.SwarmClasses {
			fmt.Printf("  %-22s %d actor(s)\n", class, n)
		}
	}

	// Strategy rollup: what the engine WOULD propose (shadow mode — nothing was
	// enforced). Each line is a policy-validated, bounded, reversible action an
	// operator could approve; this is the opt-in bridge from detection to action.
	if proposals := advisor.Propose(); len(proposals) > 0 {
		enforcing := 0
		for _, ex := range proposals {
			if ex.Decision.Verdict != "deny" {
				enforcing++
			}
		}
		fmt.Printf("heramba strategy (shadow — not enforced): %d proposed action(s), %d would clear policy\n", len(proposals), enforcing)
		for _, ex := range proposals {
			kind, id := ex.Action.Scope.Subject()
			fmt.Printf("  %-18s %s=%s  impact=%-6s verdict=%s\n",
				ex.Action.Type, kind, id, ex.Action.Impact, ex.Decision.Verdict)
		}
		fmt.Println("  (shadow mode: validate + audit only. Enable the engine in enforce mode to act, with approval gates.)")
	}
}

// buildSink wires the configured alert output.
func buildSink(cfg config.Config) alert.Sink {
	w := os.Stdout
	if cfg.Alert.File != "" {
		f, err := os.OpenFile(cfg.Alert.File, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err == nil {
			if cfg.Alert.Format == "jsonl" {
				return alert.NewJSONL(f)
			}
			return alert.NewText(f)
		}
		fmt.Fprintf(os.Stderr, "heramba: cannot open alert file %q, using stdout: %v\n", cfg.Alert.File, err)
	}
	if cfg.Alert.Format == "jsonl" {
		return alert.NewJSONL(w)
	}
	return alert.NewText(w)
}
