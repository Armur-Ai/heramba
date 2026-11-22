package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/Armur-Ai/heramba/internal/config"
	"github.com/Armur-Ai/heramba/internal/dashboard"
	"github.com/Armur-Ai/heramba/internal/event"
)

// runDashboard powers `heramba dashboard` — the dark enterprise web dashboard.
// Like `watch`, it attaches to the JSONL event stream a running edge emits, so
// it never couples to the engine: follow a live sink file, read a pipe, or run
// a self-contained demo.
//
//	heramba dashboard --demo                      # seeded synthetic attack, no edge needed
//	heramba dashboard events.jsonl                # follow a live sink file
//	heramba serve --alert jsonl - | heramba dashboard -   # live over a pipe
//
// Controls (mode/pause/clear/preset) are live only against the demo feed; when
// attached to a real edge the dashboard is a read-only viewer and the control
// panel reports that plainly.
func runDashboard(args []string) {
	fs := flag.NewFlagSet("dashboard", flag.ExitOnError)
	listen := fs.String("listen", "127.0.0.1:7171", "address to serve the dashboard on")
	demo := fs.Bool("demo", false, "serve a self-contained synthetic attack feed (no edge required)")
	follow := fs.Bool("follow", true, "tail the source for new events (tail -f)")
	token := fs.String("token", "", "bearer token required to use the control endpoints (empty = open; localhost only)")
	maxN := fs.Int("max", 500, "events retained for the feed and aggregates")
	cfgPath := fs.String("config", ".heramba.yml", "config file (for the default source + mode)")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, "usage: heramba dashboard [flags] [source]\n\n"+
			"source is a JSONL event file, '-' for stdin, or empty to use the\n"+
			"alert sink from config. Serves a dark enterprise web dashboard.\n\nflags:\n")
		fs.PrintDefaults()
	}
	_ = fs.Parse(args)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	var handler http.Handler
	if *demo {
		handler = dashboard.DemoHandler(dashboard.Authorize(*token))
		fmt.Fprintf(os.Stderr, "heramba dashboard: serving a synthetic demo feed on http://%s\n", *listen)
	} else {
		cfg, _ := config.Load(*cfgPath)
		cfg = cfg.ApplyEnv()
		src := fs.Arg(0)
		if src == "" {
			if cfg.Alert.File != "" {
				src = cfg.Alert.File
			} else {
				src = "-"
			}
		}
		ch, err := openSource(ctx, src, *follow, false, 0)
		if err != nil {
			fmt.Fprintln(os.Stderr, "heramba dashboard:", err)
			os.Exit(1)
		}
		live := newLiveSource(*maxN, string(cfg.Mode))
		go live.consume(ch)
		handler = dashboard.Handler(
			dashboard.WithEvents(live.Recent),
			dashboard.WithMetrics(live.Metrics),
			dashboard.WithController(readOnlyController{}),
			dashboard.Authorize(*token),
		)
		fmt.Fprintf(os.Stderr, "heramba dashboard: attached to %s (read-only), serving on http://%s\n", src, *listen)
	}

	srv := &http.Server{
		Addr:              *listen,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutCtx, c := context.WithTimeout(context.Background(), 3*time.Second)
		defer c()
		_ = srv.Shutdown(shutCtx)
	}()
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintln(os.Stderr, "heramba dashboard:", err)
		os.Exit(1)
	}
}

// liveSource folds a live event channel into the newest-first ring and rolling
// counters the dashboard reads, concurrency-safe for the HTTP handlers.
type liveSource struct {
	mode    string
	started time.Time
	max     int

	mu         sync.RWMutex
	ring       []event.Event
	total      int
	byKind     map[string]int
	bySeverity map[string]int
	sources    map[string]struct{}
}

func newLiveSource(max int, mode string) *liveSource {
	if max <= 0 {
		max = 500
	}
	if mode == "" {
		mode = "observe"
	}
	return &liveSource{
		mode:       mode,
		started:    time.Now(),
		max:        max,
		byKind:     map[string]int{},
		bySeverity: map[string]int{},
		sources:    map[string]struct{}{},
	}
}

func (l *liveSource) consume(ch <-chan event.Event) {
	for e := range ch {
		l.add(e)
	}
}

func (l *liveSource) add(e event.Event) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.total++
	l.byKind[string(e.Kind)]++
	l.bySeverity[e.Severity.String()]++
	if e.Source != "" {
		l.sources[e.Source] = struct{}{}
	}
	l.ring = append(l.ring, e)
	if len(l.ring) > l.max {
		l.ring = l.ring[len(l.ring)-l.max:]
	}
}

// Recent returns up to n events, newest first.
func (l *liveSource) Recent(n int) []event.Event {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if n <= 0 || n > len(l.ring) {
		n = len(l.ring)
	}
	out := make([]event.Event, n)
	for i := 0; i < n; i++ {
		out[i] = l.ring[len(l.ring)-1-i]
	}
	return out
}

// Metrics is a snapshot for the dashboard's /api/metrics.
func (l *liveSource) Metrics() dashboard.Metrics {
	l.mu.RLock()
	defer l.mu.RUnlock()
	byKind := make(map[string]int, len(l.byKind))
	for k, v := range l.byKind {
		byKind[k] = v
	}
	bySev := make(map[string]int, len(l.bySeverity))
	for k, v := range l.bySeverity {
		bySev[k] = v
	}
	return dashboard.Metrics{
		Mode:       l.mode,
		Total:      l.total,
		ByKind:     byKind,
		BySeverity: bySev,
		Sources:    len(l.sources),
		Uptime:     time.Since(l.started),
	}
}

// readOnlyController rejects every control action with a clear reason: a
// dashboard attached to a live edge is a viewer, and posture changes must be
// made on the edge itself. This keeps the control panel honest rather than
// silently pretending to toggle a process it cannot reach.
type readOnlyController struct{}

var errReadOnly = errors.New("dashboard is attached in read-only view mode; change posture on the edge (heramba serve) instead")

func (readOnlyController) SetMode(string) error          { return errReadOnly }
func (readOnlyController) Clear() error                  { return errReadOnly }
func (readOnlyController) SetPaused(bool) error          { return errReadOnly }
func (readOnlyController) ApplyDecoyPreset(string) error { return errReadOnly }
