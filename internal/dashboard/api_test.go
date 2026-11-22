package dashboard

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Armur-Ai/heramba/internal/event"
)

// sampleEvents returns a small fixed event set for handler tests.
func sampleEvents() []event.Event {
	base := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	mk := func(k event.Kind, sev event.Severity, src, tgt string, dt time.Duration) event.Event {
		e := event.New(k, sev, src, tgt, "d")
		e.Time = base.Add(dt)
		return e
	}
	return []event.Event{
		mk(event.KindDecoyHit, event.SeverityHigh, "10.0.0.1", "/.env", 0),
		mk(event.KindDecoyHit, event.SeverityHigh, "10.0.0.2", "/.env", time.Second),
		mk(event.KindCanaryTriggered, event.SeverityCritical, "10.0.0.3", "/keys", 2*time.Second),
	}
}

// testHandler builds a handler over a fixed event/metrics source.
func testHandler(opts ...Option) http.Handler {
	evs := sampleEvents()
	base := []Option{
		WithEvents(func(n int) []event.Event {
			if n > len(evs) {
				n = len(evs)
			}
			return evs[:n]
		}),
		WithMetrics(func() Metrics {
			return Metrics{Mode: "protect", Total: 3, Sources: 3,
				ByKind:     map[string]int{"decoy_hit": 2, "canary_triggered": 1},
				BySeverity: map[string]int{"high": 2, "critical": 1}}
		}),
	}
	return Handler(append(base, opts...)...)
}

func get(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, path, nil))
	return rr
}

func TestReadEndpointsStatusAndContentType(t *testing.T) {
	h := testHandler()
	cases := []struct{ path, ctype string }{
		{"/", "text/html"},
		{"/healthz", "text/plain"},
		{"/api/events", "application/json"},
		{"/api/metrics", "application/json"},
		{"/api/timeline", "application/json"},
		{"/api/sources", "application/json"},
		{"/api/campaigns", "application/json"},
		{"/api/decoys", "application/json"},
		{"/app.css", "text/css"},
		{"/app.js", "text/javascript"},
		{"/vendor/charts.js", "text/javascript"},
	}
	for _, c := range cases {
		rr := get(t, h, c.path)
		if rr.Code != http.StatusOK {
			t.Errorf("%s: status = %d, want 200", c.path, rr.Code)
		}
		if ct := rr.Header().Get("Content-Type"); !strings.Contains(ct, c.ctype) {
			t.Errorf("%s: content-type = %q, want contains %q", c.path, ct, c.ctype)
		}
	}
}

func TestEventsEncodesEmptyArray(t *testing.T) {
	h := Handler() // no events source
	rr := get(t, h, "/api/events")
	body := strings.TrimSpace(rr.Body.String())
	if body != "[]" {
		t.Fatalf("empty events body = %q, want []", body)
	}
}

func TestEventsDecodeShape(t *testing.T) {
	h := testHandler()
	rr := get(t, h, "/api/events?n=2")
	var evs []event.Event
	if err := json.Unmarshal(rr.Body.Bytes(), &evs); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(evs) != 2 {
		t.Fatalf("got %d events, want 2 (n= respected)", len(evs))
	}
	if evs[0].Kind != event.KindDecoyHit {
		t.Errorf("kind = %q, want decoy_hit", evs[0].Kind)
	}
}

func TestMetricsDecodeShape(t *testing.T) {
	h := testHandler()
	rr := get(t, h, "/api/metrics")
	var m metricsJSON
	if err := json.Unmarshal(rr.Body.Bytes(), &m); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if m.Mode != "protect" || m.Total != 3 || m.Sources != 3 {
		t.Errorf("metrics = %+v, want mode=protect total=3 sources=3", m)
	}
	if m.ByKind["decoy_hit"] != 2 {
		t.Errorf("by_kind decoy_hit = %d, want 2", m.ByKind["decoy_hit"])
	}
}

