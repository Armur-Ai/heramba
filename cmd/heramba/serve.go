package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/Armur-Ai/heramba/internal/admin"
	"github.com/Armur-Ai/heramba/internal/alert"
	"github.com/Armur-Ai/heramba/internal/config"
	"github.com/Armur-Ai/heramba/internal/deception"
	"github.com/Armur-Ai/heramba/internal/decoy"
	"github.com/Armur-Ai/heramba/internal/edge"
	"github.com/Armur-Ai/heramba/internal/event"
	"github.com/Armur-Ai/heramba/internal/evidence"
	"github.com/Armur-Ai/heramba/internal/intel"
	"github.com/Armur-Ai/heramba/internal/openapi"
)

// runServe loads config, builds the edge engine, and serves until interrupted.
func runServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	configPath := fs.String("config", ".heramba.yml", "path to config file")
	mode := fs.String("mode", "", "override mode: observe|protect")
	listen := fs.String("listen", "", "override listen address, e.g. :8080")
	upstream := fs.String("upstream", "", "override upstream, e.g. http://127.0.0.1:3000")
	adminListen := fs.String("admin-listen", "", "health/metrics address, e.g. :9090 (empty=disabled)")
	preset := fs.String("preset", "", "framework decoy preset (run 'heramba decoys presets' for the full list)")
	deceptionOn := fs.Bool("deception", false, "when there is no upstream, auto-mount the synthetic deception app for non-decoy paths")
	deceptionSeed := fs.Int64("deception-seed", 1, "seed for the deterministic synthetic world")
	_ = fs.Parse(args)

	cfg := config.Default()
	if c, err := config.Load(*configPath); err == nil {
		cfg = c
	} else if !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "heramba: %v\n", err)
		os.Exit(1)
	}
	// Env overlay sits between the file and explicit flags: flags > env > file >
	// defaults. This is what lets `docker run -e HERAMBA_UPSTREAM= -e HERAMBA_MODE=`
	// configure the edge with no config file at all.
	cfg = cfg.ApplyEnv()
	if *mode != "" {
		cfg.Mode = config.Mode(*mode)
	}
	if *listen != "" {
		cfg.Listen = *listen
	}
	if *upstream != "" {
		cfg.Upstream = *upstream
	}
	if *preset != "" {
		cfg.Decoys.Preset = *preset
	}
	if *adminListen != "" {
		cfg.AdminListen = *adminListen
	}
	if cfg.Decoys.Preset != "" {
		if _, ok := decoy.GetPreset(cfg.Decoys.Preset); !ok {
			fmt.Fprintf(os.Stderr, "heramba: unknown preset %q (have: %v)\n", cfg.Decoys.Preset, decoy.PresetNames())
			os.Exit(1)
		}
	}
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "heramba: %v\n", err)
		os.Exit(1)
	}

	// Seed request-body canaries from an OpenAPI spec, if configured.
	if augmented, n, err := applyOpenAPICanaries(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "heramba: openapi canaries disabled (%v)\n", err)
	} else if n > 0 {
		cfg = augmented
		fmt.Printf("  OPENAPI: watching %d request-body canary field(s) from %s\n", n, cfg.Decoys.OpenAPISpec)
	}

	// Fan the live detection stream into the operator's alert output, the
	// intelligence plane (evidence store + campaign reconstruction), and the
	// strategy engine running inline in SHADOW mode (proposes bounded actions,
	// never enforces).
	store := evidence.NewStore()
	ingestor := intel.NewIngestor(store, "edge")
	advisor := newShadowAdvisor()
	// The alert wiring sits behind a SwapSink so SIGHUP can replace it live
	// without rebuilding the counting sink (which feeds metrics + the summary).
	alertSink := alert.NewSwap(buildSink(cfg))
	// Ring buffer of recent events for the admin events API + web dashboard.
	ring := alert.NewRing(200)
	counting := alert.Count(alert.MultiSink{alertSink, ingestor, advisor, ring})
	engine, err := edge.New(cfg, counting)
	if err != nil {
		fmt.Fprintf(os.Stderr, "heramba: %v\n", err)
		os.Exit(1)
	}

	// Opt-in: auto-mount the synthetic deception app for non-decoy paths when
	// running honeypot-only (no upstream). Its interactions feed the same
	// evidence store, so the intelligence rollup sees deception-app touches too.
	if *deceptionOn {
		if cfg.Upstream != "" {
			fmt.Fprintln(os.Stderr, "heramba: --deception ignored because an upstream is configured (it only mounts honeypot-only)")
		} else {
			world := deception.GenerateWorld(*deceptionSeed, 8, 6, 20)
			app := deception.NewApp(world)
			app.OnInteraction = func(in deception.Interaction) {
				if in.CanaryHit != "" {
					_ = ingestWith(store, event.Deception(event.KindCanaryTriggered, in.SourceKey, in.CanaryHit, "synthetic token used in deception app"))
					return
				}
				sev := event.SeverityLow
				if in.Note == "authz_violation" {
					sev = event.SeverityMedium
				}
				_ = ingestWith(store, event.New(event.KindProbe, sev, in.SourceKey, in.Path, "deception-app "+in.Method+" "+in.Path).
					WithLabel("note", in.Note).WithLabel("sensor", "deception"))
			}
			engine.SetHoneypot(app)
			fmt.Printf("  DECEPTION: synthetic app mounted (seed=%d, %d users, %d projects) for non-decoy paths.\n",
				*deceptionSeed, len(world.Users), len(world.Projects))
		}
	}

	startTime := time.Now()
	srv := &http.Server{
		Addr:              cfg.Listen,
		Handler:           engine.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Optional admin server (health + metrics) on a separate port.
	var adminSrv *http.Server
	var ready atomic.Bool
	if cfg.AdminListen != "" {
		adminSrv = &http.Server{
			Addr: cfg.AdminListen,
			Handler: admin.HandlerWithEvents(&ready, func() admin.Metrics {
				byKind := map[string]int{}
				for _, t := range counting.Snapshot() {
					byKind[string(t.Kind)] = t.Count
				}
				hs := engine.ScoreSnapshot()
				return admin.Metrics{
					Mode:       string(cfg.Mode),
					Total:      counting.Total(),
					ByKind:     byKind,
					BySeverity: counting.BySeverity(),
					Sources:    counting.Sources(),
					Uptime:     time.Since(startTime),
					Score: admin.ScoreHistogram{
						Bounds:    hs.Bounds,
						CumCounts: hs.CumulativeCounts,
						Sum:       hs.Sum,
						Count:     hs.Count,
					},
				}
			}, ring.Recent),
			ReadHeaderTimeout: 10 * time.Second,
		}
	}

	upstreamNote := cfg.Upstream
	if upstreamNote == "" {
		upstreamNote = "(honeypot-only, no upstream)"
	}
	scheme := "http"
	if cfg.TLSEnabled() {
		scheme = "https"
	}
	fmt.Printf("heramba edge — mode=%s listen=%s (%s) upstream=%s\n", cfg.Mode, cfg.Listen, scheme, upstreamNote)
	if cfg.Mode == config.ModeObserve {
		fmt.Println("  OBSERVE mode: detection only, real traffic passes through untouched.")
	} else {
		fmt.Println("  PROTECT mode: inline — response normalization + tarpit on flagged sources.")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// SIGHUP hot-reloads the detection surface and alert wiring from the config
	// file without dropping connections. The listen sockets and TLS are NOT
	// rebound (those need a restart); mode, decoys/preset/canaries, sinks,
	// upstream and rate limit all take effect live.
	hup := make(chan os.Signal, 1)
	signal.Notify(hup, syscall.SIGHUP)
	go func() {
		for range hup {
			reloadServe(*configPath, alertSink, engine)
		}
	}()

	go func() {
		if err := serveEdge(srv, cfg); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Fprintf(os.Stderr, "heramba: serve error: %v\n", err)
			stop()
		}
	}()
	if adminSrv != nil {
		fmt.Printf("  admin: status + dashboard + health + metrics on %s (/ /dashboard /api/events /healthz /readyz /metrics /version)\n", cfg.AdminListen)
		go func() {
			if err := adminSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				fmt.Fprintf(os.Stderr, "heramba: admin serve error: %v\n", err)
			}
		}()
	}
	ready.Store(true)
	<-ctx.Done()
	fmt.Println("\nheramba: shutting down…")
	// Graceful drain: flip readiness to not-ready and wait the drain window so a
	// load balancer deregisters this instance before we stop serving.
	if d := cfg.DrainTimeout(); d > 0 {
		fmt.Printf("heramba: draining for %s (readiness now false)…\n", d)
	}
	drain(&ready, cfg.DrainTimeout(), time.Sleep)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	if adminSrv != nil {
		_ = adminSrv.Shutdown(shutdownCtx)
	}
	// Flush any buffered sinks (e.g. the email digest) so a final batch is not
	// lost on a clean shutdown.
	if err := alertSink.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "heramba: flushing sinks: %v\n", err)
	}

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

