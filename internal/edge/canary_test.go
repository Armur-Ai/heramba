package edge

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Armur-Ai/heramba/internal/config"
	"github.com/Armur-Ai/heramba/internal/decoy"
	"github.com/Armur-Ai/heramba/internal/event"
)

func TestCanaryUseDetectedInObserveAndProxied(t *testing.T) {
	up := newUpstream(t)
	defer up.Close()
	token, _ := decoy.NewCanaryStore().Mint("test")
	cfg := config.Default()
	cfg.Upstream = up.URL
	cfg.Decoys.Canaries = []string{token.Token}
	sink := &capSink{}
	e, _ := New(cfg, sink)
	srv := httptest.NewServer(e.Handler())
	defer srv.Close()

	// Attacker uses the planted canary as an auth header.
	req, _ := http.NewRequest("GET", srv.URL+"/api/data", nil)
	req.Header.Set("Authorization", "Bearer "+token.Token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	// Observe mode: still proxied (200 from upstream).
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("observe should proxy, got %d", resp.StatusCode)
	}
	if sink.kinds()[event.KindCanaryTriggered] != 1 {
		t.Fatalf("expected 1 canary_triggered event, got %+v", sink.kinds())
	}
}

func TestCanaryUseDeniedInProtect(t *testing.T) {
	up := newUpstream(t)
	defer up.Close()
	token, _ := decoy.NewCanaryStore().Mint("test")
	cfg := config.Default()
	cfg.Mode = config.ModeProtect
	cfg.Upstream = up.URL
	cfg.Decoys.Canaries = []string{token.Token}
	sink := &capSink{}
	e, _ := New(cfg, sink)
	srv := httptest.NewServer(e.Handler())
	defer srv.Close()

	resp, _ := http.Get(srv.URL + "/api/data?key=" + token.Token)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("protect should deny a canary request with 404, got %d", resp.StatusCode)
	}
	if sink.kinds()[event.KindCanaryTriggered] != 1 {
		t.Fatalf("expected 1 canary_triggered event, got %+v", sink.kinds())
	}
}

func TestNoCanaryNoEvent(t *testing.T) {
	up := newUpstream(t)
	defer up.Close()
	cfg := config.Default()
	cfg.Upstream = up.URL
	cfg.Decoys.Canaries = []string{"hrmb_deadbeefdeadbeefdeadbeefdeadbeef"}
	sink := &capSink{}
	e, _ := New(cfg, sink)
	srv := httptest.NewServer(e.Handler())
	defer srv.Close()
	_, _ = http.Get(srv.URL + "/api/data")
	if sink.kinds()[event.KindCanaryTriggered] != 0 {
		t.Fatal("no canary present should emit no canary event")
	}
}
