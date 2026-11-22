package main

import (
	"flag"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/Armur-Ai/heramba/internal/config"
	"github.com/Armur-Ai/heramba/internal/decoy"
)

// runDecoys lists the active decoy surface and mints canary tokens.
func runDecoys(args []string) {
	if len(args) == 0 {
		decoysUsage()
		os.Exit(2)
	}
	switch args[0] {
	case "list":
		runDecoysList(args[1:])
	case "presets":
		runDecoysPresets()
	case "export":
		runDecoysExport(args[1:])
	case "canary":
		runCanary(args[1:])
	case "seeds":
		runDecoysSeeds(args[1:])
	case "verify":
		runDecoysVerify(args[1:])
	default:
		decoysUsage()
		os.Exit(2)
	}
}

// verifyDecoy classifies a string against a registry: exact decoy path, decoy
// zone, watched canary, canary-shaped-but-unwatched, or unknown. It returns a
// human description and whether the string matched a decoy/canary. Pure, so it
// is directly unit-testable.
func verifyDecoy(r *decoy.Registry, s string) (string, bool) {
	if c, ok := r.Canaries().Check(s); ok {
		return "watched canary (" + c.Label + ")", true
	}
	if e, ok := r.IsDecoy(s); ok {
		return "decoy path (" + e.Note + ")", true
	}
	if decoy.Looks(s) {
		return "canary-shaped but NOT currently watched (minted elsewhere, or not configured)", false
	}
	return "not a known decoy path or watched canary", false
}

func decoysUsage() {
	fmt.Fprintln(os.Stderr, `usage:
  heramba decoys list [--preset <stack>]   list the decoy endpoints (optionally for a framework)
  heramba decoys presets                   list available framework presets
  heramba decoys export [--preset s] [--format json|nginx|caddy|traefik|haproxy|envoy|k8s-ingress]   export the decoy surface
  heramba decoys canary <label>            mint a canary token to plant
  heramba decoys seeds [--preset <stack>]  list believable canary seeds to plant per framework
  heramba decoys verify <value> [--preset s] [--config f]   is this string a decoy path / watched canary?`)
}

// runDecoysVerify is a triage helper: given a path or token, report whether the
// edge would treat it as a decoy/canary, using the same registry the serve
// command builds (defaults + config decoys/canaries + optional preset).
func runDecoysVerify(args []string) {
	fs := flag.NewFlagSet("verify", flag.ExitOnError)
	preset := fs.String("preset", "", "also apply a framework preset")
	cfgPath := fs.String("config", ".heramba.yml", "config file for extra decoys/canaries")
	_ = fs.Parse(args)
	if fs.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: heramba decoys verify <value> [--preset s] [--config f]")
		os.Exit(2)
	}
	value := fs.Arg(0)

	cfg, _ := config.Load(*cfgPath) // best-effort; defaults are fine
	cfg = cfg.ApplyEnv()
	r := decoy.NewRegistry(cfg.Decoys.Paths...)
	presetName := *preset
	if presetName == "" {
		presetName = cfg.Decoys.Preset
	}
	if presetName != "" {
		p, ok := decoy.GetPreset(presetName)
		if !ok {
			fmt.Fprintf(os.Stderr, "heramba: unknown preset %q (have: %v)\n", presetName, decoy.PresetNames())
			os.Exit(1)
		}
		r.ApplyPreset(p)
	}
	for _, tok := range cfg.Decoys.Canaries {
		r.Canaries().Watch(tok, "configured canary")
	}

	desc, matched := verifyDecoy(r, value)
	fmt.Printf("%q: %s\n", value, desc)
	if !matched {
		os.Exit(1)
	}
}

func runDecoysPresets() {
	fmt.Println("Framework decoy presets (serve --preset <name> or decoys.preset in config):")
	for _, name := range decoy.PresetNames() {
		p, _ := decoy.GetPreset(name)
		fmt.Printf("  %-10s %d lures, %d zones, %d canary seeds\n", name, len(p.Paths), len(p.Prefixes), len(p.Seeds))
	}
}

