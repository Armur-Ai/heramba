package edge

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Armur-Ai/heramba/internal/config"
	"github.com/Armur-Ai/heramba/internal/event"
)

type capSink struct{ events []event.Event }

func (c *capSink) Emit(e event.Event) error { c.events = append(c.events, e); return nil }

func (c *capSink) kinds() map[event.Kind]int {
	m := map[event.Kind]int{}
	for _, e := range c.events {
		m[e.Kind]++
	}
	return m
}

func newUpstream(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Powered-By", "PHP/8.1.2") // leaky header
		w.Header().Set("Server", "nginx/1.25.1")
		_, _ = w.Write([]byte("real app: " + r.URL.Path))
	}))
}

func TestObserveModePassesRealTrafficThrough(t *testing.T) {
	up := newUpstream(t)
	defer up.Close()
	cfg := config.Default()
	cfg.Upstream = up.URL
	sink := &capSink{}
	e, err := New(cfg, sink)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(e.Handler())
	defer srv.Close()

	resp, _ := http.Get(srv.URL + "/api/users")
	// Observe mode must NOT strip the leaky header (nothing inline).
	if resp.Header.Get("X-Powered-By") == "" {
		t.Fatal("observe mode should not rewrite response headers")
	}
}

func TestProtectModeNormalizesHeaders(t *testing.T) {
	up := newUpstream(t)
	defer up.Close()
	cfg := config.Default()
	cfg.Mode = config.ModeProtect
	cfg.Upstream = up.URL
	e, err := New(cfg, &capSink{})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(e.Handler())
	defer srv.Close()

	resp, _ := http.Get(srv.URL + "/api/users")
	if resp.Header.Get("X-Powered-By") != "" {
		t.Fatal("protect mode should strip leaky X-Powered-By")
	}
	if resp.Header.Get("Server") != "heramba" {
		t.Fatalf("protect mode should normalize Server, got %q", resp.Header.Get("Server"))
	}
}

func TestDecoyHitDetectedAndServedInBothModes(t *testing.T) {
	up := newUpstream(t)
	defer up.Close()
	cfg := config.Default() // observe
	cfg.Upstream = up.URL
	sink := &capSink{}
	e, _ := New(cfg, sink)
	srv := httptest.NewServer(e.Handler())
	defer srv.Close()

	resp, _ := http.Get(srv.URL + "/.env")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("decoy should serve 404, got %d", resp.StatusCode)
	}
	if sink.kinds()[event.KindDecoyHit] != 1 {
		t.Fatalf("expected 1 decoy_hit event, got %+v", sink.kinds())
	}
}

func TestHoneypotOnlyNoUpstream(t *testing.T) {
	cfg := config.Default() // observe, no upstream
	e, err := New(cfg, &capSink{})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(e.Handler())
	defer srv.Close()
	resp, _ := http.Get(srv.URL + "/anything")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("honeypot-only should 404 non-decoys, got %d", resp.StatusCode)
	}
}

func TestHoneypotFallthroughHandler(t *testing.T) {
	cfg := config.Default() // observe, no upstream
	e, err := New(cfg, &capSink{})
	if err != nil {
		t.Fatal(err)
	}
	// Install a deception-style fallthrough; non-decoy paths should reach it
	// instead of a neutral 404.
	e.SetHoneypot(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("synthetic-app:" + r.URL.Path))
	}))
	srv := httptest.NewServer(e.Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/projects")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("honeypot fallthrough should serve the app, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "synthetic-app:/api/projects" {
		t.Fatalf("fallthrough body wrong: %q", body)
	}

	// A decoy path must STILL be handled by the edge (decoy served), not the
	// fallthrough.
	resp2, _ := http.Get(srv.URL + "/.env")
	defer resp2.Body.Close()
	b2, _ := io.ReadAll(resp2.Body)
	if string(b2) == "synthetic-app:/.env" {
		t.Fatal("decoy path should be served by the edge, not the honeypot fallthrough")
	}
}
