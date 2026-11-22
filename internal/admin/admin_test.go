package admin

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func do(t *testing.T, h http.Handler, path string) (int, string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	b, _ := io.ReadAll(rec.Result().Body)
	return rec.Code, string(b)
}

func TestHealthzAlwaysOK(t *testing.T) {
	h := Handler(nil, nil)
	if code, body := do(t, h, "/healthz"); code != 200 || !strings.Contains(body, "ok") {
		t.Fatalf("healthz = %d %q", code, body)
	}
}

func TestReadyzGatedByFlag(t *testing.T) {
	var ready atomic.Bool
	h := Handler(&ready, nil)
	if code, _ := do(t, h, "/readyz"); code != http.StatusServiceUnavailable {
		t.Fatalf("readyz should be 503 before ready, got %d", code)
	}
	ready.Store(true)
	if code, body := do(t, h, "/readyz"); code != 200 || !strings.Contains(body, "ready") {
		t.Fatalf("readyz should be 200 ready, got %d %q", code, body)
	}
}

func TestMetricsPrometheusFormat(t *testing.T) {
	var ready atomic.Bool
	ready.Store(true)
	h := Handler(&ready, func() Metrics {
		return Metrics{Total: 5, ByKind: map[string]int{"decoy_hit": 3, "canary_triggered": 2}, Uptime: 90 * time.Second}
	})
	code, body := do(t, h, "/metrics")
	if code != 200 {
		t.Fatalf("metrics code %d", code)
	}
	for _, want := range []string{
		"heramba_up 1",
		"heramba_uptime_seconds 90",
		"heramba_detections_total 5",
		`heramba_detections_by_kind_total{kind="decoy_hit"} 3`,
		`heramba_detections_by_kind_total{kind="canary_triggered"} 2`,
		"# TYPE heramba_detections_total counter",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("metrics missing %q in:\n%s", want, body)
		}
	}
}

func TestMetricsNilSourceStillServesUp(t *testing.T) {
	code, body := do(t, Handler(nil, nil), "/metrics")
	if code != 200 || !strings.Contains(body, "heramba_up 1") {
		t.Fatalf("nil metrics should still report up: %d %q", code, body)
	}
}
