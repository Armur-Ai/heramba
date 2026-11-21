package main

import (
	"flag"
	"fmt"
	"os"
	"text/tabwriter"

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
	case "canary":
		runCanary(args[1:])
	default:
		decoysUsage()
		os.Exit(2)
	}
}

func decoysUsage() {
	fmt.Fprintln(os.Stderr, `usage:
  heramba decoys list [--preset <stack>]   list the decoy endpoints (optionally for a framework)
  heramba decoys presets                   list available framework presets
  heramba decoys canary <label>            mint a canary token to plant`)
}

func runDecoysPresets() {
	fmt.Println("Framework decoy presets (serve --preset <name> or decoys.preset in config):")
	for _, name := range decoy.PresetNames() {
		p, _ := decoy.GetPreset(name)
		fmt.Printf("  %-10s %d lures, %d zones\n", name, len(p.Paths), len(p.Prefixes))
	}
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
