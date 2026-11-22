package alert

import (
	"sync"
	"sync/atomic"

	"github.com/Armur-Ai/heramba/internal/event"
)

// AsyncSink hands Emit off to a bounded background queue so a slow or hung sink
// can never slow the edge hot path. Emit is non-blocking: when the queue is full
// the event is dropped (and counted) rather than blocking the caller. A single
// worker goroutine drains the queue in FIFO order into the inner sink. Flush
// blocks until everything queued before it has been delivered; Close stops the
// worker cleanly (no goroutine leak).
//
// It is an optional, additive wrapper — the inner sink's constructor is
// unchanged. Typical placement: AsyncSink in front of a Breaker in front of a
// network sink, so the hot path only ever does a channel send.
type AsyncSink struct {
	inner   Sink
	queue   chan asyncMsg
	quit    chan struct{}
	stopped chan struct{}

	closeOnce sync.Once
	closed    atomic.Bool
	dropped   atomic.Int64
}

// asyncMsg is either an event to deliver or a flush barrier (ack != nil).
type asyncMsg struct {
	e   event.Event
	ack chan struct{}
}

// NewAsync wraps inner with a bounded async queue of the given size (<=0 => 1024)
// and starts its worker. Call Close on shutdown to stop the worker.
func NewAsync(inner Sink, queueSize int) *AsyncSink {
	if inner == nil {
		inner = Discard
	}
	if queueSize <= 0 {
		queueSize = 1024
	}
	s := &AsyncSink{
		inner:   inner,
		queue:   make(chan asyncMsg, queueSize),
		quit:    make(chan struct{}),
		stopped: make(chan struct{}),
	}
	go s.run()
	return s
}

// run is the single worker: it delivers events and acknowledges flush barriers
// until Close signals quit.
func (s *AsyncSink) run() {
	defer close(s.stopped)
	for {
		select {
		case <-s.quit:
			return
		case m := <-s.queue:
			if m.ack != nil {
				close(m.ack)
				continue
			}
			_ = s.inner.Emit(m.e)
		}
	}
}

// Emit enqueues the event without blocking. If the queue is full (or the sink is
// closed) the event is dropped and counted, and Emit returns nil — the hot path
// must never be slowed or failed by a telemetry backlog.
func (s *AsyncSink) Emit(e event.Event) error {
	if s.closed.Load() {
		s.dropped.Add(1)
		return nil
	}
	select {
	case s.queue <- asyncMsg{e: e}:
	default:
		s.dropped.Add(1)
	}
	return nil
}

// Dropped returns the number of events dropped due to a full queue (or emits
// after Close).
func (s *AsyncSink) Dropped() int64 { return s.dropped.Load() }

// Flush blocks until every event enqueued before this call has been delivered to
// the inner sink, then flushes the inner sink if it buffers. Safe to call on
// shutdown (before Close).
func (s *AsyncSink) Flush() error {
	if s.closed.Load() {
		return FlushAll(s.inner)
	}
	ack := make(chan struct{})
	select {
	case s.queue <- asyncMsg{ack: ack}:
		select {
		case <-ack:
		case <-s.stopped: // worker stopped while we waited
		}
	case <-s.stopped:
	}
	return FlushAll(s.inner)
}

// Close stops the worker goroutine and waits for it to exit. It is idempotent.
// Events still queued may not be delivered — call Flush first to drain.
func (s *AsyncSink) Close() error {
	s.closeOnce.Do(func() {
		s.closed.Store(true)
		close(s.quit)
	})
	<-s.stopped
	return nil
}
