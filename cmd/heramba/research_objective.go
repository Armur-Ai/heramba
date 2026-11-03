package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/Armur-Ai/heramba/internal/research/objective"
)

// runResearchObjective runs the deception & attack-objective experiment (R3):
// five defensive arms measured against a configurable (optionally
// deception-aware) adversary, with a full artifact bundle.
func runResearchObjective(args []string) {
	fs := flag.NewFlagSet("research objective", flag.ExitOnError)
	asJSON := fs.Bool("json", false, "emit the summary as JSON instead of markdown")
	out := fs.String("out", "", "write the summary to this file instead of stdout")
	artifacts := fs.String("artifacts", "", "write the artifact bundle to this directory (both attacker models)")
	aware := fs.Bool("deception-aware", false, "model an adversary that rejects inconsistent lures")
	trials := fs.Int("trials", 500, "number of seeded trials per arm")
	_ = fs.Parse(args)

	seeds := make([]int64, *trials)
	for i := range seeds {
		seeds[i] = int64(1000 + i)
	}
	cfg := objective.DefaultConfig(seeds)
	cfg.CodeRevision = codeRevision()
	cfg.Attacker.DeceptionAware = *aware

	if *artifacts != "" {
		if err := writeObjectiveArtifacts(*artifacts, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "heramba research objective: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "wrote artifact bundle to %s\n", *artifacts)
	}

	rep := objective.Run(cfg)
	w := os.Stdout
	if *out != "" {
		f, err := os.Create(*out)
		if err != nil {
			fmt.Fprintf(os.Stderr, "heramba research objective: %v\n", err)
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
		fmt.Fprintf(os.Stderr, "heramba research objective: %v\n", err)
		os.Exit(1)
	}
}

// writeObjectiveArtifacts emits both attacker models (naive and deception-aware)
// so the reference bundle shows the full H2 story.
func writeObjectiveArtifacts(dir string, cfg objective.Config) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	variants := []struct {
		name  string
		aware bool
	}{
		{"objective-naive", false},
		{"objective-deception-aware", true},
	}
	for _, v := range variants {
		c := cfg
		c.Attacker.DeceptionAware = v.aware
		rep := objective.Run(c)
		for ext, write := range map[string]func(io.Writer) error{
			".json": rep.WriteJSON,
			".md":   rep.WriteMarkdown,
		} {
			f, err := os.Create(filepath.Join(dir, v.name+ext))
			if err != nil {
				return err
			}
			if err := write(f); err != nil {
				f.Close()
				return err
			}
			if err := f.Close(); err != nil {
				return err
			}
		}
	}
	return nil
}
