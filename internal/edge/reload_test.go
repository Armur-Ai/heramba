package edge

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Armur-Ai/heramba/internal/config"
	"github.com/Armur-Ai/heramba/internal/event"
)

func serveGet(e *Engine, path string) {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.RemoteAddr = "203.0.113.9:5555"
	e.ServeHTTP(httptest.NewRecorder(), req)
}

func TestReloadSwapsDecoySurfaceAndSink(t *testing.T) {
	// Start with no preset: /config/master.key is NOT a decoy yet.
	cfg1 := config.Default()
	sink1 := &capSink{}
	e, err := New(cfg1, sink1)
	if err != nil {
		t.Fatal(err)
	}
	serveGet(e, "/config/master.key")
	if n := sink1.kinds()[event.KindDecoyHit]; n != 0 {
		t.Fatalf("pre-reload: path should not be a decoy, got %d hits", n)
	}

	// Reload with the rails preset (adds /config/master.key) and a fresh sink.
	cfg2 := config.Default()
	cfg2.Decoys.Preset = "rails"
	sink2 := &capSink{}
	if err := e.Reload(cfg2, sink2); err != nil {
		t.Fatalf("reload: %v", err)
	}
	serveGet(e, "/config/master.key")

	if n := sink2.kinds()[event.KindDecoyHit]; n != 1 {
		t.Errorf("post-reload: expected 1 decoy hit on the new sink, got %d", n)
	}
	// The old sink must NOT receive the post-reload event (sink swapped).
	if n := sink1.kinds()[event.KindDecoyHit]; n != 0 {
		t.Errorf("old sink should not receive post-reload events, got %d", n)
	}
}

func TestReloadToProtectModeDeniesCanary(t *testing.T) {
	cfg := config.Default() // observe
	sink := &capSink{}
	e, err := New(cfg, sink)
	if err != nil {
		t.Fatal(err)
	}
	// Watch a canary via config, flip to protect on reload.
	cfg2 := config.Default()
	cfg2.Mode = config.ModeProtect
	cfg2.Upstream = "http://127.0.0.1:1" // required by protect; never dialed here
	cfg2.Decoys.Canaries = []string{"hrmb_deadbeef"}
	if err := e.Reload(cfg2, sink); err != nil {
		t.Fatalf("reload: %v", err)
	}
	// A request carrying the watched canary in a header must be detected.
	req := httptest.NewRequest(http.MethodGet, "/account", nil)
	req.RemoteAddr = "203.0.113.9:5555"
	req.Header.Set("Authorization", "Bearer hrmb_deadbeef")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if sink.kinds()[event.KindCanaryTriggered] != 1 {
		t.Errorf("expected canary detection after reload to protect, kinds=%v", sink.kinds())
	}
	// Protect mode denies the canary request with a neutral 404.
	if rec.Code != http.StatusNotFound {
		t.Errorf("protect mode should serve neutral 404 on canary, got %d", rec.Code)
	}
}

func TestReloadBadUpstreamLeavesEngineServing(t *testing.T) {
	cfg := config.Default()
	sink := &capSink{}
	e, err := New(cfg, sink)
	if err != nil {
		t.Fatal(err)
	}
	bad := config.Default()
	bad.Upstream = "http://%zz" // unparseable
	if err := e.Reload(bad, sink); err == nil {
		t.Fatal("reload with bad upstream should error")
	}
	// The engine must still serve on the pre-reload state (a default decoy still fires).
	serveGet(e, "/.env")
	if sink.kinds()[event.KindDecoyHit] != 1 {
		t.Errorf("engine should keep serving old state after failed reload, kinds=%v", sink.kinds())
	}
}
