package main

import (
	"fmt"
	"os"

	"github.com/Armur-Ai/heramba/internal/config"
	"github.com/Armur-Ai/heramba/internal/decoy"
)

const defaultConfigYAML = `# Heramba configuration (.heramba.yml)
# Safe default posture is OBSERVE: detection only, nothing inline.

mode: observe            # observe | protect
listen: ":8080"          # address the edge binds
upstream: ""             # reverse-proxy target, e.g. http://127.0.0.1:3000
                         # (empty = honeypot-only: decoys + 404)

# X-Forwarded-For is trusted ONLY from these proxy CIDRs (empty = never trust it,
# use the direct peer address). Set to your load balancer's CIDRs.
trusted_proxies: []

alert:
  format: text           # text | jsonl
  file: ""               # path, or empty for stdout

decoys:
  enabled: true
  paths: []              # extra decoy paths to serve/watch
  canaries: []           # canary tokens (planted elsewhere) to watch for in requests

# Agentic-traffic classifier thresholds (0/empty = conservative built-in defaults).
classifier:
  cadence_rps: 0         # high_cadence fires above this req/s (default 5)
  enum_distinct: 0       # enumeration fires at/above this many distinct paths (default 12)
  tool_at: 0             # score >= this => "tool" (default 0.4)
  agentic_at: 0          # score >= this => "agentic-suspected" (default 0.7)

# Per-source request rate limit (PROTECT mode only; 0 = disabled).
rate_limit_rps: 0
rate_limit_burst: 0      # defaults to max(rate_limit_rps, 1)

egress:                  # callback firewall (protect mode)
  enabled: false
  allowed_hosts: []      # hosts the protected app is permitted to call

# Counter-agent prompt decoys (PROTECT mode): inject fabricated vuln leads into
# HTML/JS responses for classifier-flagged agentic sources (invisible to humans;
# the fake paths become decoys and the key is watched as a canary). Off by default.
counter:
  enabled: false
`

