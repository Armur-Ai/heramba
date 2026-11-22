// Package config loads and validates Heramba's runtime configuration. The safe
// default is OBSERVE mode: detection only, nothing inline. PROTECT mode (inline
// decoys/denies) must be set explicitly and requires an upstream.
package config

import (
	"fmt"
	"os"
	"strings"
	"time"

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
	Mode        Mode   `yaml:"mode"`
	Listen      string `yaml:"listen"`       // address the edge binds, e.g. ":8080"
	AdminListen string `yaml:"admin_listen"` // health/metrics address, e.g. ":9090" ("" = disabled)
	Upstream    string `yaml:"upstream"`     // reverse-proxy target, e.g. "http://127.0.0.1:3000"
	// TrustedProxies are CIDRs whose X-Forwarded-For header the edge will trust
	// for the real client IP. Empty (default) = never trust XFF, use the direct
	// peer address. Set this only to the CIDRs of your own load balancer.
	TrustedProxies []string         `yaml:"trusted_proxies"`
	Alert          AlertConfig      `yaml:"alert"`
	Decoys         DecoyConfig      `yaml:"decoys"`
	Egress         EgressConfig     `yaml:"egress"`
	Counter        CounterConfig    `yaml:"counter"`
	Classifier     ClassifierConfig `yaml:"classifier"`
	// TLS serves the edge over HTTPS with a static certificate when both
	// cert_file and key_file are set. ACME/auto-cert is intentionally out of
	// scope — terminate ACME at an upstream LB, or supply files here.
	TLS TLSConfig `yaml:"tls"`
	// RateLimitRPS throttles each source to this many requests/second in PROTECT
	// mode (0 = disabled; never engaged in observe mode). RateLimitBurst is the
	// allowed burst (defaults to max(RateLimitRPS, 1)).
	RateLimitRPS   float64 `yaml:"rate_limit_rps"`
	RateLimitBurst float64 `yaml:"rate_limit_burst"`
	// DrainSeconds is the graceful-drain window on shutdown: the admin readiness
	// probe flips to not-ready and the edge waits this many seconds before it
	// stops serving, so a load balancer deregisters it before in-flight traffic
	// is cut. 0 (default) = shut down immediately.
	DrainSeconds int `yaml:"drain_timeout"`
}

// DrainTimeout returns the graceful-drain window as a duration (never negative).
func (c Config) DrainTimeout() time.Duration {
	if c.DrainSeconds <= 0 {
		return 0
	}
	return time.Duration(c.DrainSeconds) * time.Second
}

// TLSConfig configures static-certificate HTTPS for the edge listener.
type TLSConfig struct {
	CertFile string `yaml:"cert_file"` // PEM certificate chain
	KeyFile  string `yaml:"key_file"`  // PEM private key
}

