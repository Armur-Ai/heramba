package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Armur-Ai/heramba/internal/alert"
	"github.com/Armur-Ai/heramba/internal/config"
	"github.com/Armur-Ai/heramba/internal/edge"
	"github.com/Armur-Ai/heramba/internal/event"
)

func TestReloadServeAppliesConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".heramba.yml")
	if err := os.WriteFile(path, []byte("mode: observe\ndecoys:\n  preset: rails\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Mirror serve.go wiring: counting wraps the swap, so metrics keep counting
	// while reloadServe swaps the alert target underneath.
	swap := alert.NewSwap(alert.Discard)
	count := alert.Count(alert.MultiSink{swap})
	engine, err := edge.New(config.Default(), count)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := reloadServe(path, swap, engine)
	if err != nil {
		t.Fatalf("reloadServe: %v", err)
	}
	if cfg.Decoys.Preset != "rails" {
		t.Errorf("reloaded preset = %q, want rails", cfg.Decoys.Preset)
	}

	// The rails preset lure should now be a decoy on the live engine.
	req := httptest.NewRequest(http.MethodGet, "/config/master.key", nil)
	req.RemoteAddr = "203.0.113.5:4444"
	engine.ServeHTTP(httptest.NewRecorder(), req)

	snap := count.Snapshot()
	var decoyHits int
	for _, tl := range snap {
		if tl.Kind == event.KindDecoyHit {
			decoyHits = tl.Count
		}
	}
	if decoyHits != 1 {
		t.Errorf("expected the reloaded preset decoy to fire once, snapshot=%v", snap)
	}
}

func TestReloadServeBadConfigIsSkipped(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".heramba.yml")
	// protect mode without upstream fails Validate.
	if err := os.WriteFile(path, []byte("mode: protect\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	swap := alert.NewSwap(alert.Count(nil))
	engine, err := edge.New(config.Default(), swap)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reloadServe(path, swap, engine); err == nil {
		t.Fatal("reloadServe should reject an invalid config")
	}
}

func TestReloadServeMissingFileIsSkipped(t *testing.T) {
	swap := alert.NewSwap(alert.Count(nil))
	engine, _ := edge.New(config.Default(), swap)
	if _, err := reloadServe(filepath.Join(t.TempDir(), "nope.yml"), swap, engine); err == nil {
		t.Fatal("reloadServe should error on a missing config file")
	}
}
