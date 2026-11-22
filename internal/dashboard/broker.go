package dashboard

import (
	"sync"

	"github.com/Armur-Ai/heramba/internal/event"
)

// StreamFrame is one message fanned out over the live stream (SSE). Exactly one
// of Event / Metrics is populated per frame; Type names which, so a client can
// dispatch without inspecting the payload. Frames are JSON-encoded onto the wire
// as `data: {json}\n\n`.
type StreamFrame struct {
	Type    string       `json:"type"` // "event" | "metrics"
	Event   *event.Event `json:"event,omitempty"`
	Metrics *metricsJSON `json:"metrics,omitempty"`
}

// EventFrame wraps a detection event as a stream frame.
func EventFrame(e event.Event) StreamFrame { return StreamFrame{Type: "event", Event: &e} }

// subBuffer is the per-subscriber channel depth. A subscriber that cannot keep
// up has frames dropped (newest-wins is not attempted — the client reconnects
// and re-polls to resynchronise), so one slow reader never stalls publishers.
const subBuffer = 64

// subscriber is one connected stream client.
type subscriber struct {
	ch chan StreamFrame
}

// Broker is a tiny channel fan-out pub/sub for the live stream. Publishers call
// Publish (non-blocking, drop-on-slow); each SSE connection subscribes for the
// life of its request and unsubscribes on context cancel. It is safe for
// concurrent use and leaks no goroutines: it owns none — fan-out happens inline
// on the publisher's goroutine.
type Broker struct {
	mu     sync.Mutex
	subs   map[*subscriber]struct{}
	closed bool
}

// NewBroker returns an empty, ready broker.
func NewBroker() *Broker {
	return &Broker{subs: map[*subscriber]struct{}{}}
}

// Publish fans a frame out to every current subscriber without blocking. A
// subscriber whose buffer is full has this frame dropped rather than slowing the
// publisher — the signature of a stalled client. Publishing after Close is a
// no-op.
func (b *Broker) Publish(f StreamFrame) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	for s := range b.subs {
		select {
		case s.ch <- f:
		default: // slow subscriber: drop this frame
		}
	}
}

// subscribe registers a new subscriber and returns it. The returned channel is
// buffered; the caller must unsubscribe when done.
func (b *Broker) subscribe() *subscriber {
	s := &subscriber{ch: make(chan StreamFrame, subBuffer)}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.subs == nil {
		b.subs = map[*subscriber]struct{}{}
	}
	b.subs[s] = struct{}{}
	return s
}

// unsubscribe removes a subscriber and closes its channel. It is idempotent.
// Closing happens under the same lock Publish holds, so no send can race a
// close.
func (b *Broker) unsubscribe(s *subscriber) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.subs[s]; !ok {
		return
	}
	delete(b.subs, s)
	close(s.ch)
}

// Close drops every subscriber and marks the broker closed; further Publish
// calls are no-ops. It is safe to call more than once.
func (b *Broker) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	b.closed = true
	for s := range b.subs {
		delete(b.subs, s)
		close(s.ch)
	}
}

// Subscribers reports the number of currently connected subscribers. It exists
// for observability and tests (leak detection on context cancel).
func (b *Broker) Subscribers() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.subs)
}
