package main

import (
	"fmt"
	"os"

	"github.com/Armur-Ai/heramba/internal/config"
)

// runDoctor reports readiness: config presence/validity and posture, as an
// advisory panel (it never blocks — it just tells you what you're about to run).
func runDoctor(args []string) {
	path := ".heramba.yml"
	if len(args) > 0 {
		path = args[0]
	}

	fmt.Println("heramba doctor")
	cfg := config.Default()
	switch c, err := config.Load(path); {
	case err == nil:
		cfg = c
		check(true, "config", fmt.Sprintf("%s loaded and valid", path))
	case os.IsNotExist(err):
		check(false, "config", fmt.Sprintf("%s not found — using defaults (run: heramba config init)", path))
	default:
		check(false, "config", err.Error())
	}

	switch cfg.Mode {
	case config.ModeObserve:
		check(true, "mode", "observe (safe: detection only, nothing inline)")
	case config.ModeProtect:
		check(true, "mode", "protect (inline: normalization + tarpit)")
	default:
		check(false, "mode", fmt.Sprintf("invalid mode %q", cfg.Mode))
	}

	if cfg.Upstream == "" {
		check(true, "upstream", "none — honeypot-only (decoys + 404)")
	} else {
		check(true, "upstream", cfg.Upstream)
	}

	if cfg.Mode == config.ModeProtect && cfg.Egress.Enabled && len(cfg.Egress.AllowedHosts) == 0 {
		check(false, "egress", "firewall enabled but allowlist is empty — all egress will be blocked")
	} else if cfg.Egress.Enabled {
		check(true, "egress", fmt.Sprintf("firewall on, %d host(s) allowed", len(cfg.Egress.AllowedHosts)))
	} else {
		check(true, "egress", "firewall off")
	}
}

func check(ok bool, label, detail string) {
	mark := "✓"
	if !ok {
		mark = "✗"
	}
	fmt.Printf("  %s %-9s %s\n", mark, label, detail)
}
