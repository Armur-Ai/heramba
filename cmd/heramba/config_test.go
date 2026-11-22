package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"

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

// TestExampleConfigParsesAndValidates guards the `config example` reference:
// it must load and Validate as-is (observe mode, empty endpoints), and must not
// contain any key the config schema does not know (KnownFields catches drift
// such as a renamed or misspelled option).
func TestExampleConfigParsesAndValidates(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".heramba.yml")
	if err := os.WriteFile(p, []byte(exampleConfigYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(p)
	if err != nil {
		t.Fatalf("config example must load+validate: %v", err)
	}
	if cfg.Mode != config.ModeObserve {
		t.Errorf("example must stay observe mode, got %q", cfg.Mode)
	}

	// Strict decode: every key in the example must map to a real struct field.
	dec := yaml.NewDecoder(bytes.NewReader([]byte(exampleConfigYAML)))
	dec.KnownFields(true)
	var strict config.Config
	if err := dec.Decode(&strict); err != nil {
		t.Fatalf("example has an unknown/misspelled option: %v", err)
	}
}

// TestExampleConfigCoversEverySink ensures the reference actually documents all
// the alert sinks — so a newly added sink is not silently left out.
func TestExampleConfigCoversEverySink(t *testing.T) {
	for _, want := range []string{
		"webhook:", "slack:", "teams:", "discord:",
		"splunk:", "datadog:", "elastic:", "syslog:", "rotate:", "email:",
		"pagerduty_key:", "opsgenie_key:",
		"preset:", "admin_listen:", "counter:", "egress:", "classifier:",
	} {
		if !bytes.Contains([]byte(exampleConfigYAML), []byte(want)) {
			t.Errorf("config example missing documented option %q", want)
		}
	}
}
