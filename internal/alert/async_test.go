package alert

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Armur-Ai/heramba/internal/event"
)

// blockingSink blocks every Emit until released, to simulate a slow/hung sink.
type blockingSink struct {
	release chan struct{}
	calls   atomic.Int64
}

func (b *blockingSink) Emit(event.Event) error {
	b.calls.Add(1)
	<-b.release
	return nil
}

func TestAsyncEmitIsNonBlocking(t *testing.T) {
	blk := &blockingSink{release: make(chan struct{})}
	a := NewAsync(blk, 4)
	defer func() { close(blk.release); _ = a.Close() }()

	// The worker will grab one event and block on it; the queue (cap 4) then
	// fills; everything after must be dropped, never blocking the caller.
	done := make(chan struct{})
	go func() {
		for i := 0; i < 1000; i++ {
			_ = a.Emit(ev())
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Emit blocked under load — hot path was not protected")
	}
	if a.Dropped() == 0 {
		t.Fatal("expected drops once the bounded queue filled")
	}
}

func TestAsyncFlushDrains(t *testing.T) {
	var mu sync.Mutex
	var got int
	counter := sinkFunc(func(event.Event) error {
		mu.Lock()
		got++
		mu.Unlock()
		return nil
	})
	a := NewAsync(counter, 128)
	defer func() { _ = a.Close() }()

	for i := 0; i < 50; i++ {
		_ = a.Emit(ev())
	}
	if err := a.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if got != 50 {
		t.Fatalf("Flush should drain all queued events: got %d/50", got)
	}
}

func TestAsyncDropAccounting(t *testing.T) {
	blk := &blockingSink{release: make(chan struct{})}
	a := NewAsync(blk, 2)
	defer func() { close(blk.release); _ = a.Close() }()

	// Give the worker a moment to pull the first event and block.
	_ = a.Emit(ev())
	deadline := time.Now().Add(time.Second)
	for blk.calls.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	// Queue capacity is 2; fill it then overflow deterministically.
	for i := 0; i < 2; i++ {
		_ = a.Emit(ev())
	}
	before := a.Dropped()
	for i := 0; i < 10; i++ {
		_ = a.Emit(ev())
	}
	if a.Dropped() <= before {
		t.Fatalf("expected additional drops on overflow (before=%d now=%d)", before, a.Dropped())
	}
}

func TestAsyncCloseStopsWorker(t *testing.T) {
	a := NewAsync(Discard, 8)
	_ = a.Emit(ev())
	if err := a.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	// The worker has exited: stopped is closed.
	select {
	case <-a.stopped:
	default:
		t.Fatal("worker goroutine did not stop after Close")
	}
	// Close is idempotent and Emit after close just drops, never panics/blocks.
	if err := a.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}
	if err := a.Emit(ev()); err != nil {
		t.Fatalf("emit after close should be a no-op error, got %v", err)
	}
	if a.Dropped() == 0 {
		t.Fatal("emit after close should count as a drop")
	}
}
