package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/Armur-Ai/heramba/internal/alert"
	"github.com/Armur-Ai/heramba/internal/config"
	"github.com/Armur-Ai/heramba/internal/event"
	"github.com/Armur-Ai/heramba/internal/watch"
)

// runReplay powers `heramba replay` — feed a recorded JSONL capture back through
// the alert sinks wired in config. Unlike `watch` (a live TUI), replay is for
// exercising the delivery path: point it at a capture to verify your SIEM /
// Slack / Teams / Discord / webhook wiring actually receives events, without
// waiting for a real attack.
//
//	heramba replay events.jsonl                 # blast every event through the sinks
//	heramba replay --speed 8 events.jsonl       # pace by timestamp, 8x real time
//	heramba serve --alert jsonl - | heramba replay -   # (stdin)
func runReplay(args []string) {
	fs := flag.NewFlagSet("replay", flag.ExitOnError)
	cfgPath := fs.String("config", ".heramba.yml", "config file (for the alert sink wiring)")
	speed := fs.Float64("speed", 0, "replay speed multiplier; 0 = no pacing (as fast as possible)")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, "usage: heramba replay [flags] <capture.jsonl|->\n\n"+
			"Feed a recorded JSONL capture through the alert sinks from config, to\n"+
			"test SIEM / webhook / chat wiring against a capture.\n\nflags:\n")
		fs.PrintDefaults()
	}
	_ = fs.Parse(args)

	src := fs.Arg(0)
	if src == "" {
		fs.Usage()
		os.Exit(2)
	}

	events, err := readCapture(src)
	if err != nil {
		fmt.Fprintln(os.Stderr, "heramba replay:", err)
		os.Exit(1)
	}
	if len(events) == 0 {
		fmt.Fprintf(os.Stderr, "heramba replay: no events in %s\n", src)
		os.Exit(1)
	}

	cfg, _ := config.Load(*cfgPath) // best-effort; fall back to defaults
	cfg = cfg.ApplyEnv()            // honor HERAMBA_* sink wiring like serve
	counting := alert.Count(buildSink(cfg))

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	fmt.Printf("heramba replay — %d event(s) from %s through configured sinks (speed=%s)\n",
		len(events), src, speedLabel(*speed))
	n := replayThrough(ctx, counting, events, *speed)
	fmt.Printf("heramba replay: emitted %d event(s)\n", n)
	for _, t := range counting.Snapshot() {
		fmt.Printf("  %-18s %d\n", t.Kind, t.Count)
	}
}

// readCapture loads a JSONL capture from a file path or stdin ("-").
func readCapture(src string) ([]event.Event, error) {
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

// replayThrough feeds every event through sink, returning how many were emitted.
// speed<=0 blasts through with no pacing; speed>0 paces by timestamp via
// watch.Replay (ctrl-C stops it). Exported logic kept here so it stays testable
// with a counting sink and no network.
func replayThrough(ctx context.Context, sink alert.Sink, events []event.Event, speed float64) int {
	if speed <= 0 {
		n := 0
		for _, e := range events {
			select {
			case <-ctx.Done():
				return n
			default:
			}
			_ = sink.Emit(e)
			n++
		}
		return n
	}
	n := 0
	for e := range watch.Replay(ctx, events, speed) {
		_ = sink.Emit(e)
		n++
	}
	return n
}

func speedLabel(speed float64) string {
	if speed <= 0 {
		return "unpaced"
	}
	return fmt.Sprintf("%.3gx", speed)
}
