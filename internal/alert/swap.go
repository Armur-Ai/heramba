package alert

import (
	"sync/atomic"

	"github.com/Armur-Ai/heramba/internal/event"
)

// SwapSink is a sink whose delegate can be replaced atomically at runtime. It
// lets the edge hot-reload its alert wiring (e.g. on SIGHUP) without rebuilding
// the sinks that wrap it — the counting sink and detection engine keep emitting
// to the same SwapSink while its inner target changes underneath them.
type SwapSink struct {
	inner atomic.Pointer[Sink]
}

// NewSwap wraps s (nil becomes Discard).
func NewSwap(s Sink) *SwapSink {
	if s == nil {
		s = Discard
	}
	sw := &SwapSink{}
	sw.inner.Store(&s)
	return sw
}

// Swap replaces the delegate. nil becomes Discard.
func (s *SwapSink) Swap(n Sink) {
	if n == nil {
		n = Discard
	}
	s.inner.Store(&n)
}

// Emit forwards to the current delegate.
func (s *SwapSink) Emit(e event.Event) error {
	return (*s.inner.Load()).Emit(e)
}

// Flush flushes the current delegate (and, if it is a MultiSink, its members),
// so buffered sinks behind a SwapSink are still drained on shutdown.
func (s *SwapSink) Flush() error {
	return FlushAll(*s.inner.Load())
}
