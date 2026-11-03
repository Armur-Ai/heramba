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
	"github.com/Armur-Ai/heramba/internal/deception"
	"github.com/Armur-Ai/heramba/internal/edge"
	"github.com/Armur-Ai/heramba/internal/event"
	"github.com/Armur-Ai/heramba/internal/evidence"
	"github.com/Armur-Ai/heramba/internal/intel"
)

// runHoneypot serves the stateful deception app behind the edge, with the
// edge's decoys/canaries in front and non-decoy paths answered by a coherent
// synthetic application. Every interaction — edge detections AND deception-app
// touches — flows into one evidence store and is reconstructed at shutdown.
func runHoneypot(args []string) {
	fs := flag.NewFlagSet("honeypot", flag.ExitOnError)
	listen := fs.String("listen", "127.0.0.1:8080", "listen address")
	seed := fs.Int64("seed", 1, "seed for the deterministic synthetic world")
	format := fs.String("alert", "text", "alert format: text|jsonl")
	_ = fs.Parse(args)

	// Build the synthetic world + deception app.
	world := deception.GenerateWorld(*seed, 8, 6, 20)
	app := deception.NewApp(world)

	// One shared evidence store fed by both planes.
	store := evidence.NewStore()
	ingestor := intel.NewIngestor(store, "edge")

	// Deception-app interactions → evidence (a synthetic-app touch is, by
	// construction, an attacker interaction).
	app.OnInteraction = func(in deception.Interaction) {
		kind := event.KindProbe
		sev := event.SeverityLow
		switch {
		case in.CanaryHit != "":
			ev := event.Deception(event.KindCanaryTriggered, in.SourceKey, in.CanaryHit, "synthetic token used in deception app")
			_ = ingestWith(store, ev)
			return
		case in.Note == "authz_violation":
			kind, sev = event.KindProbe, event.SeverityMedium
		}
		ev := event.New(kind, sev, in.SourceKey, in.Path, "deception-app "+in.Method+" "+in.Path).
			WithLabel("note", in.Note).WithLabel("sensor", "deception")
		_ = ingestWith(store, ev)
	}

	// Edge in front: observe mode, honeypot-only, with the deception app as the
	// fallthrough handler for non-decoy paths.
	cfg := config.Default()
	cfg.Mode = config.ModeObserve
	cfg.Listen = *listen
	cfg.Upstream = ""
	cfg.Alert.Format = *format
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "heramba: %v\n", err)
		os.Exit(1)
	}
	counting := alert.Count(alert.MultiSink{buildSink(cfg), ingestor})
	engine, err := edge.New(cfg, counting)
	if err != nil {
		fmt.Fprintf(os.Stderr, "heramba: %v\n", err)
		os.Exit(1)
	}
	engine.SetHoneypot(app)

	srv := &http.Server{Addr: cfg.Listen, Handler: engine.Handler(), ReadHeaderTimeout: 10 * time.Second}
	fmt.Printf("heramba honeypot — listen=%s · deception world seed=%d (%d users, %d projects)\n",
		cfg.Listen, *seed, len(world.Users), len(world.Projects))
	fmt.Println("  decoys/canaries fire in front; non-decoy paths get a coherent synthetic app.")
	fmt.Println("  try: POST /api/login {\"username\":\"<user>\"}, then GET /api/projects")

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

	if s := ingestor.Summarize(); s.Events > 0 {
		fmt.Printf("heramba intelligence: %d event(s) → %d session(s), %d actor(s), %d campaign(s)\n",
			s.Events, s.Sessions, s.Actors, s.Campaigns)
		for class, n := range s.SwarmClasses {
			fmt.Printf("  %-22s %d actor(s)\n", class, n)
		}
	} else {
		fmt.Println("heramba: no interactions this session.")
	}
}

// ingestWith stores a lifted event directly (shared store), mirroring what the
// intel ingestor does for edge events.
func ingestWith(store *evidence.Store, e event.Event) error {
	se := evidence.FromDetection(e, firstLabel(e, "sensor", "deception"), "")
	if se.Validate() != nil {
		return nil
	}
	return store.PutEvent(se)
}

func firstLabel(e event.Event, key, def string) string {
	if e.Labels != nil {
		if v, ok := e.Labels[key]; ok && v != "" {
			return v
		}
	}
	return def
}