// exampleConfigYAML is a fully-commented reference config exercising EVERY
// option — every alert sink, decoy preset, egress/counter/classifier knob — so
// an operator can copy just the lines they need. It stays observe-mode with all
// push endpoints empty, so it both parses and validates as-is (guarded by a test).
const exampleConfigYAML = `# Heramba — full reference configuration (.heramba.yml)
# Every option is shown with its default/example value and a note. Safe default
# posture is OBSERVE: detection only, nothing inline. Delete what you don't need.

mode: observe              # observe | protect  (protect acts inline and requires an upstream)
listen: ":8080"            # address the edge binds
admin_listen: ":9090"      # health + metrics + version (/healthz /readyz /metrics /version); "" disables
upstream: ""               # reverse-proxy target, e.g. http://127.0.0.1:3000 ("" = honeypot-only)

# Serve the edge over HTTPS with a static cert when BOTH are set (no ACME —
# terminate ACME at an upstream LB, or supply files here). Both or neither.
tls:
  cert_file: ""            # PEM certificate chain
  key_file: ""             # PEM private key

# X-Forwarded-For is trusted ONLY from these proxy CIDRs ("" / empty = never trust
# it, use the direct peer address). Set to your load balancer's CIDRs.
trusted_proxies: []

alert:
  format: text             # text | jsonl  (local record; always captures everything)
  file: ""                 # path, or "" for stdout
  min_severity: high       # floor for PUSH sinks below: info|low|medium|high|critical

  # --- Push integrations (fan out above min_severity) ---
  webhook: ""              # POST raw event JSON to this URL
  slack: ""                # Slack incoming-webhook URL (Block Kit)
  teams: ""                # Microsoft Teams incoming-webhook URL (MessageCard)
  discord: ""              # Discord webhook URL (embed)
  google_chat: ""          # Google Chat incoming-webhook URL (text)
  mattermost: ""           # Mattermost incoming-webhook URL (attachment)

  # --- SIEM integrations (enabled when the endpoint is set) ---
  splunk:
    url: ""                # Splunk HEC collector URL
    token: ""              # HEC token
  datadog:
    url: ""                # "" = US1 intake
    api_key: ""            # Datadog API key
  elastic:
    url: ""                # Elasticsearch base URL
    index: ""              # "" = heramba-detections
    api_key: ""            # optional ApiKey auth
  syslog:
    network: udp           # udp | tcp
    address: ""            # host:port of a CEF-over-syslog collector
  loki:
    url: ""                # Grafana Loki push endpoint, e.g. http://loki:3100/loki/api/v1/push
    tenant: ""             # optional X-Scope-OrgID for multi-tenant Loki
  newrelic:
    url: ""                # empty = US Log API (https://log-api.newrelic.com/log/v1)
    api_key: ""            # New Relic license/ingest key (Api-Key header)
  honeycomb:
    url: ""                # empty = US API (https://api.honeycomb.io)
    dataset: ""            # Honeycomb dataset name
    api_key: ""            # X-Honeycomb-Team key
  openobserve:
    url: ""                # instance root, e.g. http://localhost:5080 ("" = disabled)
    org: ""                # empty = default
    stream: ""             # empty = heramba
    username: ""           # basic-auth user (email)
    password: ""           # basic-auth password/token
  sumo: ""                 # Sumo Logic HTTP Source URL (the URL embeds the token)
  # Generic templated webhook: render any payload shape from a Go text/template
  # (executed with the detection event as data) and POST it to any service.
  template_webhook:
    url: ""                # POST target ("" = disabled)
    content_type: ""       # "" = application/json
    template: ""           # e.g. '{"kind":"{{.Kind}}","src":"{{.Source}}","sev":"{{.SeverityS}}"}'

  # --- SMTP email digest (batches events above min_severity into one message) ---
  email:
    host: ""               # SMTP server host ("" = disabled)
    port: 25               # SMTP port
    from: ""               # envelope + header From
    to: []                 # recipient addresses
    username: ""           # optional SMTP auth user ("" = no auth)
    password: ""           # SMTP auth password
    batch_size: 10         # events per digest (0/1 = send each immediately)

  # --- Durable local audit trail (records everything, ignores min_severity) ---
  rotate:
    file: ""               # active JSONL log path ("" = disabled)
    max_bytes: 10485760    # rotate at this size (0 = 10 MiB)
    max_backups: 5         # rotated files to keep (0 = truncate in place)

  # --- Paging (default to a high floor; they wake a human) ---
  pagerduty_key: ""        # PagerDuty Events API v2 routing key
  opsgenie_key: ""         # Opsgenie API key

decoys:
  enabled: true
  preset: ""               # framework preset: node|next|rails|django|laravel|wordpress|fastapi|
                           #   symfony|aspnet|go|phoenix|spring|drupal|magento|joomla|flask|express
                           #   ("" = generic defaults only; run: heramba decoys presets)
  paths: []                # extra decoy paths to serve/watch
  canaries: []             # canary tokens (planted elsewhere) to watch for in requests
  openapi_spec: ""         # path to an OpenAPI/Swagger JSON spec: derive + watch a
                           #   request-body canary per sensitive field (list them with:
                           #   heramba decoys canary --from-openapi spec.json --salt <salt>)
  canary_salt: ""          # salt that makes the derived body-canary values unguessable

# Agentic-traffic classifier thresholds (0/empty = conservative built-in defaults).
classifier:
  cadence_rps: 0           # high_cadence fires above this req/s (default 5)
  enum_distinct: 0         # enumeration fires at/above this many distinct paths (default 12)
  tool_at: 0               # score >= this => "tool" (default 0.4)
  agentic_at: 0            # score >= this => "agentic-suspected" (default 0.7)

# Per-source request rate limit (PROTECT mode only; 0 = disabled).
rate_limit_rps: 0
rate_limit_burst: 0        # defaults to max(rate_limit_rps, 1)

# Graceful-drain window (seconds) on shutdown: readiness flips to not-ready and
# the edge waits this long before stopping, so a load balancer deregisters it
# first. 0 = shut down immediately.
drain_timeout: 0

egress:                    # callback firewall (protect mode)
  enabled: false
  allowed_hosts: []        # hosts the protected app is permitted to call

# Counter-agent prompt decoys (PROTECT mode): inject fabricated vuln leads into
# HTML/JS responses for classifier-flagged agentic sources (invisible to humans).
counter:
  enabled: false
`

const configUsage = "usage: heramba config init [path] | show [path] | example | lint [path]"

func runConfig(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, configUsage)
		os.Exit(2)
	}
	switch args[0] {
	case "init":
		runConfigInit(args[1:])
	case "show":
		runConfigShow(args[1:])
	case "example":
		fmt.Print(exampleConfigYAML)
	case "lint":
		runConfigLint(args[1:])
	default:
		fmt.Fprintln(os.Stderr, configUsage)
		os.Exit(2)
	}
}

// lintLevel ranks a lint finding.
type lintLevel int

const (
	lintInfo lintLevel = iota
	lintWarn
	lintError
)

func (l lintLevel) String() string {
	switch l {
	case lintError:
		return "error"
	case lintWarn:
		return "warn"
	default:
		return "info"
	}
}

// lintFinding is one advisory note about a configuration.
type lintFinding struct {
	Level lintLevel
	Msg   string
}

