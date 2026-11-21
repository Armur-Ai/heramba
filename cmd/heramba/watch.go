package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Armur-Ai/heramba/internal/alert"
	"github.com/Armur-Ai/heramba/internal/config"
	"github.com/Armur-Ai/heramba/internal/event"
	"github.com/Armur-Ai/heramba/internal/watch"
)

// runWatch powers `heramba watch` — the live dashboard. It consumes the JSONL
// event stream a running edge emits (its alert sink), so it attaches to any
// deployment without coupling to the engine: tail a live sink file, read a
// pipe, or replay a recorded attack for a demo.
//
//	heramba watch                         # follow the sink from .heramba.yml
//	heramba watch events.jsonl            # follow a specific sink file
//	heramba serve --alert jsonl - | heramba watch -   # live over a pipe
//	heramba watch --replay demo/attack.jsonl --speed 8   # demo playback
func runWatch(args []string) {
	fs := flag.NewFlagSet("watch", flag.ExitOnError)
	follow := fs.Bool("follow", true, "tail the source for new events (tail -f)")
	replay := fs.Bool("replay", false, "replay the source as a recorded attack, paced by timestamp (for demos)")
	speed := fs.Float64("speed", 8, "replay speed multiplier (use with --replay)")
	modeF := fs.String("mode", "", "posture label in the header: observe|protect (default: from config)")
	maxN := fs.Int("max", 200, "events kept in the live feed")
	cfgPath := fs.String("config", ".heramba.yml", "config file (for the default source + mode)")
	plain := fs.Bool("plain", false, "no TUI: print events as text (non-interactive / piping)")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, "usage: heramba watch [flags] [source]\n\n"+
			"source is a JSONL event file, '-' for stdin, or empty to use the\n"+
			"alert sink from config. Starts a live attack dashboard.\n\nflags:\n")
		fs.PrintDefaults()
	}
	_ = fs.Parse(args)

	cfg, _ := config.Load(*cfgPath) // best-effort; fall back to defaults
	cfg = cfg.ApplyEnv()            // honor HERAMBA_* (mode + sink file) like serve
	mode := *modeF
	if mode == "" {
		mode = string(cfg.Mode)
	}
	src := fs.Arg(0)
	if src == "" {
		if cfg.Alert.File != "" {
			src = cfg.Alert.File
		} else {
			src = "-"
		}
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	ch, err := openSource(ctx, src, *follow, *replay, *speed)
	if err != nil {
		fmt.Fprintln(os.Stderr, "heramba watch:", err)
		os.Exit(1)
	}

	if *plain {
		sink := alert.NewText(os.Stdout)
		for e := range ch {
			_ = sink.Emit(e)
		}
		return
	}

	prog := tea.NewProgram(
		watch.NewModel(ch, mode, *maxN),
		tea.WithAltScreen(),
		tea.WithContext(ctx),
	)
	if _, err := prog.Run(); err != nil && ctx.Err() == nil {
		fmt.Fprintln(os.Stderr, "heramba watch:", err)
		os.Exit(1)
	}
}

// openSource turns the resolved source string into an event channel.
func openSource(ctx context.Context, src string, follow, replay bool, speed float64) (<-chan event.Event, error) {
	if src == "-" {
		return watch.Stream(ctx, os.Stdin), nil
	}
	if replay {
		f, err := os.Open(src) //nolint:gosec // operator-supplied capture path
		if err != nil {
			return nil, err
		}
		defer f.Close() //nolint:errcheck
		events, err := watch.ReadAll(f)
		if err != nil {
			return nil, err
		}
		if len(events) == 0 {
			return nil, fmt.Errorf("no events to replay in %s", src)
		}
		return watch.Replay(ctx, events, speed), nil
	}
	if follow {
		return watch.TailFile(ctx, src, 0)
	}
	f, err := os.Open(src) //nolint:gosec // operator-supplied sink path
	if err != nil {
		return nil, err
	}
	defer f.Close() //nolint:errcheck
	events, err := watch.ReadAll(f)
	if err != nil {
		return nil, err
	}
	return watch.Feed(ctx, events), nil
}
