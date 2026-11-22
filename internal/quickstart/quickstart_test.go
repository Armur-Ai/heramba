package quickstart

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mk(t *testing.T, dir, rel, content string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDetectStacks(t *testing.T) {
	cases := []struct {
		marker, content, want string
	}{
		{"artisan", "", "laravel"},
		{"manage.py", "", "django"},
		{"Gemfile", "source 'https://rubygems.org'\n", "rails"},
		{"pom.xml", "<project/>", "spring"},
		{"wp-config.php", "<?php", "wordpress"},
		{"next.config.js", "module.exports={}", "next"},
		{"package.json", `{"dependencies":{"express":"^4"}}`, "node"},
		{"go.mod", "module x\n", "go"},
		{"mix.exs", "defmodule X do\nend\n", "phoenix"},
		{"appsettings.json", "{}", "aspnet"},
	}
	for _, c := range cases {
		dir := t.TempDir()
		mk(t, dir, c.marker, c.content)
		if got := Detect(dir).Name; got != c.want {
			t.Errorf("marker %s => %q, want %q", c.marker, got, c.want)
		}
	}
}

func TestDetectNextViaPackageDep(t *testing.T) {
	dir := t.TempDir()
	mk(t, dir, "package.json", `{"dependencies":{"next":"14.2.0","react":"18"}}`)
	if got := Detect(dir).Name; got != "next" {
		t.Fatalf("next dep should detect next, got %q", got)
	}
}

func TestDetectUnknown(t *testing.T) {
	if Detect(t.TempDir()).Name != "" {
		t.Fatal("empty dir should be unknown")
	}
}

func TestGenerateConfigIncludesPresetAndUpstream(t *testing.T) {
	cfg := GenerateConfig(Stack{"rails", "Gemfile"}, "http://app:3000")
	for _, want := range []string{"mode: observe", "preset: rails", `upstream: "http://app:3000"`, "admin_listen"} {
		if !strings.Contains(cfg, want) {
			t.Errorf("generated config missing %q", want)
		}
	}
}

func TestGenerateConfigUnknownStackOmitsPreset(t *testing.T) {
	cfg := GenerateConfig(Stack{}, "")
	if strings.Contains(cfg, "preset:") {
		t.Error("unknown stack must not emit a preset line")
	}
}