// lintConfig runs Validate plus advisory checks that Validate does not cover
// (unknown preset, admin/listen port collision, no alert sinks, deny-all egress,
// rate-limit in observe mode). loadErr is the error config.Load returned (if
// any), so a file that fails to validate still produces findings. It is pure, so
// it is directly unit-testable.
func lintConfig(cfg config.Config, loadErr error) []lintFinding {
	var out []lintFinding
	if err := cfg.Validate(); err != nil {
		out = append(out, lintFinding{lintError, err.Error()})
	} else if loadErr != nil {
		out = append(out, lintFinding{lintError, loadErr.Error()})
	}

	if cfg.Decoys.Preset != "" {
		if _, ok := decoy.GetPreset(cfg.Decoys.Preset); !ok {
			out = append(out, lintFinding{lintError,
				fmt.Sprintf("unknown decoys.preset %q (have: %v)", cfg.Decoys.Preset, decoy.PresetNames())})
		}
	}

	if cfg.AdminListen != "" && cfg.AdminListen == cfg.Listen {
		out = append(out, lintFinding{lintError,
			fmt.Sprintf("admin_listen (%s) must differ from listen (%s) — they would collide", cfg.AdminListen, cfg.Listen)})
	}

	if !hasPushSink(cfg.Alert) {
		out = append(out, lintFinding{lintWarn,
			"no webhook/chat/SIEM/email/rotate sink configured — detections only reach the local text/jsonl record"})
	}

	if cfg.Egress.Enabled && len(cfg.Egress.AllowedHosts) == 0 {
		out = append(out, lintFinding{lintWarn,
			"egress.enabled with an empty allowed_hosts denies ALL app callbacks — set allowed_hosts"})
	}

	if cfg.Mode == config.ModeObserve && cfg.RateLimitRPS > 0 {
		out = append(out, lintFinding{lintWarn,
			"rate_limit_rps is set but mode is observe — rate limiting only engages in protect mode"})
	}

	if len(out) == 0 {
		out = append(out, lintFinding{lintInfo, "no issues found"})
	}
	return out
}

// hasPushSink reports whether any non-local alert sink is configured.
func hasPushSink(a config.AlertConfig) bool {
	switch {
	case a.Webhook != "", a.Slack != "", a.Teams != "", a.Discord != "":
		return true
	case a.Splunk.URL != "", a.Datadog.APIKey != "", a.Elastic.URL != "", a.Syslog.Address != "":
		return true
	case a.PagerDutyKey != "", a.OpsgenieKey != "":
		return true
	case a.Rotate.File != "":
		return true
	case a.Email.Host != "" && a.Email.From != "" && len(a.Email.To) > 0:
		return true
	}
	return false
}

// lintHasError reports whether any finding is an error.
func lintHasError(findings []lintFinding) bool {
	for _, f := range findings {
		if f.Level == lintError {
			return true
		}
	}
	return false
}

func runConfigLint(args []string) {
	path := ".heramba.yml"
	if len(args) > 0 {
		path = args[0]
	}
	cfg, err := config.Load(path)
	if os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "heramba: no config at %s\n", path)
		os.Exit(1)
	}
	findings := lintConfig(cfg, err)
	fmt.Printf("linting %s:\n", path)
	for _, f := range findings {
		fmt.Printf("  [%-5s] %s\n", f.Level, f.Msg)
	}
	if lintHasError(findings) {
		os.Exit(1)
	}
}

func runConfigInit(args []string) {
	path := ".heramba.yml"
	if len(args) > 0 {
		path = args[0]
	}
	if _, err := os.Stat(path); err == nil {
		fmt.Fprintf(os.Stderr, "heramba: %s already exists; refusing to overwrite\n", path)
		os.Exit(1)
	}
	if err := os.WriteFile(path, []byte(defaultConfigYAML), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "heramba: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s (observe mode). Review it, then: heramba serve\n", path)
}

func runConfigShow(args []string) {
	path := ".heramba.yml"
	if len(args) > 0 {
		path = args[0]
	}
	cfg, err := config.Load(path)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("no %s found — effective defaults:\n", path)
			cfg = config.Default()
		} else {
			fmt.Fprintf(os.Stderr, "heramba: %v\n", err)
			os.Exit(1)
		}
	}
	fmt.Printf("mode:     %s\nlisten:   %s\nupstream: %s\nalert:    %s (file=%q)\ndecoys:   enabled=%v extra=%d\negress:   enabled=%v allowed=%d\n",
		cfg.Mode, cfg.Listen, orNone(cfg.Upstream), orDefault(cfg.Alert.Format, "text"), cfg.Alert.File,
		cfg.Decoys.Enabled, len(cfg.Decoys.Paths), cfg.Egress.Enabled, len(cfg.Egress.AllowedHosts))
}

func orNone(s string) string {
	if s == "" {
		return "(honeypot-only)"
	}
	return s
}

func orDefault(s, d string) string {
	if s == "" {
		return d
	}
	return s
}
