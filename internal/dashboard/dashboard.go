// Package dashboard is a self-contained, embeddable dark enterprise web
// dashboard for Heramba — the active-defense / deception security proxy.
//
// It is deliberately decoupled from the rest of the edge: it consumes detection
// events and a metrics snapshot through small function fields (mirroring the
// shapes in internal/admin) and drives write-side controls through a Controller
// interface the caller provides. The frontend is a single-page app whose HTML,
// CSS, JS and a hand-rolled, MIT-licensed SVG charting micro-library are all
// embedded via go:embed — nothing is fetched from a CDN, so the dashboard runs
// fully offline in air-gapped environments.
//
// Build a handler with functional options:
//
//	h := dashboard.Handler(
//		dashboard.WithEvents(src.Recent),
//		dashboard.WithMetrics(src.Metrics),
//		dashboard.WithController(ctl),
//		dashboard.Authorize(token), // optional: guards POST /api/control/*
//	)
//	http.ListenAndServe("127.0.0.1:7171", h)
//
// Or run it standalone against a synthetic live attack stream:
//
//	h := dashboard.DemoHandler()
//
// Security note: the dashboard exposes write controls and is intended to bind
// localhost by default. When exposing it beyond loopback, always set a bearer
// token with Authorize so the control endpoints reject unauthenticated writes.
package dashboard

import (
	"embed"
	"io/fs"
	"net/http"
	"time"

	"github.com/Armur-Ai/heramba/internal/event"
)

//go:embed all:web
var webFS embed.FS

// EventsFunc returns up to n of the most recent detection events, newest first.
// It mirrors admin.EventsFunc so an existing source satisfies both.
type EventsFunc func(n int) []event.Event

// MetricsFunc returns the current metrics snapshot.
type MetricsFunc func() Metrics

// ScoreHistogram is a renderable agentic-classifier-score histogram snapshot
// (cumulative counts). Bounds are bucket upper bounds; CumCounts has one more
// entry (the trailing +Inf bucket) and is cumulative. It mirrors the shape in
// internal/admin so a caller can reuse the same snapshot.
type ScoreHistogram struct {
	Bounds    []float64 `json:"bounds"`
	CumCounts []uint64  `json:"cum_counts"`
	Sum       float64   `json:"sum"`
	Count     uint64    `json:"count"`
}

// Metrics is the snapshot the dashboard renders. It mirrors admin.Metrics but
// is defined locally to keep this package decoupled from the ops server.
type Metrics struct {
	Mode       string         `json:"mode"`
	Total      int            `json:"total"`
	ByKind     map[string]int `json:"by_kind"`
	BySeverity map[string]int `json:"by_severity"`
	Sources    int            `json:"sources"`
	Uptime     time.Duration  `json:"-"`
	Score      ScoreHistogram `json:"score"`
}

// Controller is the write side of the dashboard: the control panel routes its
// POST actions through it. Implementations apply the change to the live edge.
// The caller supplies one; a safe in-memory default (MemoryController) is used
// when none is configured, so the dashboard never writes to a real edge by
// accident.
type Controller interface {
	// SetMode switches the edge posture, e.g. "observe" or "protect".
	SetMode(mode string) error
	// Clear resets accumulated detections / counters.
	Clear() error
	// SetPaused pauses or resumes detection processing.
	SetPaused(paused bool) error
	// ApplyDecoyPreset activates a named decoy preset (bundle of decoys/canaries).
	ApplyDecoyPreset(preset string) error
}

// config holds the resolved options for a handler.
type config struct {
	events     EventsFunc
	metrics    MetricsFunc
	controller Controller
	token      string
}

// Option configures a dashboard Handler.
type Option func(*config)

// WithEvents sets the recent-events source feeding the live feed and charts.
func WithEvents(f EventsFunc) Option { return func(c *config) { c.events = f } }

// WithMetrics sets the metrics-snapshot source feeding the tiles and gauges.
func WithMetrics(f MetricsFunc) Option { return func(c *config) { c.metrics = f } }

// WithController sets the Controller the control panel writes through.
func WithController(ctl Controller) Option { return func(c *config) { c.controller = ctl } }

// Authorize requires a bearer token on write (POST /api/control/*) endpoints.
// An empty token leaves writes unauthenticated (intended for localhost binds).
func Authorize(token string) Option { return func(c *config) { c.token = token } }

// server holds the resolved dependencies behind the mux.
type server struct {
	events     EventsFunc
	metrics    MetricsFunc
	controller Controller
	token      string
	startedAt  time.Time
}

// Handler builds the dashboard HTTP handler from the given options. It is safe
// to call with no options (serves the SPA and empty API responses). Missing
// sources degrade gracefully: no events means an empty feed, no metrics means
// zeroed tiles, no controller means an in-memory MemoryController.
func Handler(opts ...Option) http.Handler {
	cfg := &config{}
	for _, o := range opts {
		o(cfg)
	}
	if cfg.controller == nil {
		cfg.controller = NewMemoryController()
	}
	s := &server{
		events:     cfg.events,
		metrics:    cfg.metrics,
		controller: cfg.controller,
		token:      cfg.token,
		startedAt:  time.Now(),
	}
	return s.routes()
}

// routes wires the mux: the embedded SPA and static assets, the read APIs, the
// write controls, and health.
func (s *server) routes() http.Handler {
	mux := http.NewServeMux()

	sub, err := fs.Sub(webFS, "web")
	if err != nil {
		panic("dashboard: embedded web assets missing: " + err.Error())
	}
	fileServer := http.FileServerFS(sub)
	mux.Handle("GET /app.css", fileServer)
	mux.Handle("GET /app.js", fileServer)
	mux.Handle("GET /vendor/", fileServer)

	// SPA shell at the root only.
	mux.HandleFunc("GET /{$}", s.handleIndex(sub))

	// Read APIs.
	mux.HandleFunc("GET /api/events", s.handleEvents)
	mux.HandleFunc("GET /api/metrics", s.handleMetrics)
	mux.HandleFunc("GET /api/timeline", s.handleTimeline)
	mux.HandleFunc("GET /api/sources", s.handleSources)
	mux.HandleFunc("GET /api/campaigns", s.handleCampaigns)
	mux.HandleFunc("GET /api/decoys", s.handleDecoys)

	// Write controls.
	mux.HandleFunc("POST /api/control/mode", s.handleControlMode)
	mux.HandleFunc("POST /api/control/clear", s.handleControlClear)
	mux.HandleFunc("POST /api/control/pause", s.handleControlPause)
	mux.HandleFunc("POST /api/control/decoy-preset", s.handleControlDecoyPreset)

	// Health.
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})

	return mux
}

// handleIndex serves the embedded SPA shell at "/".
func (s *server) handleIndex(sub fs.FS) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		b, err := fs.ReadFile(sub, "index.html")
		if err != nil {
			http.Error(w, "dashboard unavailable", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(b)
	}
}

// uptime reports how long this handler has been serving.
func (s *server) uptime() time.Duration { return time.Since(s.startedAt) }
