package main

import (
	"fmt"
	"os"

	"github.com/Armur-Ai/heramba/internal/config"
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
  preset: ""               # framework preset: node|next|rails|django|laravel|wordpress|
                           #   fastapi|symfony|aspnet|go|phoenix|spring  ("" = generic defaults only)
  paths: []                # extra decoy paths to serve/watch
  canaries: []             # canary tokens (planted elsewhere) to watch for in requests

# Agentic-traffic classifier thresholds (0/empty = conservative built-in defaults).
classifier:
  cadence_rps: 0           # high_cadence fires above this req/s (default 5)
  enum_distinct: 0         # enumeration fires at/above this many distinct paths (default 12)
  tool_at: 0               # score >= this => "tool" (default 0.4)
  agentic_at: 0            # score >= this => "agentic-suspected" (default 0.7)

# Per-source request rate limit (PROTECT mode only; 0 = disabled).
rate_limit_rps: 0
rate_limit_burst: 0        # defaults to max(rate_limit_rps, 1)

egress:                    # callback firewall (protect mode)
  enabled: false
  allowed_hosts: []        # hosts the protected app is permitted to call

# Counter-agent prompt decoys (PROTECT mode): inject fabricated vuln leads into
# HTML/JS responses for classifier-flagged agentic sources (invisible to humans).
counter:
  enabled: false
`

func runConfig(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: heramba config init [path] | heramba config show [path] | heramba config example")
		os.Exit(2)
	}
	switch args[0] {
	case "init":
		runConfigInit(args[1:])
	case "show":
		runConfigShow(args[1:])
	case "example":
		fmt.Print(exampleConfigYAML)
	default:
		fmt.Fprintln(os.Stderr, "usage: heramba config init [path] | heramba config show [path] | heramba config example")
		os.Exit(2)
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
