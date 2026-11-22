package main

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Armur-Ai/heramba/internal/event"
	"github.com/Armur-Ai/heramba/internal/poison"
)

// capturingSink records emitted events for assertions.
type capturingSink struct {
	mu sync.Mutex
	ev []event.Event
}

func (c *capturingSink) Emit(e event.Event) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ev = append(c.ev, e)
	return nil
}
func (c *capturingSink) events() []event.Event {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]event.Event(nil), c.ev...)
}

func newTestTrap(t *testing.T) (*trap, *capturingSink) {
	t.Helper()
	sink := &capturingSink{}
	th, err := newTrapHandler(trapConfig{
		Seed: 7, Base: "http://trap.test", Sink: sink,
		BaseDelay: 0, MaxDelay: 0, Jitter: 0, ChunkSize: 4096, // fast for tests
	})
	if err != nil {
		t.Fatalf("newTrapHandler: %v", err)
	}
	return th, sink
}

func TestTrapServesBaitSurfaces(t *testing.T) {
	th, _ := newTestTrap(t)
	rec := httptest.NewRecorder()
	th.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/robots.txt", nil))
	if rec.Code != 200 || rec.Body.Len() == 0 {
		t.Fatalf("robots.txt bait should be served, got code=%d len=%d", rec.Code, rec.Body.Len())
	}
	if !strings.Contains(rec.Body.String(), "Disallow") {
		t.Error("robots.txt bait should contain Disallow entries")
	}
}

func TestTrapDetectsTripAndConfirms(t *testing.T) {
	th, sink := newTestTrap(t)
	trips := th.planter.AllPaths()
	if len(trips) == 0 {
		t.Fatal("planter should expose trip paths")
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, trips[0], nil)
	req.RemoteAddr = "203.0.113.9:5555"
	th.ServeHTTP(rec, req)

	evs := sink.events()
	if len(evs) == 0 {
		t.Fatal("following a trip-URL should emit a detection event")
	}
	if evs[0].Confidence != 1.0 {
		t.Errorf("a trip hit is zero-false-positive (conf 1.0), got %v", evs[0].Confidence)
	}
	// The source must now be confirmed bad.
	if _, bad := th.confirmed.Load("203.0.113.9"); !bad {
		t.Error("source should be confirmed-bad after a trip")
	}
}

func TestTrapServesWatermarkedPoisonToConfirmedBad(t *testing.T) {
	th, _ := newTestTrap(t)
	th.confirmed.Store("198.51.100.7", struct{}{}) // pre-confirm

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/.env", nil)
	req.RemoteAddr = "198.51.100.7:4444"
	done := make(chan struct{})
	go func() { th.ServeHTTP(rec, req); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("trapped poison response did not complete in time")
	}
	body := rec.Body.String()
	if !strings.Contains(body, "=") {
		t.Fatalf("poison config should look like env assignments, got %q", body)
	}
	// At least one value must be watermarked as ours. Reconstruct the marker from
	// the same seed-derived key the handler uses.
	key := sha256.Sum256([]byte(fmt.Sprintf("heramba-trap:%d", int64(7))))
	marker, err := poison.NewMarker(key[:])
	if err != nil {
		t.Fatalf("marker: %v", err)
	}
	ours := false
	for _, line := range strings.Split(body, "\n") {
		if i := strings.IndexByte(line, '='); i > 0 {
			if marker.IsOurs(strings.TrimSpace(line[i+1:])) {
				ours = true
				break
			}
		}
	}
	if !ours {
		t.Error("served poison values should be watermarked as ours")
	}
}

func TestTrapBenignResponseCarriesBait(t *testing.T) {
	th, _ := newTestTrap(t)
	rec := httptest.NewRecorder()
	th.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Header().Get("Link") == "" {
		t.Error("benign response should carry a Link-header trip-URL")
	}
	if !strings.Contains(rec.Body.String(), "<!--") {
		t.Error("benign response should embed an HTML-comment trip-link")
	}
}

func TestLooksLikeSecretPath(t *testing.T) {
	for _, p := range []string{"/.env", "/config.json", "/backup.zip", "/.git/config", "/aws-credentials"} {
		if !looksLikeSecretPath(p) {
			t.Errorf("%q should look like a secret path", p)
		}
	}
	if looksLikeSecretPath("/about") {
		t.Error("/about should not look like a secret path")
	}
}
