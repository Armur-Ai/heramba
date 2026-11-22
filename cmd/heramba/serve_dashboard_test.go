package main

import (
	"testing"

	"github.com/Armur-Ai/heramba/internal/config"
	"github.com/Armur-Ai/heramba/internal/edge"
)

func newTestEngine(t *testing.T) (*edge.Engine, config.Config) {
	t.Helper()
	cfg := config.Default()
	cfg.Mode = config.ModeObserve
	cfg.Upstream = "http://127.0.0.1:3000" // protect mode requires one
	eng, err := edge.New(cfg, nil)
	if err != nil {
		t.Fatalf("build engine: %v", err)
	}
	return eng, cfg
}

func TestLiveControlSetModeHotSwaps(t *testing.T) {
	eng, cfg := newTestEngine(t)
	lc := newLiveControl(cfg, eng)

	if lc.Mode() != "observe" {
		t.Fatalf("initial mode = %q, want observe", lc.Mode())
	}
	if err := lc.SetMode("protect"); err != nil {
		t.Fatalf("SetMode(protect): %v", err)
	}
	if lc.Mode() != "protect" {
		t.Errorf("after SetMode, mode = %q, want protect", lc.Mode())
	}
	// Case-insensitive and trims whitespace.
	if err := lc.SetMode("  Observe "); err != nil {
		t.Fatalf("SetMode(Observe): %v", err)
	}
	if lc.Mode() != "observe" {
		t.Errorf("mode = %q, want observe", lc.Mode())
	}
}

func TestLiveControlSetModeRejectsUnknown(t *testing.T) {
	eng, cfg := newTestEngine(t)
	lc := newLiveControl(cfg, eng)
	if err := lc.SetMode("paranoid"); err == nil {
		t.Error("unknown mode should be rejected")
	}
	if lc.Mode() != "observe" {
		t.Error("a rejected SetMode must not change the live mode")
	}
}

func TestLiveControlProtectRequiresUpstream(t *testing.T) {
	cfg := config.Default()
	cfg.Mode = config.ModeObserve // no upstream
	eng, err := edge.New(cfg, nil)
	if err != nil {
		t.Fatalf("build engine: %v", err)
	}
	lc := newLiveControl(cfg, eng)
	if err := lc.SetMode("protect"); err == nil {
		t.Error("switching to protect with no upstream should be rejected by validation")
	}
	if lc.Mode() != "observe" {
		t.Error("a rejected mode change must leave the edge in its prior mode")
	}
}

func TestLiveControlApplyPreset(t *testing.T) {
	eng, cfg := newTestEngine(t)
	lc := newLiveControl(cfg, eng)
	if err := lc.ApplyDecoyPreset("node"); err != nil {
		t.Fatalf("ApplyDecoyPreset(node): %v", err)
	}
	if err := lc.ApplyDecoyPreset("cobol-on-cogs"); err == nil {
		t.Error("unknown preset should be rejected")
	}
}

func TestLiveControlPauseAndClearUnsupported(t *testing.T) {
	eng, cfg := newTestEngine(t)
	lc := newLiveControl(cfg, eng)
	if lc.SetPaused(true) == nil {
		t.Error("SetPaused should be unsupported on a live edge")
	}
	if lc.Clear() == nil {
		t.Error("Clear should be unsupported on a live edge")
	}
}

func TestLiveControlSetConfigSyncsMode(t *testing.T) {
	eng, cfg := newTestEngine(t)
	lc := newLiveControl(cfg, eng)
	reloaded := cfg
	reloaded.Mode = config.ModeProtect
	lc.setConfig(reloaded)
	if lc.Mode() != "protect" {
		t.Errorf("setConfig should update the reported mode, got %q", lc.Mode())
	}
}
