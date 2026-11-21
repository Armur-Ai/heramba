// Package watch powers `heramba watch` — the live "you're being attacked right
// now" dashboard. It consumes the same JSONL event stream the alert sinks emit
// (see internal/alert), so the dashboard is fully decoupled from the running
// edge: point it at a live sink file, a pipe, or a recorded attack for a demo.
//
// This file is the ingest side: turn a byte stream of newline-delimited event
// JSON into a channel of event.Event, optionally tailing a growing file.
package watch

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"time"

	"github.com/Armur-Ai/heramba/internal/event"
)

// maxLine bounds a single JSONL record (1 MiB) so a malformed stream can't
// grow the scanner buffer without limit.
const maxLine = 1 << 20

// decodeLine parses one JSONL record. Blank lines yield (zero, false, nil) so
// callers can skip them; a non-empty line that will not parse returns an error.
func decodeLine(line []byte) (event.Event, bool, error) {
	if len(strings.TrimSpace(string(line))) == 0 {
		return event.Event{}, false, nil
	}
	var e event.Event
	if err := json.Unmarshal(line, &e); err != nil {
		return event.Event{}, false, err
	}
	return e, true, nil
}

// ReadAll decodes every event from r to EOF. Malformed lines are skipped (a
// recorded capture may be truncated mid-write); the first hard read error that
// is not EOF is returned alongside whatever was decoded so far.
func ReadAll(r io.Reader) ([]event.Event, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), maxLine)
	var out []event.Event
	for sc.Scan() {
		if e, ok, err := decodeLine(sc.Bytes()); err == nil && ok {
			out = append(out, e)
		}
	}
	if err := sc.Err(); err != nil && !errors.Is(err, io.EOF) {
		return out, err
	}
	return out, nil
}

// Stream reads r line by line and delivers each decoded event on the returned
// channel until EOF or ctx cancellation. It is the right source for a pipe
// (`heramba serve --alert-jsonl - | heramba watch -`) or a one-shot file read.
// The channel is closed when the stream ends. Malformed lines are skipped.
func Stream(ctx context.Context, r io.Reader) <-chan event.Event {
	out := make(chan event.Event, 128)
	go func() {
		defer close(out)
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 0, 64*1024), maxLine)
		for sc.Scan() {
			e, ok, err := decodeLine(sc.Bytes())
			if err != nil || !ok {
				continue
			}
			// Prioritize cancellation over delivery so no event is emitted
			// after ctx is done, even when a receiver is also ready.
			select {
			case <-ctx.Done():
				return
			default:
			}
			select {
			case out <- e:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}

// TailFile streams events from path and keeps watching for lines appended after
// EOF (like `tail -f`), which is how the dashboard follows a live `heramba
// serve` JSONL sink. It polls every interval; cancel ctx to stop. A file that
// does not exist yet is waited for rather than treated as an error, so you can
// start the dashboard before the first attack lands.
func TailFile(ctx context.Context, path string, interval time.Duration) (<-chan event.Event, error) {
	if interval <= 0 {
		interval = 250 * time.Millisecond
	}
	out := make(chan event.Event, 128)
	go func() {
		defer close(out)
		var f *os.File
		var reader *bufio.Reader
		open := func() bool {
			fh, err := os.Open(path) //nolint:gosec // operator-supplied sink path
			if err != nil {
				return false
			}
			f = fh
			reader = bufio.NewReaderSize(f, 64*1024)
			return true
		}
		defer func() {
			if f != nil {
				_ = f.Close()
			}
		}()
		var pending []byte
		for {
			if f == nil && !open() {
				if !sleep(ctx, interval) {
					return
				}
				continue
			}
			line, err := reader.ReadBytes('\n')
			if len(line) > 0 {
				pending = append(pending, line...)
				if line[len(line)-1] == '\n' {
					if e, ok, derr := decodeLine(pending); derr == nil && ok {
						select {
						case out <- e:
						case <-ctx.Done():
							return
						}
					}
					pending = pending[:0]
				}
			}
			if errors.Is(err, io.EOF) {
				if !sleep(ctx, interval) {
					return
				}
			}
		}
	}()
	return out, nil
}

// sleep waits d or until ctx is done; it reports false if ctx ended.
func sleep(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		return false
	}
}