// reloadServe reloads config from path and applies it to a running engine: it
// swaps the alert wiring (into swap) and the engine's detection surface/mode.
// On any error it logs and leaves the running state untouched. Returns the
// applied config (or an error) so the behavior is unit-testable.
func reloadServe(path string, swap *alert.SwapSink, engine *edge.Engine) (config.Config, error) {
	cfg, err := config.Load(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "heramba: SIGHUP reload skipped: %v\n", err)
		return cfg, err
	}
	cfg = cfg.ApplyEnv()
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "heramba: SIGHUP reload skipped: %v\n", err)
		return cfg, err
	}
	if augmented, _, err := applyOpenAPICanaries(cfg); err == nil {
		cfg = augmented
	} else {
		fmt.Fprintf(os.Stderr, "heramba: openapi canaries disabled on reload (%v)\n", err)
	}
	newSink := buildSink(cfg)
	if err := engine.Reload(cfg, nil); err != nil { // nil = keep the engine's (counting) sink
		fmt.Fprintf(os.Stderr, "heramba: SIGHUP reload skipped: %v\n", err)
		return cfg, err
	}
	swap.Swap(newSink)
	fmt.Printf("heramba: reloaded config from %s (mode=%s preset=%q)\n", path, cfg.Mode, cfg.Decoys.Preset)
	return cfg, nil
}

