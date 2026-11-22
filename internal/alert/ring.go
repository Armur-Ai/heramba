package alert

import (
	"sync"

	"github.com/Armur-Ai/heramba/internal/event"
)

// RingSink keeps a bounded, concurrent-safe buffer of the most recent events.
// It is the data source for the admin /api/events endpoint and the web
// dashboard: placed in the sink fan-out, it sees every detection but only ever
// retains the last N, so memory is bounded regardless of attack volume.
type RingSink struct {
	mu    sync.Mutex
	buf   []event.Event
	size  int
	next  int // index to write the next event
	count int // number of events held (<= size)
}

// NewRing creates a ring buffer holding the last size events (size<=0 => 1).
func NewRing(size int) *RingSink {
	if size <= 0 {
		size = 1
	}
	return &RingSink{buf: make([]event.Event, size), size: size}
}

// Emit records the event, overwriting the oldest once full.
func (r *RingSink) Emit(e event.Event) error {
	r.mu.Lock()
	r.buf[r.next] = e
	r.next = (r.next + 1) % r.size
	if r.count < r.size {
		r.count++
	}
	r.mu.Unlock()
	return nil
}

// Recent returns up to n of the most recent events, newest first. n<=0 or n >
// the number held returns everything held (newest first).
func (r *RingSink) Recent(n int) []event.Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	if n <= 0 || n > r.count {
		n = r.count
	}
	out := make([]event.Event, 0, n)
	// Most recent is at (next-1); walk backwards.
	idx := (r.next - 1 + r.size) % r.size
	for i := 0; i < n; i++ {
		out = append(out, r.buf[idx])
		idx = (idx - 1 + r.size) % r.size
	}
	return out
}

// Len returns the number of events currently held.
func (r *RingSink) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.count
}
