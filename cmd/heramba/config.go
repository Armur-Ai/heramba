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

alert:
  format: text           # text | jsonl
  file: ""               # path, or empty for stdout

decoys:
  enabled: true
  paths: []              # extra decoy paths to serve/watch

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
