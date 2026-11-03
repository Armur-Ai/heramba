package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Armur-Ai/heramba/internal/research"
	"github.com/Armur-Ai/heramba/internal/research/coord"
)

// runResearchCoord runs the fair coordination-detection experiment (R2) over
// the pre-registered held-out families and emits a complete artifact bundle:
// the protocol manifest, the raw per-cohort scores, and a JSON + markdown
// summary with uncertainty. One command, repeatable.
func runResearchCoord(args []string) {
	fs := flag.NewFlagSet("research coord", flag.ExitOnError)
	asJSON := fs.Bool("json", false, "emit the summary as JSON instead of markdown")
	out := fs.String("out", "", "write the summary to this file instead of stdout")
	artifacts := fs.String("artifacts", "", "write the full artifact bundle (manifest + summary) to this directory")
	all := fs.Bool("all-families", false, "evaluate every family (default: held-out test families only)")
	_ = fs.Parse(args)

	p := research.DefaultProtocol()
	if err := p.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "heramba research coord: invalid protocol: %v\n", err)
		os.Exit(1)
	}

	opt := coord.Options{CodeRevision: codeRevision()}
	if *all {
		opt.Families = p.Families
	}
	rep := coord.Run(p, opt)

	if *artifacts != "" {
		if err := writeCoordArtifacts(*artifacts, p, rep); err != nil {
			fmt.Fprintf(os.Stderr, "heramba research coord: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "wrote artifact bundle to %s\n", *artifacts)
	}

	w := os.Stdout
	if *out != "" {
		f, err := os.Create(*out)
		if err != nil {
			fmt.Fprintf(os.Stderr, "heramba research coord: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		w = f
	}
	var err error
	if *asJSON {
		err = rep.WriteJSON(w)
	} else {
		err = rep.WriteMarkdown(w)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "heramba research coord: %v\n", err)
		os.Exit(1)
	}
}

func writeCoordArtifacts(dir string, p research.Protocol, rep coord.Report) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	type named struct {
		name  string
		write func(f *os.File) error
	}
	files := []named{
		{"protocol.manifest.json", func(f *os.File) error { return p.WriteManifest(f) }},
		{"coord-summary.json", func(f *os.File) error { return rep.WriteJSON(f) }},
		{"coord-summary.md", func(f *os.File) error { return rep.WriteMarkdown(f) }},
	}
	for _, nf := range files {
		f, err := os.Create(filepath.Join(dir, nf.name))
		if err != nil {
			return err
		}
		if err := nf.write(f); err != nil {
			f.Close()
			return err
		}
		if err := f.Close(); err != nil {
			return err
		}
	}
	return nil
}

// codeRevision returns the short git revision for provenance, or "unknown".
func codeRevision() string {
	out, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}
