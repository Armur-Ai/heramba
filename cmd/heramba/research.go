package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Armur-Ai/heramba/internal/research"
)

// runResearch is the entry point for Heramba's research plane: the reproducible
// Phase 2 experiments and their manifests. Every subcommand is deterministic and
// emits complete artifacts (config, seeds, revision, raw, summary).
func runResearch(args []string) {
	if len(args) < 1 {
		researchUsage(os.Stderr)
		os.Exit(2)
	}
	switch args[0] {
	case "manifest":
		runResearchManifest(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "heramba research: unknown subcommand %q\n\n", args[0])
		researchUsage(os.Stderr)
		os.Exit(2)
	}
}

func researchUsage(w *os.File) {
	fmt.Fprint(w, `heramba research — reproducible Phase 2 experiments

usage:
  heramba research <subcommand> [flags]

subcommands:
  manifest   print the pre-registered protocol + hypotheses (JSON), validated
`)
}

// runResearchManifest validates and prints the pre-registered protocol. It is
// the R1 "hypothesis & scenario manifest": the single source of truth for
// H1-H5, the threat-model actors, endpoints, and leakage-free partitions.
func runResearchManifest(args []string) {
	fs := flag.NewFlagSet("research manifest", flag.ExitOnError)
	out := fs.String("out", "", "write the manifest to this file instead of stdout")
	_ = fs.Parse(args)

	p := research.DefaultProtocol()
	if err := p.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "heramba research manifest: protocol is invalid: %v\n", err)
		os.Exit(1)
	}

	w := os.Stdout
	if *out != "" {
		f, err := os.Create(*out)
		if err != nil {
			fmt.Fprintf(os.Stderr, "heramba research manifest: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		w = f
	}
	if err := p.WriteManifest(w); err != nil {
		fmt.Fprintf(os.Stderr, "heramba research manifest: %v\n", err)
		os.Exit(1)
	}
}
