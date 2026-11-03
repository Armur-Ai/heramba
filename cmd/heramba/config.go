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
`

func runConfig(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: heramba config init [path] | heramba config show [path]")
		os.Exit(2)
	}
	switch args[0] {
	case "init":
		runConfigInit(args[1:])
	case "show":
		runConfigShow(args[1:])
	default:
		fmt.Fprintln(os.Stderr, "usage: heramba config init [path] | heramba config show [path]")
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