// runDecoysSeeds lists the believable canary seeds a preset ships — fake secrets
// to plant where only an attacker would find them; the edge watches each value,
// so any later use is a zero-false-positive confirmed intrusion.
func runDecoysSeeds(args []string) {
	fs := flag.NewFlagSet("seeds", flag.ExitOnError)
	preset := fs.String("preset", "", "show seeds for one framework preset (default: all)")
	_ = fs.Parse(args)

	names := decoy.PresetNames()
	if *preset != "" {
		if _, ok := decoy.GetPreset(*preset); !ok {
			fmt.Fprintf(os.Stderr, "heramba: unknown preset %q (have: %v)\n", *preset, decoy.PresetNames())
			os.Exit(1)
		}
		names = []string{*preset}
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "PRESET\tPLANT AT\tSEED VALUE (fake — plant & watch)")
	for _, name := range names {
		p, _ := decoy.GetPreset(name)
		for _, seed := range p.Seeds {
			fmt.Fprintf(tw, "%s\t%s\t%s\n", name, seed.Plant, seed.Value)
		}
	}
	_ = tw.Flush()
	fmt.Println("\nThese values are inert examples. Plant one inside the matching decoy file; applying")
	fmt.Println("the preset (serve --preset / decoys.preset) makes the edge watch it. Any later use is")
	fmt.Println("a confirmed intrusion — no legitimate client ever carries a planted seed.")
}

func runDecoysList(args []string) {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	preset := fs.String("preset", "", "also show the lures a framework preset adds")
	_ = fs.Parse(args)

	r := decoy.NewRegistry()
	if *preset != "" {
		p, ok := decoy.GetPreset(*preset)
		if !ok {
			fmt.Fprintf(os.Stderr, "heramba: unknown preset %q (have: %v)\n", *preset, decoy.PresetNames())
			os.Exit(1)
		}
		r.ApplyPreset(p)
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "PATH\tWHY IT'S A LURE")
	for _, e := range r.Endpoints() {
		fmt.Fprintf(tw, "%s\t%s\n", e.Path, e.Note)
	}
	// Decoy zones: any path under the prefix is a hit (shown with a trailing *).
	for _, e := range r.PrefixZones() {
		fmt.Fprintf(tw, "%s*\t%s\n", e.Path, e.Note)
	}
	_ = tw.Flush()
	fmt.Println("\nA request to any of these is zero-false-positive: no legitimate client touches them.")
	fmt.Println("A trailing * marks a decoy zone: every path under that prefix is a lure.")
}

func runCanary(args []string) {
	fs := flag.NewFlagSet("canary", flag.ExitOnError)
	_ = fs.Parse(args)
	label := "manual"
	if fs.NArg() > 0 {
		label = fs.Arg(0)
	}
	c, err := decoy.NewCanaryStore().Mint(label)
	if err != nil {
		fmt.Fprintf(os.Stderr, "heramba: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("canary token (label %q):\n  %s\n\n", c.Label, c.Token)
	fmt.Println("Plant it where only an attacker would find and use it (a fake secret, a")
	fmt.Println("decoy API key, a hidden field). Any later use is a confirmed intrusion.")
}

// runDecoysExport prints the decoy surface as JSON (default) or an nginx snippet.
func runDecoysExport(args []string) {
	fs := flag.NewFlagSet("export", flag.ExitOnError)
	preset := fs.String("preset", "", "include a framework preset's lures")
	format := fs.String("format", "json", "output format: json|nginx|caddy|traefik|haproxy|envoy|k8s-ingress")
	_ = fs.Parse(args)

	r := decoy.NewRegistry()
	if *preset != "" {
		p, ok := decoy.GetPreset(*preset)
		if !ok {
			fmt.Fprintf(os.Stderr, "heramba: unknown preset %q (have: %v)\n", *preset, decoy.PresetNames())
			os.Exit(1)
		}
		r.ApplyPreset(p)
	}
	surface := r.Export()
	switch *format {
	case "nginx":
		fmt.Print(surface.Nginx())
	case "caddy":
		fmt.Print(surface.Caddy())
	case "traefik":
		fmt.Print(surface.Traefik())
	case "haproxy":
		fmt.Print(surface.HAProxy())
	case "envoy":
		fmt.Print(surface.Envoy())
	case "k8s-ingress":
		fmt.Print(surface.K8sIngress())
	default:
		b, err := surface.JSON()
		if err != nil {
			fmt.Fprintln(os.Stderr, "heramba:", err)
			os.Exit(1)
		}
		fmt.Println(string(b))
	}
}
