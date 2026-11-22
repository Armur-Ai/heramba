// Package admin serves Heramba's operational endpoints — liveness, readiness,
// and Prometheus metrics — on a separate port from the proxied traffic, so an
// orchestrator (k8s, a load balancer, Prometheus) can probe the edge without
// ever touching the data path.
package admin

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"sort"
	"sync/atomic"
	"time"

	"github.com/Armur-Ai/heramba/internal/version"
)

// Metrics is the snapshot the /metrics endpoint renders. It is provided by a
// function so this package stays decoupled from the alert/edge internals.
type Metrics struct {
	Mode       string
	Total      int
	ByKind     map[string]int
	BySeverity map[string]int
	Sources    int
	Uptime     time.Duration
	// Score is the agentic-classifier-score histogram (optional). Bounds are the
	// bucket upper bounds; CumCounts has one more entry (the trailing +Inf
	// bucket) and is cumulative. Zero value (nil Bounds) omits the metric.
	Score ScoreHistogram
}

// ScoreHistogram is a renderable histogram snapshot (cumulative counts).
type ScoreHistogram struct {
	Bounds    []float64
	CumCounts []uint64
	Sum       float64
	Count     uint64
}

// MetricsFunc returns the current metrics snapshot.
type MetricsFunc func() Metrics

// Handler builds the admin mux. ready gates /readyz (flip it true once the edge
// is actually serving); metrics may be nil (then /metrics reports only up/uptime).
func Handler(ready *atomic.Bool, metrics MetricsFunc) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		if ready != nil && ready.Load() {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ready\n"))
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("not ready\n"))
	})
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		writeMetrics(w, metrics)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" { // only the root is the status page
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		writeStatusPage(w, metrics)
	})
	mux.HandleFunc("/version", func(w http.ResponseWriter, _ *http.Request) {
		var uptime time.Duration
		if metrics != nil {
			uptime = metrics().Uptime
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(versionInfo{
			Version:       version.Version,
			Commit:        version.Commit,
			Date:          version.Date,
			UptimeSeconds: int(uptime.Seconds()),
		})
	})
	return mux
}

// versionInfo is the JSON body served by /version: build metadata (stamped at
// release time via -ldflags) plus how long this edge has been running.
type versionInfo struct {
	Version       string `json:"version"`
	Commit        string `json:"commit"`
	Date          string `json:"date"`
	UptimeSeconds int    `json:"uptime_seconds"`
}

func writeMetrics(w http.ResponseWriter, metrics MetricsFunc) {
	fmt.Fprintln(w, "# HELP heramba_up 1 if the edge is running.")
	fmt.Fprintln(w, "# TYPE heramba_up gauge")
	fmt.Fprintln(w, "heramba_up 1")

	fmt.Fprintln(w, "# HELP heramba_build_info Build metadata; value is always 1.")
	fmt.Fprintln(w, "# TYPE heramba_build_info gauge")
	fmt.Fprintf(w, "heramba_build_info{version=%q,commit=%q,date=%q} 1\n",
		version.Version, version.Commit, version.Date)

	if metrics == nil {
		return
	}
	m := metrics()
	fmt.Fprintln(w, "# HELP heramba_uptime_seconds Seconds since the edge started.")
	fmt.Fprintln(w, "# TYPE heramba_uptime_seconds gauge")
	fmt.Fprintf(w, "heramba_uptime_seconds %d\n", int(m.Uptime.Seconds()))

	fmt.Fprintln(w, "# HELP heramba_detections_total Total detections emitted.")
	fmt.Fprintln(w, "# TYPE heramba_detections_total counter")
	fmt.Fprintf(w, "heramba_detections_total %d\n", m.Total)

	fmt.Fprintln(w, "# HELP heramba_detections_by_kind_total Detections by kind.")
	fmt.Fprintln(w, "# TYPE heramba_detections_by_kind_total counter")
	kinds := make([]string, 0, len(m.ByKind))
	for k := range m.ByKind {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds) // stable output
	for _, k := range kinds {
		fmt.Fprintf(w, "heramba_detections_by_kind_total{kind=%q} %d\n", k, m.ByKind[k])
	}

	fmt.Fprintln(w, "# HELP heramba_detections_by_severity_total Detections by severity.")
	fmt.Fprintln(w, "# TYPE heramba_detections_by_severity_total counter")
	sevs := make([]string, 0, len(m.BySeverity))
	for s := range m.BySeverity {
		sevs = append(sevs, s)
	}
	sort.Strings(sevs) // stable output
	for _, s := range sevs {
		fmt.Fprintf(w, "heramba_detections_by_severity_total{severity=%q} %d\n", s, m.BySeverity[s])
	}

	fmt.Fprintln(w, "# HELP heramba_sources_total Distinct detection sources seen.")
	fmt.Fprintln(w, "# TYPE heramba_sources_total gauge")
	fmt.Fprintf(w, "heramba_sources_total %d\n", m.Sources)

	writeScoreHistogram(w, m.Score)
}

