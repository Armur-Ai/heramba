package admin

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Armur-Ai/heramba/internal/version"
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
		return Metrics{
			Total:      5,
			ByKind:     map[string]int{"decoy_hit": 3, "canary_triggered": 2},
			BySeverity: map[string]int{"high": 4, "medium": 1},
			Sources:    2,
			Uptime:     90 * time.Second,
		}
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
		`heramba_detections_by_severity_total{severity="high"} 4`,
		`heramba_detections_by_severity_total{severity="medium"} 1`,
		"# TYPE heramba_detections_by_severity_total counter",
		"heramba_sources_total 2",
		"# TYPE heramba_sources_total gauge",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("metrics missing %q in:\n%s", want, body)
		}
	}
}

func TestVersionEndpointReportsBuildAndUptime(t *testing.T) {
	h := Handler(nil, func() Metrics {
		return Metrics{Uptime: 42 * time.Second}
	})
	code, body := do(t, h, "/version")
	if code != 200 {
		t.Fatalf("version code %d", code)
	}
	var info struct {
		Version       string `json:"version"`
		Commit        string `json:"commit"`
		Date          string `json:"date"`
		UptimeSeconds int    `json:"uptime_seconds"`
	}
	if err := json.Unmarshal([]byte(body), &info); err != nil {
		t.Fatalf("version body must be JSON: %v (%s)", err, body)
	}
	if info.Version != version.Version {
		t.Errorf("version = %q, want wired %q", info.Version, version.Version)
	}
	if info.UptimeSeconds != 42 {
		t.Errorf("uptime_seconds = %d, want 42", info.UptimeSeconds)
	}
}

func TestVersionEndpointWithNilMetrics(t *testing.T) {
	code, body := do(t, Handler(nil, nil), "/version")
	if code != 200 {
		t.Fatalf("version code %d", code)
	}
	if !strings.Contains(body, `"uptime_seconds":0`) {
		t.Errorf("nil metrics should report zero uptime: %s", body)
	}
	if !strings.Contains(body, `"version"`) {
		t.Errorf("version body missing version field: %s", body)
	}
}

func TestMetricsNilSourceStillServesUp(t *testing.T) {
	code, body := do(t, Handler(nil, nil), "/metrics")
	if code != 200 || !strings.Contains(body, "heramba_up 1") {
		t.Fatalf("nil metrics should still report up: %d %q", code, body)
	}
	// build_info is independent of the metrics source and must always appear.
	if !strings.Contains(body, "heramba_build_info{version=") {
		t.Errorf("metrics missing build_info: %s", body)
	}
	if !strings.Contains(body, "# TYPE heramba_build_info gauge") {
		t.Errorf("metrics missing build_info TYPE line: %s", body)
	}
}

func TestStatusPageServesHTML(t *testing.T) {
	h := Handler(nil, func() Metrics {
		return Metrics{
			Mode:    "observe",
			Total:   7,
			ByKind:  map[string]int{"decoy_hit": 5, "probe": 2},
			Sources: 3,
			Uptime:  65 * time.Second,
		}
	})
	code, body := do(t, h, "/")
	if code != 200 {
		t.Fatalf("status page code %d", code)
	}
	for _, want := range []string{
		"<!doctype html>", "Heramba edge", "observe", "decoy_hit", "distinct sources", "7", "/metrics",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("status page missing %q in:\n%s", want, body)
		}
	}
	// Self-contained: no external asset references.
	for _, bad := range []string{"http://", "https://", "src="} {
		if strings.Contains(body, bad) {
			t.Errorf("status page should be self-contained, found %q", bad)
		}
	}
}

func TestStatusPageUnknownPathIs404(t *testing.T) {
	h := Handler(nil, nil)
	if code, _ := do(t, h, "/nope"); code != http.StatusNotFound {
		t.Fatalf("unknown admin path should 404, got %d", code)
	}
}

func TestStatusPageNilMetrics(t *testing.T) {
	code, body := do(t, Handler(nil, nil), "/")
	if code != 200 || !strings.Contains(body, "No metrics source") {
		t.Fatalf("nil-metrics status page: %d %q", code, body)
	}
}
