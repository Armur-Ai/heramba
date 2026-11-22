package alert

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Armur-Ai/heramba/internal/event"
)

func TestSleepCtx(t *testing.T) {
	if err := sleepCtx(context.Background(), 0); err != nil {
		t.Fatalf("zero duration should return nil, got %v", err)
	}
	if err := sleepCtx(context.Background(), time.Millisecond); err != nil {
		t.Fatalf("timer path should return nil, got %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := sleepCtx(ctx, time.Hour); err == nil {
		t.Fatal("cancelled context should return its error")
	}
}

func TestSetRetryClamps(t *testing.T) {
	s := newHTTPSender(time.Second)

	s.setRetry(RetryOptions{MaxAttempts: -5, BaseBackoff: -1, MaxBackoff: -1})
	if s.policy.MaxAttempts != 1 {
		t.Fatalf("negative attempts should clamp to 1, got %d", s.policy.MaxAttempts)
	}
	if s.policy.BaseBackoff != defaultRetryBaseBackoff || s.policy.MaxBackoff != defaultRetryMaxBackoff {
		t.Fatal("non-positive backoffs should fall back to defaults")
	}

	s.setRetry(RetryOptions{MaxAttempts: 0})
	if s.policy.MaxAttempts != defaultRetryAttempts {
		t.Fatalf("zero attempts should use the default, got %d", s.policy.MaxAttempts)
	}

	s.setRetry(RetryOptions{MaxAttempts: 3, BaseBackoff: time.Second, MaxBackoff: time.Millisecond})
	if s.policy.MaxBackoff != s.policy.BaseBackoff {
		t.Fatalf("MaxBackoff below BaseBackoff should clamp up, got %v", s.policy.MaxBackoff)
	}
}

func TestWithHTTPRetryNoopOnNonHTTPSink(t *testing.T) {
	// A non-HTTP sink is returned unchanged (and never panics).
	if got := WithHTTPRetry(Discard, RetryOptions{MaxAttempts: 9}); got != Discard {
		t.Fatal("WithHTTPRetry should return the same sink for non-HTTP sinks")
	}
	// An HTTP sink is configured in place.
	w := NewWebhook("http://example.invalid", time.Second, event.SeverityInfo)
	if got := WithHTTPRetry(w, RetryOptions{MaxAttempts: 4}); got != w {
		t.Fatal("WithHTTPRetry should return the same HTTP sink")
	}
	if w.policy.MaxAttempts != 4 {
		t.Fatalf("retry policy not applied, got %d", w.policy.MaxAttempts)
	}
}

func TestNilInnerDefaultsToDiscard(t *testing.T) {
	if err := NewBreaker(nil, BreakerOptions{}).Emit(ev()); err != nil {
		t.Fatalf("breaker over nil inner: %v", err)
	}
	a := NewAsync(nil, 4)
	defer func() { _ = a.Close() }()
	if err := a.Emit(ev()); err != nil {
		t.Fatalf("async over nil inner: %v", err)
	}
	if err := NewRedacting(nil, 0).Emit(ev()); err != nil {
		t.Fatalf("redacting over nil inner: %v", err)
	}
}

func TestDeliverStopsWhenSleepInterrupted(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusInternalServerError) // retryable
	}))
	defer srv.Close()

	s := newHTTPSender(time.Second)
	s.setRetry(RetryOptions{MaxAttempts: 5, BaseBackoff: time.Millisecond, MaxBackoff: time.Second})
	s.rand = func() float64 { return 0 }
	// A sleep that reports interruption makes deliver give up without exhausting
	// all attempts (mirrors a cancelled shutdown).
	s.sleep = func(context.Context, time.Duration) error { return context.Canceled }

	if err := s.deliver("test", httpPost{url: srv.URL, body: []byte("{}")}); err == nil {
		t.Fatal("expected the retryable error to be surfaced")
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("interrupted backoff should stop after the first attempt, got %d", got)
	}
}

func TestBreakerHalfOpenAllowsSingleProbe(t *testing.T) {
	var calls atomic.Int64
	entered := make(chan struct{})
	release := make(chan struct{})
	inner := sinkFunc(func(event.Event) error {
		if calls.Add(1) == 1 {
			return errors.New("boom") // first call trips the circuit open
		}
		close(entered) // the probe reached the inner sink
		<-release      // hold the probe in flight
		return nil
	})

	now := time.Unix(0, 0)
	b := NewBreaker(inner, BreakerOptions{FailureThreshold: 1, Cooldown: 10 * time.Second})
	b.now = func() time.Time { return now }

	_ = b.Emit(ev()) // trip open
	now = now.Add(10 * time.Second)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() { defer wg.Done(); _ = b.Emit(ev()) }() // becomes the half-open probe, blocks
	<-entered

	// While the single probe is in flight, other calls fast-fail.
	if err := b.Emit(ev()); !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("concurrent half-open call should fast-fail, got %v", err)
	}
	close(release)
	wg.Wait()
	if calls.Load() != 2 {
		t.Fatalf("inner should be called exactly twice (trip + probe), got %d", calls.Load())
	}
}