// TLSEnabled reports whether the edge should serve HTTPS (both files set).
func (c Config) TLSEnabled() bool {
	return c.TLS.CertFile != "" && c.TLS.KeyFile != ""
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
	Teams       string `yaml:"teams"`        // Microsoft Teams incoming-webhook URL (MessageCard)
	Discord     string `yaml:"discord"`      // Discord webhook URL (embed)
	GoogleChat  string `yaml:"google_chat"`  // Google Chat incoming-webhook URL (text)
	Mattermost  string `yaml:"mattermost"`   // Mattermost incoming-webhook URL (attachment)
	MinSeverity string `yaml:"min_severity"` // floor for push sinks: info|low|medium|high|critical (default high)
	// SIEM integrations — each fans out alongside the local text/jsonl record,
	// above MinSeverity. Enabled when its endpoint field is set.
	Splunk      SplunkConfig      `yaml:"splunk"`
	Datadog     DatadogConfig     `yaml:"datadog"`
	Elastic     ElasticConfig     `yaml:"elastic"`
	Syslog      SyslogConfig      `yaml:"syslog"`
	Loki        LokiConfig        `yaml:"loki"`
	NewRelic    NewRelicConfig    `yaml:"newrelic"`
	Honeycomb   HoneycombConfig   `yaml:"honeycomb"`
	OpenObserve OpenObserveConfig `yaml:"openobserve"`
	Axiom       AxiomConfig       `yaml:"axiom"`
	BetterStack BetterStackConfig `yaml:"betterstack"`
	Sumo        string            `yaml:"sumo"` // Sumo Logic HTTP Source URL (the URL is the secret)
	// TemplateWebhook POSTs a body rendered from an operator-supplied Go
	// text/template, so any service with a fixed payload shape can be targeted.
	TemplateWebhook TemplateWebhookConfig `yaml:"template_webhook"`
	// Rotate is an optional bounded on-disk JSONL audit trail (local, no
	// network). Unlike the push sinks it records everything, ignoring
	// MinSeverity — it is a durable record, not a notification channel.
	Rotate RotateConfig `yaml:"rotate"`
	// Email is an optional SMTP digest sink (batches events above MinSeverity
	// into a single message). Enabled when Host, From and To are all set.
	Email EmailConfig `yaml:"email"`
	// Paging integrations — default to a high floor (they wake a human).
	PagerDutyKey string `yaml:"pagerduty_key"` // PagerDuty Events API v2 routing key
	OpsgenieKey  string `yaml:"opsgenie_key"`  // Opsgenie API key
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

// TemplateWebhookConfig configures the generic templated webhook sink.
type TemplateWebhookConfig struct {
	URL         string `yaml:"url"`          // POST target (empty = disabled)
	Template    string `yaml:"template"`     // Go text/template rendered with the event
	ContentType string `yaml:"content_type"` // empty = application/json
}

// NewRelicConfig configures a New Relic Logs sink.
type NewRelicConfig struct {
	URL    string `yaml:"url"`     // empty = US Log API endpoint
	APIKey string `yaml:"api_key"` // New Relic license/ingest key
}

// HoneycombConfig configures a Honeycomb events sink.
type HoneycombConfig struct {
	URL     string `yaml:"url"`     // empty = US API (https://api.honeycomb.io)
	Dataset string `yaml:"dataset"` // Honeycomb dataset
	APIKey  string `yaml:"api_key"` // X-Honeycomb-Team key
}

// OpenObserveConfig configures an OpenObserve ingest sink.
type OpenObserveConfig struct {
	URL      string `yaml:"url"`      // instance root, e.g. http://localhost:5080
	Org      string `yaml:"org"`      // empty = default
	Stream   string `yaml:"stream"`   // empty = heramba
	Username string `yaml:"username"` // basic-auth user (email)
	Password string `yaml:"password"` // basic-auth password/token
}

// AxiomConfig configures an Axiom ingest sink.
type AxiomConfig struct {
	URL     string `yaml:"url"`     // empty = Axiom Cloud (https://api.axiom.co)
	Dataset string `yaml:"dataset"` // Axiom dataset
	Token   string `yaml:"token"`   // Bearer API token
}

// BetterStackConfig configures a BetterStack (Logtail) sink.
type BetterStackConfig struct {
	URL   string `yaml:"url"`   // the source's ingest URL
	Token string `yaml:"token"` // source token (Bearer)
}

// LokiConfig configures a Grafana Loki push sink.
type LokiConfig struct {
	URL    string `yaml:"url"`    // full push endpoint, e.g. http://loki:3100/loki/api/v1/push
	Tenant string `yaml:"tenant"` // optional X-Scope-OrgID for multi-tenant Loki
}

// SyslogConfig configures a CEF-over-syslog sink.
type SyslogConfig struct {
	Network string `yaml:"network"` // udp | tcp (default udp)
	Address string `yaml:"address"` // host:port
}

// EmailConfig configures the SMTP email digest sink.
type EmailConfig struct {
	Host      string   `yaml:"host"`       // SMTP server host (empty = disabled)
	Port      int      `yaml:"port"`       // SMTP port (0 = 25)
	From      string   `yaml:"from"`       // envelope + header From
	To        []string `yaml:"to"`         // recipients
	Username  string   `yaml:"username"`   // optional SMTP auth user (empty = no auth)
	Password  string   `yaml:"password"`   // SMTP auth password
	BatchSize int      `yaml:"batch_size"` // events per digest (0/1 = send each immediately)
}

// RotateConfig configures the rotating-file JSONL audit sink.
type RotateConfig struct {
	File       string `yaml:"file"`        // path to the active log (empty = disabled)
	MaxBytes   int64  `yaml:"max_bytes"`   // rotate at this size (0 = 10 MiB)
	MaxBackups int    `yaml:"max_backups"` // rotated files to keep (0 = truncate in place)
}

// DecoyConfig controls decoy/canary behavior.
type DecoyConfig struct {
	Enabled  bool     `yaml:"enabled"`
	Preset   string   `yaml:"preset"`   // framework preset (see `heramba decoys presets`)
	Paths    []string `yaml:"paths"`    // extra decoy paths to serve/watch
	Canaries []string `yaml:"canaries"` // canary tokens (planted elsewhere) to watch for in requests
	// OpenAPISpec is a path to an OpenAPI/Swagger JSON spec. When set, Heramba
	// derives a canary value for each sensitive-looking request-body field and
	// watches for it in request bodies — so an attacker replaying a planted spec
	// example is caught (zero false positive). CanarySalt keeps the derived
	// values unguessable; use the same salt when listing them to plant.
	OpenAPISpec string `yaml:"openapi_spec"`
	CanarySalt  string `yaml:"canary_salt"`
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
	if s := c.Alert.MinSeverity; s != "" && !validMinSeverity(s) {
		return fmt.Errorf("config: invalid alert.min_severity %q (want info|low|medium|high|critical)", s)
	}
	if n := c.Alert.Syslog.Network; n != "" && n != "udp" && n != "tcp" {
		return fmt.Errorf("config: invalid alert.syslog.network %q (want udp|tcp)", n)
	}
	if (c.TLS.CertFile == "") != (c.TLS.KeyFile == "") {
		return fmt.Errorf("config: tls.cert_file and tls.key_file must both be set or both empty")
	}
	return nil
}

func validMinSeverity(s string) bool {
	switch strings.ToLower(s) {
	case "info", "low", "medium", "high", "critical":
		return true
	}
	return false
}
