package dashboard

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSecurityHeadersOnShellAndAssets(t *testing.T) {
	h := testHandler()
	for _, path := range []string{"/", "/app.js", "/app.css", "/vendor/charts.js", "/api/metrics"} {
		rr := get(t, h, path)
		if got := rr.Header().Get("X-Content-Type-Options"); got != "nosniff" {
			t.Errorf("%s: X-Content-Type-Options = %q, want nosniff", path, got)
		}
		if got := rr.Header().Get("X-Frame-Options"); got != "DENY" {
			t.Errorf("%s: X-Frame-Options = %q, want DENY", path, got)
		}
		if csp := rr.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "default-src 'self'") {
			t.Errorf("%s: CSP = %q, want a self default-src", path, csp)
		}
	}
}

func TestAPIsAreNoStore(t *testing.T) {
	h := testHandler()
	for _, path := range []string{
		"/api/events", "/api/metrics", "/api/timeline", "/api/sources",
		"/api/sources/10.0.0.1", "/api/breaches", "/api/campaigns", "/api/decoys",
		"/api/export/events.csv", "/api/export/events.json",
	} {
		rr := get(t, h, path)
		if cc := rr.Header().Get("Cache-Control"); cc != "no-store" {
			t.Errorf("%s: Cache-Control = %q, want no-store", path, cc)
		}
	}
}

func TestWrongMethodReturns405(t *testing.T) {
	h := testHandler()
	// POST to GET-only read routes.
	for _, path := range []string{"/api/events", "/api/metrics", "/api/breaches", "/api/stream"} {
		req := httptest.NewRequest(http.MethodPost, path, nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("POST %s: status = %d, want 405", path, rr.Code)
		}
	}
	// GET on POST-only control routes.
	for _, path := range []string{"/api/control/mode", "/api/control/clear", "/api/control/pause"} {
		rr := get(t, h, path)
		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("GET %s: status = %d, want 405", path, rr.Code)
		}
	}
}

func TestEventCountIsCapped(t *testing.T) {
	if n := clampCount("999999"); n != maxEventCount {
		t.Errorf("clampCount(999999) = %d, want cap %d", n, maxEventCount)
	}
	if n := clampCount("0"); n != defaultEventCount {
		t.Errorf("clampCount(0) = %d, want default %d", n, defaultEventCount)
	}
	if n := clampCount("-5"); n != defaultEventCount {
		t.Errorf("clampCount(-5) = %d, want default %d", n, defaultEventCount)
	}
	if n := clampCount("junk"); n != defaultEventCount {
		t.Errorf("clampCount(junk) = %d, want default %d", n, defaultEventCount)
	}
}
