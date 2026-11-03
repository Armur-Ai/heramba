package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/Armur-Ai/heramba/internal/research/coevolve"
)

// runResearchCoevolve runs the adaptation-race & co-evolution lab (R4).
func runResearchCoevolve(args []string) {
	fs := flag.NewFlagSet("research coevolve", flag.ExitOnError)
	asJSON := fs.Bool("json", false, "emit the summary as JSON instead of markdown")
	out := fs.String("out", "", "write the summary to this file instead of stdout")
	artifacts := fs.String("artifacts", "", "write the artifact bundle (summary + sample event log) to this directory")
	rounds := fs.Int("rounds", 8, "strategy shifts per race")
	trials := fs.Int("seeds", 100, "number of seeded races per archetype/mode")
	_ = fs.Parse(args)

	seeds := make([]int64, *trials)
	for i := range seeds {
		seeds[i] = int64(100 + i)
	}
	rep := coevolve.Run(*rounds, seeds, coevolve.DefaultParams(), codeRevision())

	if *artifacts != "" {
		if err := writeCoevolveArtifacts(*artifacts, rep, *rounds); err != nil {
			fmt.Fprintf(os.Stderr, "heramba research coevolve: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "wrote artifact bundle to %s\n", *artifacts)
	}

	w := os.Stdout
	if *out != "" {
		f, err := os.Create(*out)
		if err != nil {
			fmt.Fprintf(os.Stderr, "heramba research coevolve: %v\n", err)
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
		fmt.Fprintf(os.Stderr, "heramba research coevolve: %v\n", err)
		os.Exit(1)
	}
}

func writeCoevolveArtifacts(dir string, rep coevolve.Report, rounds int) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	writeFile := func(name string, write func(io.Writer) error) error {
		f, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			return err
		}
		if err := write(f); err != nil {
			f.Close()
			return err
		}
		return f.Close()
	}
	if err := writeFile("coevolve-summary.json", rep.WriteJSON); err != nil {
		return err
	}
	if err := writeFile("coevolve-summary.md", rep.WriteMarkdown); err != nil {
		return err
	}
	// A sample replayable event log for the feinting adversary (shows oscillation).
	sim := &coevolve.Sim{Archetype: coevolve.Archetypes()[4], Mode: coevolve.Adaptive, Params: coevolve.DefaultParams()}
	sim.Run(rounds, 100)
	return writeFile("event-log-adversarial.json", func(w io.Writer) error {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(sim.Log)
	})
}