// writeScoreHistogram renders the agentic-score histogram in Prometheus format
// (cumulative buckets + _sum + _count). A nil-bounds histogram is omitted.
func writeScoreHistogram(w http.ResponseWriter, h ScoreHistogram) {
	if len(h.Bounds) == 0 || len(h.CumCounts) != len(h.Bounds)+1 {
		return
	}
	fmt.Fprintln(w, "# HELP heramba_agentic_score Per-request agentic classifier score (0..1).")
	fmt.Fprintln(w, "# TYPE heramba_agentic_score histogram")
	for i, b := range h.Bounds {
		fmt.Fprintf(w, "heramba_agentic_score_bucket{le=\"%g\"} %d\n", b, h.CumCounts[i])
	}
	fmt.Fprintf(w, "heramba_agentic_score_bucket{le=\"+Inf\"} %d\n", h.CumCounts[len(h.CumCounts)-1])
	fmt.Fprintf(w, "heramba_agentic_score_sum %g\n", h.Sum)
	fmt.Fprintf(w, "heramba_agentic_score_count %d\n", h.Count)
}

// writeStatusPage renders a tiny, self-contained (no external assets) read-only
// status page: build version, mode, uptime, totals, and the detections broken
// down by kind. Everything comes from the same metrics func that feeds /metrics.
func writeStatusPage(w http.ResponseWriter, metrics MetricsFunc) {
	esc := html.EscapeString
	fmt.Fprintln(w, "<!doctype html><html lang=en><head><meta charset=utf-8>")
	fmt.Fprintln(w, "<meta name=viewport content=\"width=device-width,initial-scale=1\">")
	fmt.Fprintln(w, "<title>Heramba status</title>")
	fmt.Fprintln(w, "<style>body{font:14px/1.5 system-ui,sans-serif;max-width:640px;margin:2rem auto;padding:0 1rem;color:#1a1a1a}"+
		"h1{font-size:1.3rem}table{border-collapse:collapse;width:100%}td,th{text-align:left;padding:.3rem .6rem;border-bottom:1px solid #eee}"+
		"code{background:#f4f4f5;padding:.1rem .3rem;border-radius:3px}.muted{color:#71717a}</style></head><body>")
	fmt.Fprintf(w, "<h1>Heramba edge</h1>\n")
	fmt.Fprintf(w, "<p class=muted>version <code>%s</code> · commit <code>%s</code></p>\n",
		esc(version.Version), esc(version.Commit))

	if metrics == nil {
		fmt.Fprintln(w, "<p>No metrics source attached.</p></body></html>")
		return
	}
	m := metrics()
	mode := m.Mode
	if mode == "" {
		mode = "unknown"
	}
	fmt.Fprintln(w, "<table>")
	fmt.Fprintf(w, "<tr><th>mode</th><td>%s</td></tr>\n", esc(mode))
	fmt.Fprintf(w, "<tr><th>uptime</th><td>%s</td></tr>\n", esc(m.Uptime.Round(time.Second).String()))
	fmt.Fprintf(w, "<tr><th>detections</th><td>%d</td></tr>\n", m.Total)
	fmt.Fprintf(w, "<tr><th>distinct sources</th><td>%d</td></tr>\n", m.Sources)
	fmt.Fprintln(w, "</table>")

	if len(m.ByKind) > 0 {
		kinds := make([]string, 0, len(m.ByKind))
		for k := range m.ByKind {
			kinds = append(kinds, k)
		}
		sort.Slice(kinds, func(i, j int) bool {
			if m.ByKind[kinds[i]] != m.ByKind[kinds[j]] {
				return m.ByKind[kinds[i]] > m.ByKind[kinds[j]]
			}
			return kinds[i] < kinds[j]
		})
		fmt.Fprintln(w, "<h2>By kind</h2><table><tr><th>kind</th><th>count</th></tr>")
		for _, k := range kinds {
			fmt.Fprintf(w, "<tr><td>%s</td><td>%d</td></tr>\n", esc(k), m.ByKind[k])
		}
		fmt.Fprintln(w, "</table>")
	}
	fmt.Fprintln(w, "<p class=muted>Endpoints: <code>/healthz</code> <code>/readyz</code> <code>/metrics</code> <code>/version</code></p>")
	fmt.Fprintln(w, "</body></html>")
}
