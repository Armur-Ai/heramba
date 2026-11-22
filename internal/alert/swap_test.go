package alert

import (
	"testing"

	"github.com/Armur-Ai/heramba/internal/event"
)

func TestSwapSinkDelegatesAndSwaps(t *testing.T) {
	a, b := Count(nil), Count(nil)
	sw := NewSwap(a)

	_ = sw.Emit(event.New(event.KindProbe, event.SeverityLow, "x", "/x", ""))
	if a.Total() != 1 || b.Total() != 0 {
		t.Fatalf("pre-swap routing wrong: a=%d b=%d", a.Total(), b.Total())
	}

	sw.Swap(b)
	_ = sw.Emit(event.New(event.KindProbe, event.SeverityLow, "y", "/y", ""))
	if a.Total() != 1 || b.Total() != 1 {
		t.Fatalf("post-swap routing wrong: a=%d b=%d", a.Total(), b.Total())
	}
}

func TestSwapSinkNilBecomesDiscard(t *testing.T) {
	sw := NewSwap(nil)
	if err := sw.Emit(event.New(event.KindProbe, event.SeverityInfo, "", "", "")); err != nil {
		t.Fatalf("nil swap should discard, got %v", err)
	}
	sw.Swap(nil) // must not panic
	if err := sw.Emit(event.New(event.KindProbe, event.SeverityInfo, "", "", "")); err != nil {
		t.Fatal(err)
	}
}

// flushRecorder is a Flusher that records flush calls.
type flushRecorder struct{ flushed bool }

func (f *flushRecorder) Emit(event.Event) error { return nil }
func (f *flushRecorder) Flush() error           { f.flushed = true; return nil }

func TestSwapSinkFlushReachesInner(t *testing.T) {
	fr := &flushRecorder{}
	sw := NewSwap(MultiSink{Discard, fr})
	if err := sw.Flush(); err != nil {
		t.Fatal(err)
	}
	if !fr.flushed {
		t.Error("SwapSink.Flush must reach a buffered inner sink")
	}
}
