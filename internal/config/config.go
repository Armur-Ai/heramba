// Package config loads and validates Heramba's runtime configuration. The safe
// default is OBSERVE mode: detection only, nothing inline. PROTECT mode (inline
// decoys/denies) must be set explicitly and requires an upstream.
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Mode is the engine's posture.
type Mode string

const (
	// ModeObserve is the default: classify + detect, pass traffic through
	// untouched. Zero inline risk.
	ModeObserve Mode = "observe"
	// ModeProtect acts inline: serve decoys, deny out-of-policy egress, tarpit.
	ModeProtect Mode = "protect"
)

// Config is the engine configuration (file: .heramba.yml).
type Config struct {
	Mode     Mode         `yaml:"mode"`
	Listen   string       `yaml:"listen"`   // address the edge binds, e.g. ":8080"
	Upstream string       `yaml:"upstream"` // reverse-proxy target, e.g. "http://127.0.0.1:3000"
	Alert    AlertConfig  `yaml:"alert"`
	Decoys   DecoyConfig  `yaml:"decoys"`
	Egress   EgressConfig `yaml:"egress"`
}

// AlertConfig controls where detections go.
type AlertConfig struct {
	Format string `yaml:"format"` // "text" | "jsonl"
	File   string `yaml:"file"`   // path, or "" for stdout
}

// DecoyConfig controls decoy/canary behavior.
type DecoyConfig struct {
	Enabled bool     `yaml:"enabled"`
	Paths   []string `yaml:"paths"` // extra decoy paths to serve/watch
}

// EgressConfig controls the callback firewall (PROTECT mode).
type EgressConfig struct {
	Enabled      bool     `yaml:"enabled"`
	AllowedHosts []string `yaml:"allowed_hosts"` // hosts the app is permitted to call
}

// Default returns the safe default configuration (observe mode).
func Default() Config {
	return Config{
		Mode:   ModeObserve,
		Listen: ":8080",
		Alert:  AlertConfig{Format: "text"},
		Decoys: DecoyConfig{Enabled: true},
		Egress: EgressConfig{Enabled: false},
	}
}

// Load reads and validates a config file, applying defaults for empty fields.
func Load(path string) (Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("config: parse %s: %w", path, err)
	}
	if cfg.Mode == "" {
		cfg.Mode = ModeObserve
	}
	if cfg.Listen == "" {
		cfg.Listen = ":8080"
	}
	if err := cfg.Validate(); err != nil {
		return cfg, err
	}
	return cfg, nil
}

// Validate enforces invariants. A reverse-proxy needs an upstream; PROTECT mode
// especially must not run without one (it would have nothing to protect and no
// pass-through for legitimate traffic).
func (c Config) Validate() error {
	switch c.Mode {
	case ModeObserve, ModeProtect:
	default:
		return fmt.Errorf("config: invalid mode %q (want observe|protect)", c.Mode)
	}
	if c.Mode == ModeProtect && c.Upstream == "" {
		return fmt.Errorf("config: protect mode requires an upstream")
	}
	if c.Alert.Format != "" && c.Alert.Format != "text" && c.Alert.Format != "jsonl" {
		return fmt.Errorf("config: invalid alert.format %q (want text|jsonl)", c.Alert.Format)
	}
	return nil
}
