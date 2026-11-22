package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Armur-Ai/heramba/internal/quickstart"
)

// runQuickstart detects the project's framework and generates a tailored
// .heramba.yml (decoy preset already chosen).
//
//	heramba quickstart                     # detect ./ , write .heramba.yml
//	heramba quickstart --dir ./app --upstream http://app:3000 --print
func runQuickstart(args []string) {
	fs := flag.NewFlagSet("quickstart", flag.ExitOnError)
	dir := fs.String("dir", ".", "project directory to inspect")
	upstream := fs.String("upstream", "", "your app's address (empty = honeypot-only)")
	out := fs.String("out", ".heramba.yml", "config file to write")
	print := fs.Bool("print", false, "print to stdout instead of writing the file")
	force := fs.Bool("force", false, "overwrite an existing config file")
	_ = fs.Parse(args)

	stack := quickstart.Detect(*dir)
	if stack.Name != "" {
		fmt.Fprintf(os.Stderr, "heramba: detected %s (%s)\n", stack.Name, stack.Reason)
	} else {
		fmt.Fprintf(os.Stderr, "heramba: no known framework detected — generating a generic config\n")
	}
	cfg := quickstart.GenerateConfig(stack, *upstream)

	if *print {
		fmt.Print(cfg)
		return
	}
	if _, err := os.Stat(*out); err == nil && !*force {
		fmt.Fprintf(os.Stderr, "heramba: %s already exists (use --force to overwrite, or --print)\n", *out)
		os.Exit(1)
	}
	if err := os.WriteFile(*out, []byte(cfg), 0o644); err != nil { //nolint:gosec
		fmt.Fprintln(os.Stderr, "heramba quickstart:", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "heramba: wrote %s — review it, then `heramba serve`\n", *out)
}
