package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/Armur-Ai/heramba/internal/research/scorecard"
)

// runResearchStats builds the R8 target-vs-observed scorecard from the three
// reproducible experiments and reports it with intervals and effect sizes.
func runResearchStats(args []string) {
	fs := flag.NewFlagSet("research stats", flag.ExitOnError)
	asJSON := fs.Bool("json", false, "emit the scorecard as JSON instead of markdown")
	out := fs.String("out", "", "write the scorecard to this file instead of stdout")
	artifacts := fs.String("artifacts", "", "write the scorecard bundle to this directory")
	_ = fs.Parse(args)

	sc := scorecard.Build(codeRevision())

	if *artifacts != "" {
		if err := os.MkdirAll(*artifacts, 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "heramba research stats: %v\n", err)
			os.Exit(1)
		}
		write := func(name string, f func(io.Writer) error) {
			fh, err := os.Create(filepath.Join(*artifacts, name))
			if err != nil {
				fmt.Fprintf(os.Stderr, "heramba research stats: %v\n", err)
				os.Exit(1)
			}
			defer fh.Close()
			if err := f(fh); err != nil {
				fmt.Fprintf(os.Stderr, "heramba research stats: %v\n", err)
				os.Exit(1)
			}
		}
		write("scorecard.json", sc.WriteJSON)
		write("scorecard.md", sc.WriteMarkdown)
		fmt.Fprintf(os.Stderr, "wrote scorecard bundle to %s\n", *artifacts)
	}

	w := os.Stdout
	if *out != "" {
		f, err := os.Create(*out)
		if err != nil {
			fmt.Fprintf(os.Stderr, "heramba research stats: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		w = f
	}
	var err error
	if *asJSON {
		err = sc.WriteJSON(w)
	} else {
		err = sc.WriteMarkdown(w)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "heramba research stats: %v\n", err)
		os.Exit(1)
	}
}
