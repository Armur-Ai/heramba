package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Armur-Ai/heramba/internal/replay"
)

// runExperiment is the single documented command that runs the controlled
// four-baseline defense evaluation over identical seeded scenarios and writes a
// reproducible report (markdown by default, JSON with --json).
func runExperiment(args []string) {
	fs := flag.NewFlagSet("experiment", flag.ExitOnError)
	trials := fs.Int("trials", 20, "number of seeded scenario trials per baseline")
	startSeed := fs.Int64("seed", 1, "first seed (seeds run seed..seed+trials-1)")
	asJSON := fs.Bool("json", false, "emit machine-readable JSON instead of markdown")
	out := fs.String("out", "", "write the report to this file instead of stdout")
	_ = fs.Parse(args)

	if *trials < 1 {
		fmt.Fprintln(os.Stderr, "heramba experiment: --trials must be >= 1")
		os.Exit(2)
	}
	seeds := make([]int64, *trials)
	for i := range seeds {
		seeds[i] = *startSeed + int64(i)
	}

	exp := replay.RunFourBaselineExperiment(seeds)

	w := os.Stdout
	if *out != "" {
		f, err := os.Create(*out)
		if err != nil {
			fmt.Fprintf(os.Stderr, "heramba experiment: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		w = f
	}

	var err error
	if *asJSON {
		err = exp.WriteJSON(w)
	} else {
		err = exp.WriteMarkdown(w)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "heramba experiment: %v\n", err)
		os.Exit(1)
	}
}
