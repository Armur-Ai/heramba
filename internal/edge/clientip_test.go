package edge

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Armur-Ai/heramba/internal/config"
	"github.com/Armur-Ai/heramba/internal/event"
)

// sourceOfDecoyHit drives a decoy request and returns the Source the engine
// recorded (which is the resolved client IP).
func sourceOfDecoyHit(t *testing.T, cfg config.Config, xff string) string {
	t.Helper()
	sink := &capSink{}
	e, _ := New(cfg, sink)
	srv := httptest.NewServer(e.Handler())
	defer srv.Close()
	req, _ := http.NewRequest("GET", srv.URL+"/.env", nil)
	if xff != "" {
		req.Header.Set("X-Forwarded-For", xff)
	}
	_, _ = http.DefaultClient.Do(req)
	for _, ev := range sink.events {
		if ev.Kind == event.KindDecoyHit {
			return ev.Source
		}
	}
	t.Fatal("no decoy hit recorded")
	return ""
}

func TestXFFIgnoredByDefault(t *testing.T) {
	src := sourceOfDecoyHit(t, config.Default(), "203.0.113.9")
	if src == "203.0.113.9" {
		t.Fatal("XFF must NOT be trusted by default (spoofable)")
	}
}

func TestXFFHonoredFromTrustedProxy(t *testing.T) {
	cfg := config.Default()
	cfg.TrustedProxies = []string{"127.0.0.0/8"} // httptest peer is loopback
	src := sourceOfDecoyHit(t, cfg, "203.0.113.9")
	if src != "203.0.113.9" {
		t.Fatalf("trusted proxy XFF should be honored, got %q", src)
	}
}

func TestXFFPicksLeftmostValid(t *testing.T) {
	cfg := config.Default()
	cfg.TrustedProxies = []string{"127.0.0.0/8"}
	src := sourceOfDecoyHit(t, cfg, "198.51.100.7, 10.0.0.1")
	if src != "198.51.100.7" {
		t.Fatalf("should take leftmost XFF entry, got %q", src)
	}
}