func TestSourceDrillEndpoint(t *testing.T) {
	h := testHandler()
	rr := get(t, h, "/api/sources/10.0.0.1")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var d SourceDetail
	if err := json.Unmarshal(rr.Body.Bytes(), &d); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if d.Source != "10.0.0.1" || d.Count != 1 {
		t.Errorf("drill = %+v, want source 10.0.0.1 count 1", d)
	}
	if len(d.Events) != 1 || d.Events[0].Source != "10.0.0.1" {
		t.Errorf("drill events = %+v, want one event for 10.0.0.1", d.Events)
	}
}

func TestBreachesEndpoint(t *testing.T) {
	h := testHandler()
	rr := get(t, h, "/api/breaches")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var b Breaches
	if err := json.Unmarshal(rr.Body.Bytes(), &b); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// sampleEvents has exactly one canary_triggered.
	if b.Count != 1 {
		t.Errorf("breach count = %d, want 1", b.Count)
	}
}

func TestSubpathsAreNotIndex(t *testing.T) {
	h := testHandler()
	// An unknown path must not be swallowed by the SPA root handler.
	rr := get(t, h, "/does-not-exist")
	if rr.Code != http.StatusNotFound {
		t.Errorf("unknown path status = %d, want 404", rr.Code)
	}
}

// --- control endpoints ---

func postJSON(t *testing.T, h http.Handler, path, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func TestControlRejectsMissingTokenWhenSet(t *testing.T) {
	h := testHandler(Authorize("s3cret"))
	paths := map[string]string{
		"/api/control/mode":         `{"mode":"protect"}`,
		"/api/control/clear":        ``,
		"/api/control/pause":        `{"paused":true}`,
		"/api/control/decoy-preset": `{"preset":"honeyA"}`,
	}
	for p, b := range paths {
		rr := postJSON(t, h, p, "", b) // no token
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("%s without token: status = %d, want 401", p, rr.Code)
		}
		rr = postJSON(t, h, p, "wrong", b) // wrong token
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("%s with wrong token: status = %d, want 401", p, rr.Code)
		}
	}
}

func TestControlAcceptsWithToken(t *testing.T) {
	mc := NewMemoryController()
	h := testHandler(Authorize("s3cret"), WithController(mc))

	if rr := postJSON(t, h, "/api/control/mode", "s3cret", `{"mode":"protect"}`); rr.Code != http.StatusOK {
		t.Fatalf("mode: status = %d, want 200", rr.Code)
	}
	if rr := postJSON(t, h, "/api/control/pause", "s3cret", `{"paused":true}`); rr.Code != http.StatusOK {
		t.Fatalf("pause: status = %d", rr.Code)
	}
	if rr := postJSON(t, h, "/api/control/decoy-preset", "s3cret", `{"preset":"honeyA"}`); rr.Code != http.StatusOK {
		t.Fatalf("preset: status = %d", rr.Code)
	}
	if rr := postJSON(t, h, "/api/control/clear", "s3cret", ``); rr.Code != http.StatusOK {
		t.Fatalf("clear: status = %d", rr.Code)
	}
	mode, paused, preset, clears := mc.State()
	if mode != "protect" || !paused || preset != "honeyA" || clears != 1 {
		t.Errorf("state = (%q,%v,%q,%d), want (protect,true,honeyA,1)", mode, paused, preset, clears)
	}
}

func TestControlNoTokenAllowsWrites(t *testing.T) {
	h := testHandler() // no Authorize → localhost-style open writes
	rr := postJSON(t, h, "/api/control/mode", "", `{"mode":"observe"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("open mode write: status = %d, want 200", rr.Code)
	}
}

func TestControlModeRejectsBadValue(t *testing.T) {
	h := testHandler()
	rr := postJSON(t, h, "/api/control/mode", "", `{"mode":"banana"}`)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("bad mode: status = %d, want 400", rr.Code)
	}
}

func TestControlMethodNotAllowed(t *testing.T) {
	h := testHandler()
	// GET on a POST-only control route should not succeed.
	rr := get(t, h, "/api/control/mode")
	if rr.Code == http.StatusOK {
		t.Errorf("GET on control/mode returned 200, want non-200")
	}
}
