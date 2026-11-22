package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
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

func findingsHave(fs []lintFinding, level lintLevel, substr string) bool {
	for _, f := range fs {
		if f.Level == level && strings.Contains(f.Msg, substr) {
			return true
		}
	}
	return false
}

func TestLintCleanConfigHasNoErrors(t *testing.T) {
	cfg := config.Default()
	cfg.AdminListen = ":9090"
	cfg.Alert.Slack = "https://hooks.example/abc" // a push sink, so no "no sinks" warn
	fs := lintConfig(cfg, nil)
	if lintHasError(fs) {
		t.Fatalf("clean config should have no error findings: %+v", fs)
	}
}

func TestLintUnknownPresetIsError(t *testing.T) {
	cfg := config.Default()
	cfg.Decoys.Preset = "cobol"
	fs := lintConfig(cfg, nil)
	if !findingsHave(fs, lintError, "unknown decoys.preset") {
		t.Errorf("expected unknown-preset error, got %+v", fs)
	}
	if !lintHasError(fs) {
		t.Error("lintHasError should be true")
	}
}

func TestLintPortCollisionIsError(t *testing.T) {
	cfg := config.Default()
	cfg.Listen = ":9090"
	cfg.AdminListen = ":9090"
	fs := lintConfig(cfg, nil)
	if !findingsHave(fs, lintError, "must differ from listen") {
		t.Errorf("expected port-collision error, got %+v", fs)
	}
}

func TestLintNoSinksWarns(t *testing.T) {
	cfg := config.Default() // no push sinks
	fs := lintConfig(cfg, nil)
	if !findingsHave(fs, lintWarn, "no webhook/chat/SIEM") {
		t.Errorf("expected no-sinks warning, got %+v", fs)
	}
	if lintHasError(fs) {
		t.Errorf("no-sinks alone must not be an error: %+v", fs)
	}
}

func TestLintEgressDenyAllWarns(t *testing.T) {
	cfg := config.Default()
	cfg.Alert.Slack = "https://x"
	cfg.Egress.Enabled = true
	cfg.Egress.AllowedHosts = nil
	fs := lintConfig(cfg, nil)
	if !findingsHave(fs, lintWarn, "denies ALL app callbacks") {
		t.Errorf("expected egress deny-all warning, got %+v", fs)
	}
}

func TestLintRateLimitInObserveWarns(t *testing.T) {
	cfg := config.Default()
	cfg.Alert.Slack = "https://x"
	cfg.RateLimitRPS = 10
	fs := lintConfig(cfg, nil)
	if !findingsHave(fs, lintWarn, "only engages in protect") {
		t.Errorf("expected rate-limit-observe warning, got %+v", fs)
	}
}

func TestLintProtectWithoutUpstreamIsError(t *testing.T) {
	cfg := config.Default()
	cfg.Mode = config.ModeProtect
	cfg.Upstream = "" // invalid: protect needs upstream
	cfg.Alert.Slack = "https://x"
	fs := lintConfig(cfg, nil)
	if !lintHasError(fs) {
		t.Errorf("protect without upstream must be an error: %+v", fs)
	}
}
