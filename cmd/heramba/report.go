package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Armur-Ai/heramba/internal/event"
	"github.com/Armur-Ai/heramba/internal/report"
	"github.com/Armur-Ai/heramba/internal/watch"
)

// runReport renders a shareable incident report from a recorded JSONL capture
// (the alert sink's output), as Markdown (default) or self-contained HTML.
//
//	heramba report events.jsonl                 # Markdown to stdout
//	heramba report --html --out report.html events.jsonl
func runReport(args []string) {
	fs := flag.NewFlagSet("report", flag.ExitOnError)
	asHTML := fs.Bool("html", false, "render self-contained HTML instead of Markdown")
	out := fs.String("out", "", "write to this file instead of stdout")
	title := fs.String("title", "", "report title")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: heramba report [--html] [--out file] [--title t] <events.jsonl|->")
		fs.PrintDefaults()
	}
	_ = fs.Parse(args)

	src := fs.Arg(0)
	if src == "" {
		fs.Usage()
		os.Exit(2)
	}

	evs, err := readEvents(src)
	if err != nil {
		fmt.Fprintln(os.Stderr, "heramba report:", err)
		os.Exit(1)
	}
	st := watch.NewState(len(evs) + 1)
	for _, e := range evs {
		st.Add(e)
	}

	rendered := report.Markdown(st, *title)
	if *asHTML {
		rendered = report.HTML(st, *title)
	}

	if *out == "" {
		fmt.Print(rendered)
		return
	}
	if err := os.WriteFile(*out, []byte(rendered), 0o644); err != nil { //nolint:gosec
		fmt.Fprintln(os.Stderr, "heramba report:", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "heramba: wrote %s\n", *out)
}

// readEvents loads all events from a file or '-' (stdin).
func readEvents(src string) ([]event.Event, error) {
	if src == "-" {
		return watch.ReadAll(os.Stdin)
	}
	f, err := os.Open(src) //nolint:gosec // operator-supplied capture path
	if err != nil {
		return nil, err
	}
	defer f.Close() //nolint:errcheck
	return watch.ReadAll(f)
}
