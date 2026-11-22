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
	Mode     Mode   `yaml:"mode"`
	Listen   string `yaml:"listen"`   // address the edge binds, e.g. ":8080"
	Upstream string `yaml:"upstream"` // reverse-proxy target, e.g. "http://127.0.0.1:3000"
	// TrustedProxies are CIDRs whose X-Forwarded-For header the edge will trust
	// for the real client IP. Empty (default) = never trust XFF, use the direct
	// peer address. Set this only to the CIDRs of your own load balancer.
	TrustedProxies []string         `yaml:"trusted_proxies"`
	Alert          AlertConfig      `yaml:"alert"`
	Decoys         DecoyConfig      `yaml:"decoys"`
	Egress         EgressConfig     `yaml:"egress"`
	Counter        CounterConfig    `yaml:"counter"`
	Classifier     ClassifierConfig `yaml:"classifier"`
	// RateLimitRPS throttles each source to this many requests/second in PROTECT
	// mode (0 = disabled; never engaged in observe mode). RateLimitBurst is the
	// allowed burst (defaults to max(RateLimitRPS, 1)).
	RateLimitRPS   float64 `yaml:"rate_limit_rps"`
	RateLimitBurst float64 `yaml:"rate_limit_burst"`
}

// ClassifierConfig overrides the agentic-traffic classifier thresholds. Zero
// values keep the conservative built-in defaults.
type ClassifierConfig struct {
	CadenceRPS   float64 `yaml:"cadence_rps"`   // high_cadence fires above this req/s
	EnumDistinct int     `yaml:"enum_distinct"` // enumeration fires at/above this many distinct paths
	ToolAt       float64 `yaml:"tool_at"`       // score >= this => "tool"
	AgenticAt    float64 `yaml:"agentic_at"`    // score >= this => "agentic-suspected"
}

// AlertConfig controls where detections go.
type AlertConfig struct {
	Format string `yaml:"format"` // "text" | "jsonl"
	File   string `yaml:"file"`   // path, or "" for stdout
	// Webhook and Slack are optional push integrations, fanned out alongside the
	// text/jsonl sink. Both honor MinSeverity so a busy edge does not flood a
	// channel; the local text/jsonl sink always records everything.
	Webhook     string `yaml:"webhook"`      // POST raw event JSON to this URL
	Slack       string `yaml:"slack"`        // Slack incoming-webhook URL (Block Kit)
	MinSeverity string `yaml:"min_severity"` // floor for push sinks: info|low|medium|high|critical (default high)
	// SIEM integrations — each fans out alongside the local text/jsonl record,
	// above MinSeverity. Enabled when its endpoint field is set.
	Splunk  SplunkConfig  `yaml:"splunk"`
	Datadog DatadogConfig `yaml:"datadog"`
	Elastic ElasticConfig `yaml:"elastic"`
	Syslog  SyslogConfig  `yaml:"syslog"`
}

// SplunkConfig configures a Splunk HTTP Event Collector sink.
type SplunkConfig struct {
	URL   string `yaml:"url"`   // full HEC collector URL
	Token string `yaml:"token"` // HEC token
}

// DatadogConfig configures a Datadog Logs sink.
type DatadogConfig struct {
	URL    string `yaml:"url"`     // empty = US1 intake
	APIKey string `yaml:"api_key"` // Datadog API key
}

// ElasticConfig configures an Elasticsearch sink.
type ElasticConfig struct {
	URL    string `yaml:"url"`     // Elasticsearch base URL
	Index  string `yaml:"index"`   // empty = heramba-detections
	APIKey string `yaml:"api_key"` // optional ApiKey auth
}

// SyslogConfig configures a CEF-over-syslog sink.
type SyslogConfig struct {
	Network string `yaml:"network"` // udp | tcp (default udp)
	Address string `yaml:"address"` // host:port
}

// DecoyConfig controls decoy/canary behavior.
type DecoyConfig struct {
	Enabled  bool     `yaml:"enabled"`
	Preset   string   `yaml:"preset"`   // framework preset: node|next|rails|django|laravel|wordpress|spring
	Paths    []string `yaml:"paths"`    // extra decoy paths to serve/watch
	Canaries []string `yaml:"canaries"` // canary tokens (planted elsewhere) to watch for in requests
}

// CounterConfig controls counter-agent prompt-decoy injection (PROTECT mode).
// Off by default: when enabled, HTML/JS responses served to classifier-flagged
// agentic sources get a fabricated-lead comment injected (invisible to humans),
// whose fake paths become decoys and whose key is watched as a canary.
type CounterConfig struct {
	Enabled bool `yaml:"enabled"`
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
