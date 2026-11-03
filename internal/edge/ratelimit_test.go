package edge

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Armur-Ai/heramba/internal/config"
	"github.com/Armur-Ai/heramba/internal/event"
)

func TestProtectRateLimits(t *testing.T) {
	up := newUpstream(t)
	defer up.Close()
	cfg := config.Default()
	cfg.Mode = config.ModeProtect
	cfg.Upstream = up.URL
	cfg.RateLimitRPS = 1
	cfg.RateLimitBurst = 2
	sink := &capSink{}
	e, _ := New(cfg, sink)
	srv := httptest.NewServer(e.Handler())
	defer srv.Close()

	// Burst of 2 passes, 3rd is limited (same instant).
	codes := []int{}
	for i := 0; i < 3; i++ {
		resp, _ := http.Get(srv.URL + "/api/x")
		codes = append(codes, resp.StatusCode)
	}
	if codes[0] != 200 || codes[1] != 200 {
		t.Fatalf("first two should pass: %v", codes)
	}
	if codes[2] != http.StatusTooManyRequests {
		t.Fatalf("third should be 429, got %d", codes[2])
	}
	if sink.kinds()[event.KindRateLimited] < 1 {
		t.Fatalf("expected a rate_limited event, got %+v", sink.kinds())
	}
}

func TestObserveNeverRateLimits(t *testing.T) {
	up := newUpstream(t)
	defer up.Close()
	cfg := config.Default() // observe
	cfg.Upstream = up.URL
	cfg.RateLimitRPS = 1 // set, but observe must ignore it
	e, _ := New(cfg, &capSink{})
	srv := httptest.NewServer(e.Handler())
	defer srv.Close()
	for i := 0; i < 5; i++ {
		resp, _ := http.Get(srv.URL + "/api/x")
		if resp.StatusCode != 200 {
			t.Fatalf("observe must never rate-limit, got %d on request %d", resp.StatusCode, i)
		}
	}
}