// applyOpenAPICanaries, when cfg.Decoys.OpenAPISpec is set, reads the spec,
// derives a canary value for each sensitive request-body field, and appends the
// tokens to cfg.Decoys.Canaries (which the edge already watches — including in
// request bodies). It returns the augmented config and the number of canaries
// added. A read/parse error is returned so the caller can log and continue.
func applyOpenAPICanaries(cfg config.Config) (config.Config, int, error) {
	if cfg.Decoys.OpenAPISpec == "" {
		return cfg, 0, nil
	}
	spec, err := os.ReadFile(cfg.Decoys.OpenAPISpec) //nolint:gosec // operator-supplied spec path
	if err != nil {
		return cfg, 0, err
	}
	fields, err := openapi.SensitiveFields(spec)
	if err != nil {
		return cfg, 0, err
	}
	cans := decoy.BodyCanaries(fields, cfg.Decoys.CanarySalt)
	for _, c := range cans {
		cfg.Decoys.Canaries = append(cfg.Decoys.Canaries, c.Token)
	}
	return cfg, len(cans), nil
}

// serveEdge starts the edge server over HTTPS when TLS files are configured,
// otherwise plain HTTP. ACME/auto-cert is intentionally not supported — supply
// a static cert/key or terminate ACME at an upstream load balancer.
func serveEdge(srv *http.Server, cfg config.Config) error {
	if cfg.TLSEnabled() {
		return srv.ListenAndServeTLS(cfg.TLS.CertFile, cfg.TLS.KeyFile)
	}
	return srv.ListenAndServe()
}

// drain performs the graceful-drain step: it flips the readiness flag to false
// (so load balancers stop routing new traffic) and then waits the drain window.
// sleep is injected so the behavior is unit-testable without real time.
func drain(ready *atomic.Bool, window time.Duration, sleep func(time.Duration)) {
	if ready != nil {
		ready.Store(false)
	}
	if window > 0 {
		sleep(window)
	}
}

