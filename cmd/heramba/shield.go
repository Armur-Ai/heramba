package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/Armur-Ai/heramba/internal/shield"
)

// runShield orchestrates Heramba's defensive tool suite (agat, byakugan,
// sharingan, skillscan) over a target and reports an aggregated assessment.
// Each tool is also usable standalone; this is the composed view.
func runShield(args []string) {
	fs := flag.NewFlagSet("shield", flag.ExitOnError)
	target := fs.String("target", "", "path to assess (directory, artifact, model file, or repo)")
	jsonOut := fs.Bool("json", false, "emit the results as JSON")
	_ = fs.Parse(args)

	if *target == "" {
		fmt.Fprintln(os.Stderr, "heramba shield: --target <path> is required")
		fmt.Fprintln(os.Stderr, "example: heramba shield --target ./build/out")
		os.Exit(2)
	}
	if _, err := os.Stat(*target); err != nil {
		fmt.Fprintf(os.Stderr, "heramba shield: cannot access target: %v\n", err)
		os.Exit(1)
	}

	results := shield.Default().Run(context.Background(), shield.Target{Path: *target})

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(results)
		return
	}

	fmt.Printf("Heramba defensive assessment — %s\n\n", *target)
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "TOOL\tSTATUS\tFINDINGS\tDETAIL")
	ran, findings := 0, 0
	for _, r := range results {
		if r.Status == shield.StatusClean || r.Status == shield.StatusFindings {
			ran++
			findings += r.Findings
		}
		fmt.Fprintf(tw, "%s\t%s\t%d\t%s\n", r.Tool, r.Status, r.Findings, r.Detail)
	}
	_ = tw.Flush()
	fmt.Printf("\n%d/%d tools ran · %d finding(s). Tools not installed are an explicit gap, not a clean result.\n",
		ran, len(results), findings)
}
