package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultIsObserve(t *testing.T) {
	d := Default()
	if d.Mode != ModeObserve {
		t.Fatalf("default mode = %q, want observe", d.Mode)
	}
	if err := d.Validate(); err != nil {
		t.Fatalf("default config invalid: %v", err)
	}
}

func TestProtectRequiresUpstream(t *testing.T) {
	c := Default()
	c.Mode = ModeProtect
	if err := c.Validate(); err == nil {
		t.Fatal("protect mode without upstream must be invalid")
	}
	c.Upstream = "http://127.0.0.1:3000"
	if err := c.Validate(); err != nil {
		t.Fatalf("protect+upstream should be valid: %v", err)
	}
}

func TestLoadAppliesDefaults(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".heramba.yml")
	if err := os.WriteFile(p, []byte("upstream: http://127.0.0.1:3000\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if c.Mode != ModeObserve || c.Listen != ":8080" {
		t.Fatalf("defaults not applied: %+v", c)
	}
}

func TestLoadRejectsBadMode(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".heramba.yml")
	_ = os.WriteFile(p, []byte("mode: destroy\n"), 0o644)
	if _, err := Load(p); err == nil {
		t.Fatal("invalid mode must error")
	}
}
