package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Armur-Ai/heramba/internal/research"
	"github.com/Armur-Ai/heramba/internal/research/coord"
)

// runResearchH4 runs the H4 missing-sensor degradation sweep.
func runResearchH4(args []string) {
	fs := flag.NewFlagSet("research h4", flag.ExitOnError)
	asJSON := fs.Bool("json", false, "emit JSON instead of markdown")
	out := fs.String("out", "", "write to this file instead of stdout")
	artifacts := fs.String("artifacts", "", "write the sweep bundle to this directory")
	_ = fs.Parse(args)

	rep := coord.RunH4(research.DefaultProtocol(), coord.DefaultParams(), codeRevision())

	if *artifacts != "" {
		if err := os.MkdirAll(*artifacts, 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "heramba research h4: %v\n", err)
			os.Exit(1)
		}
		jf, _ := os.Create(filepath.Join(*artifacts, "h4-sensors.json"))
		_ = rep.WriteJSON(jf)
		jf.Close()
		mf, _ := os.Create(filepath.Join(*artifacts, "h4-sensors.md"))
		_ = rep.WriteMarkdown(mf)
		mf.Close()
		fmt.Fprintf(os.Stderr, "wrote H4 sweep to %s\n", *artifacts)
	}

	w := os.Stdout
	if *out != "" {
		f, err := os.Create(*out)
		if err != nil {
			fmt.Fprintf(os.Stderr, "heramba research h4: %v\n", err)
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
		fmt.Fprintf(os.Stderr, "heramba research h4: %v\n", err)
		os.Exit(1)
	}
}
