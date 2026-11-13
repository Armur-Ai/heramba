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
	failOn := fs.String("fail-on", "none", "exit non-zero on: none|findings|error|gap (CI gating)")
	_ = fs.Parse(args)

	policy, ok := shield.ParseFailPolicy(*failOn)
	if !ok {
		fmt.Fprintf(os.Stderr, "heramba shield: unknown --fail-on %q (want none|findings|error|gap)\n", *failOn)
		os.Exit(2)
	}
	if *target == "" {
		fmt.Fprintln(os.Stderr, "heramba shield: --target <path> is required")
		fmt.Fprintln(os.Stderr, "example: heramba shield --target ./build/out --fail-on findings")
		os.Exit(2)
	}
	if _, err := os.Stat(*target); err != nil {
		fmt.Fprintf(os.Stderr, "heramba shield: cannot access target: %v\n", err)
		os.Exit(1)
	}

	results := shield.Default().Run(context.Background(), shield.Target{Path: *target})
	assessment := shield.Assess(results)

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(struct {
			Target     string            `json:"target"`
			Assessment shield.Assessment `json:"assessment"`
			Results    []shield.Result   `json:"results"`
		}{*target, assessment, results})
	} else {
		fmt.Printf("Heramba defensive assessment — %s\n\n", *target)
		tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "TOOL\tSTATUS\tFINDINGS\tDETAIL")
		for _, r := range results {
			fmt.Fprintf(tw, "%s\t%s\t%d\t%s\n", r.Tool, r.Status, r.Findings, r.Detail)
		}
		_ = tw.Flush()
		fmt.Printf("\nOverall: %s · %d/%d tools ran · %d finding(s). Tools not installed are an explicit gap, not a clean result.\n",
			assessment.Overall, assessment.Coverage.Ran, assessment.Coverage.Total, assessment.TotalFindings)
	}

	// CI gating: a strict policy turns findings / errors / missing coverage into
	// a non-zero exit. The default (none) is report-only.
	if assessment.ShouldFail(policy) {
		if !*jsonOut {
			fmt.Fprintf(os.Stderr, "\nshield: failing under --fail-on=%s (overall=%s)\n", policy, assessment.Overall)
		}
		os.Exit(1)
	}
}
