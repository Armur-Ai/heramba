package alert

import (
	"sort"
	"sync"

	"github.com/Armur-Ai/heramba/internal/event"
)

// CountingSink wraps another sink and tallies events by kind, so the runtime can
// report a detection summary (e.g. on shutdown). It is safe for concurrent use.
type CountingSink struct {
	inner  Sink
	mu     sync.Mutex
	counts map[event.Kind]int
	total  int
}

// Count wraps inner with a tally. If inner is nil, events are only counted.
func Count(inner Sink) *CountingSink {
	if inner == nil {
		inner = Discard
	}
	return &CountingSink{inner: inner, counts: map[event.Kind]int{}}
}

func (c *CountingSink) Emit(e event.Event) error {
	c.mu.Lock()
	c.counts[e.Kind]++
	c.total++
	c.mu.Unlock()
	return c.inner.Emit(e)
}

// Total returns the number of events seen.
func (c *CountingSink) Total() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.total
}

// Tally is one kind's count, for stable ordered reporting.
type Tally struct {
	Kind  event.Kind
	Count int
}

// Snapshot returns the per-kind counts, sorted by count descending then kind.
func (c *CountingSink) Snapshot() []Tally {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]Tally, 0, len(c.counts))
	for k, n := range c.counts {
		out = append(out, Tally{Kind: k, Count: n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Kind < out[j].Kind
	})
	return out
}
