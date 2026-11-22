// Package admin serves Heramba's operational endpoints — liveness, readiness,
// and Prometheus metrics — on a separate port from the proxied traffic, so an
// orchestrator (k8s, a load balancer, Prometheus) can probe the edge without
// ever touching the data path.
package admin

import (
	"fmt"
	"net/http"
	"sort"
	"sync/atomic"
	"time"
)

// Metrics is the snapshot the /metrics endpoint renders. It is provided by a
// function so this package stays decoupled from the alert/edge internals.
type Metrics struct {
	Total  int
	ByKind map[string]int
	Uptime time.Duration
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
	return mux
}

func writeMetrics(w http.ResponseWriter, metrics MetricsFunc) {
	fmt.Fprintln(w, "# HELP heramba_up 1 if the edge is running.")
	fmt.Fprintln(w, "# TYPE heramba_up gauge")
	fmt.Fprintln(w, "heramba_up 1")

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
}
