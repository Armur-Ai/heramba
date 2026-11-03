package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Armur-Ai/heramba/internal/config"
)

// TestDefaultConfigTemplateParses guards against the `config init` template
// drifting out of sync with the real schema: whatever it writes must load and
// validate, and stay observe-mode by default.
func TestDefaultConfigTemplateParses(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".heramba.yml")
	if err := os.WriteFile(p, []byte(defaultConfigYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(p)
	if err != nil {
		t.Fatalf("config init template must load+validate: %v", err)
	}
	if cfg.Mode != config.ModeObserve {
		t.Fatalf("default template must be observe mode, got %q", cfg.Mode)
	}
}
