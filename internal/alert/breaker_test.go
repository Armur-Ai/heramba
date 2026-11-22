package alert

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Armur-Ai/heramba/internal/event"
)

// flakySink returns a configurable error, counting calls. Safe for concurrent use.
type flakySink struct {
	mu    sync.Mutex
	err   error
	calls int
}

func (f *flakySink) Emit(event.Event) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	return f.err
}

func (f *flakySink) setErr(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = err
}

func (f *flakySink) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func TestBreakerOpensAfterThreshold(t *testing.T) {
	inner := &flakySink{err: errors.New("boom")}
	now := time.Unix(0, 0)
	b := NewBreaker(inner, BreakerOptions{FailureThreshold: 3, Cooldown: time.Minute})
	b.now = func() time.Time { return now }

	// 3 failures trip the circuit.
	for i := 0; i < 3; i++ {
		if err := b.Emit(ev()); err == nil {
			t.Fatalf("attempt %d: expected inner error", i)
		}
	}
	if inner.callCount() != 3 {
		t.Fatalf("expected 3 inner calls before opening, got %d", inner.callCount())
	}
	if !b.isOpen() {
		t.Fatal("circuit should be open after threshold failures")
	}
	// While open: fast-fail with ErrCircuitOpen, inner NOT called.
	if err := b.Emit(ev()); !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen while open, got %v", err)
	}
	if inner.callCount() != 3 {
		t.Fatalf("inner must not be called while open, got %d calls", inner.callCount())
	}
}

func TestBreakerRecoversAfterCooldown(t *testing.T) {
	inner := &flakySink{err: errors.New("boom")}
	now := time.Unix(0, 0)
	b := NewBreaker(inner, BreakerOptions{FailureThreshold: 2, Cooldown: 30 * time.Second})
	b.now = func() time.Time { return now }

	for i := 0; i < 2; i++ {
		_ = b.Emit(ev())
	}
	if !b.isOpen() {
		t.Fatal("expected open circuit")
	}
	// Before cooldown elapses: still fast-fails.
	now = now.Add(10 * time.Second)
	if err := b.Emit(ev()); !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("expected fast-fail before cooldown, got %v", err)
	}

	// Cooldown elapsed and inner now healthy: half-open probe succeeds, closes.
	now = now.Add(30 * time.Second)
	inner.setErr(nil)
	callsBefore := inner.callCount()
	if err := b.Emit(ev()); err != nil {
		t.Fatalf("half-open probe should pass through and succeed, got %v", err)
	}
	if inner.callCount() != callsBefore+1 {
		t.Fatal("half-open probe should call inner exactly once")
	}
	if b.isOpen() {
		t.Fatal("circuit should be closed after a successful probe")
	}
	// Subsequent calls flow normally.
	if err := b.Emit(ev()); err != nil {
		t.Fatalf("closed circuit should pass through, got %v", err)
	}
}

func TestBreakerReopensOnProbeFailure(t *testing.T) {
	inner := &flakySink{err: errors.New("boom")}
	now := time.Unix(0, 0)
	b := NewBreaker(inner, BreakerOptions{FailureThreshold: 1, Cooldown: 10 * time.Second})
	b.now = func() time.Time { return now }

	_ = b.Emit(ev()) // 1 failure => open
	if !b.isOpen() {
		t.Fatal("expected open after single failure")
	}
	now = now.Add(10 * time.Second) // cooldown elapsed
	// Probe runs (inner still failing) => reopen.
	if err := b.Emit(ev()); err == nil {
		t.Fatal("probe should surface the inner error")
	}
	if !b.isOpen() {
		t.Fatal("circuit should reopen after a failed probe")
	}
	// Immediately after reopen, fast-fail again.
	if err := b.Emit(ev()); !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("expected fast-fail after reopen, got %v", err)
	}
}

func TestBreakerSuccessResetsFailures(t *testing.T) {
	inner := &flakySink{err: errors.New("boom")}
	b := NewBreaker(inner, BreakerOptions{FailureThreshold: 3, Cooldown: time.Minute})
	b.now = func() time.Time { return time.Unix(0, 0) }

	_ = b.Emit(ev())
	_ = b.Emit(ev()) // 2 failures
	inner.setErr(nil)
	_ = b.Emit(ev()) // success resets counter
	inner.setErr(errors.New("boom"))
	_ = b.Emit(ev())
	_ = b.Emit(ev()) // 2 more failures, not yet 3 consecutive
	if b.isOpen() {
		t.Fatal("a success should have reset the consecutive-failure count")
	}
}