// buildSink wires the configured alert output: a local text/jsonl record
// (always everything), optionally fanned out to a webhook and/or Slack above a
// severity floor.
func buildSink(cfg config.Config) alert.Sink {
	sinks := alert.MultiSink{baseSink(cfg)}
	floor := parseSeverity(cfg.Alert.MinSeverity, event.SeverityHigh)
	if u := cfg.Alert.Webhook; u != "" {
		sinks = append(sinks, alert.NewWebhook(u, 5*time.Second, floor))
	}
	if u := cfg.Alert.Slack; u != "" {
		sinks = append(sinks, alert.NewSlack(u, 5*time.Second, floor))
	}
	if u := cfg.Alert.Teams; u != "" {
		sinks = append(sinks, alert.NewTeams(u, 5*time.Second, floor))
	}
	if u := cfg.Alert.Discord; u != "" {
		sinks = append(sinks, alert.NewDiscord(u, 5*time.Second, floor))
	}
	if u := cfg.Alert.GoogleChat; u != "" {
		sinks = append(sinks, alert.NewGoogleChat(u, 5*time.Second, floor))
	}
	if u := cfg.Alert.Mattermost; u != "" {
		sinks = append(sinks, alert.NewMattermost(u, 5*time.Second, floor))
	}
	if c := cfg.Alert.Splunk; c.URL != "" {
		sinks = append(sinks, alert.NewSplunk(c.URL, c.Token, 5*time.Second, floor))
	}
	if c := cfg.Alert.Datadog; c.APIKey != "" {
		sinks = append(sinks, alert.NewDatadog(c.URL, c.APIKey, 5*time.Second, floor))
	}
	if c := cfg.Alert.Elastic; c.URL != "" {
		sinks = append(sinks, alert.NewElastic(c.URL, c.Index, c.APIKey, 5*time.Second, floor))
	}
	if c := cfg.Alert.Loki; c.URL != "" {
		sinks = append(sinks, alert.NewLoki(c.URL, c.Tenant, 5*time.Second, floor))
	}
	if c := cfg.Alert.NewRelic; c.APIKey != "" {
		sinks = append(sinks, alert.NewNewRelic(c.URL, c.APIKey, 5*time.Second, floor))
	}
	if u := cfg.Alert.Sumo; u != "" {
		sinks = append(sinks, alert.NewSumoLogic(u, 5*time.Second, floor))
	}
	if c := cfg.Alert.TemplateWebhook; c.URL != "" {
		if tw, err := alert.NewTemplateWebhook(c.URL, c.Template, c.ContentType, 5*time.Second, floor); err == nil {
			sinks = append(sinks, tw)
		} else {
			fmt.Fprintf(os.Stderr, "heramba: template webhook disabled (%v)\n", err)
		}
	}
	if c := cfg.Alert.Rotate; c.File != "" {
		if rs, err := alert.NewRotatingFile(c.File, c.MaxBytes, c.MaxBackups); err == nil {
			sinks = append(sinks, rs) // records everything (no floor): durable audit trail
		} else {
			fmt.Fprintf(os.Stderr, "heramba: rotating-file sink disabled (%v)\n", err)
		}
	}
	if c := cfg.Alert.Email; c.Host != "" && c.From != "" && len(c.To) > 0 {
		if es, err := alert.NewEmail(alert.EmailOptions{
			Host: c.Host, Port: c.Port, From: c.From, To: c.To,
			Username: c.Username, Password: c.Password,
			MinSev: floor, BatchSize: c.BatchSize,
		}); err == nil {
			sinks = append(sinks, es)
		} else {
			fmt.Fprintf(os.Stderr, "heramba: email sink disabled (%v)\n", err)
		}
	}
	if c := cfg.Alert.Syslog; c.Address != "" {
		network := c.Network
		if network == "" {
			network = "udp"
		}
		if s, err := alert.NewSyslogCEF(network, c.Address, floor); err == nil {
			sinks = append(sinks, s)
		} else {
			fmt.Fprintf(os.Stderr, "heramba: syslog sink disabled (dial %s %s: %v)\n", network, c.Address, err)
		}
	}
	// Paging defaults to a high floor regardless of the global one — never page
	// on observe-mode noise unless explicitly asked.
	pageFloor := floor
	if pageFloor < event.SeverityHigh {
		pageFloor = event.SeverityHigh
	}
	if k := cfg.Alert.PagerDutyKey; k != "" {
		sinks = append(sinks, alert.NewPagerDuty(k, 5*time.Second, pageFloor))
	}
	if k := cfg.Alert.OpsgenieKey; k != "" {
		sinks = append(sinks, alert.NewOpsgenie(k, 5*time.Second, pageFloor))
	}
	if len(sinks) == 1 {
		return sinks[0]
	}
	return sinks
}

// baseSink is the local text/jsonl record (file or stdout).
func baseSink(cfg config.Config) alert.Sink {
	w := os.Stdout
	if cfg.Alert.File != "" {
		f, err := os.OpenFile(cfg.Alert.File, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644) //nolint:gosec
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

// parseSeverity maps a config string to a severity floor, or def when unset.
func parseSeverity(s string, def event.Severity) event.Severity {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "info":
		return event.SeverityInfo
	case "low":
		return event.SeverityLow
	case "medium":
		return event.SeverityMedium
	case "high":
		return event.SeverityHigh
	case "critical":
		return event.SeverityCritical
	default:
		return def
	}
}
