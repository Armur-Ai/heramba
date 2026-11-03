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
		runDecoysList()
	case "canary":
		runCanary(args[1:])
	default:
		decoysUsage()
		os.Exit(2)
	}
}

func decoysUsage() {
	fmt.Fprintln(os.Stderr, `usage:
  heramba decoys list              list the default decoy endpoints
  heramba decoys canary <label>    mint a canary token to plant`)
}

func runDecoysList() {
	r := decoy.NewRegistry()
	tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "PATH\tWHY IT'S A LURE")
	for _, e := range r.Endpoints() {
		fmt.Fprintf(tw, "%s\t%s\n", e.Path, e.Note)
	}
	_ = tw.Flush()
	fmt.Println("\nA request to any of these is zero-false-positive: no legitimate client touches them.")
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
