// Package alert delivers detection events to outputs (sinks). The OSS engine
// ships deterministic local sinks: human-readable text and JSONL. Richer sinks
// (Slack/PagerDuty/SIEM) are commercial plugins; the engine depends only on the
// Sink interface so those attach without touching the core.
package alert

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/Armur-Ai/heramba/internal/event"
)

// Sink receives detection events. Implementations must be safe for concurrent
// use (the edge runtime emits from many goroutines).
type Sink interface {
	Emit(event.Event) error
}

// WriterSink writes events to an io.Writer, either as JSONL or human text.
type WriterSink struct {
	mu   sync.Mutex
	w    io.Writer
	json bool
}

// NewJSONL writes one JSON object per line (for ingestion).
func NewJSONL(w io.Writer) *WriterSink { return &WriterSink{w: w, json: true} }

// NewText writes a compact human-readable line.
func NewText(w io.Writer) *WriterSink { return &WriterSink{w: w, json: false} }

func (s *WriterSink) Emit(e event.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.json {
		b, err := json.Marshal(e)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(s.w, "%s\n", b)
		return err
	}
	_, err := fmt.Fprintf(s.w, "[%s] %s %s src=%s target=%q conf=%.2f %s\n",
		e.Time.Format("15:04:05"), e.SeverityS, e.Kind, e.Source, e.Target, e.Confidence, e.Detail)
	return err
}

// MultiSink fans an event out to several sinks, returning the first error (all
// sinks are still attempted).
type MultiSink []Sink

func (m MultiSink) Emit(e event.Event) error {
	var firstErr error
	for _, s := range m {
		if err := s.Emit(e); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// Discard drops every event (useful in tests / observe-silent setups).
var Discard Sink = discard{}

type discard struct{}

func (discard) Emit(event.Event) error { return nil }
