package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/Armur-Ai/heramba/internal/version"
)

// runVersion prints the build banner, or machine-readable JSON with --json.
func runVersion(args []string) {
	fs := flag.NewFlagSet("version", flag.ExitOnError)
	asJSON := fs.Bool("json", false, "emit build info (version/commit/date/go) as JSON")
	_ = fs.Parse(args)
	if *asJSON {
		b, err := json.MarshalIndent(version.Info(), "", "  ")
		if err != nil {
			fmt.Fprintln(os.Stderr, "heramba:", err)
			os.Exit(1)
		}
		fmt.Println(string(b))
		return
	}
	fmt.Println(version.String())
}
