package alert

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Armur-Ai/heramba/internal/event"
)

// errSink always fails; used to prove failures don't crash or stall callers.
type errSink struct{ calls atomic.Int64 }

func (e *errSink) Emit(event.Event) error {
	e.calls.Add(1)
	return errors.New("sink down")
}

func TestMultiSinkFailureDoesNotStopOthers(t *testing.T) {
	bad := &errSink{}
	good := &errSink{} // reuse counter; we only check it was called
	good2 := sinkFunc(func(event.Event) error { return nil })
	var delivered atomic.Int64
	counting := sinkFunc(func(event.Event) error { delivered.Add(1); return nil })

	m := MultiSink{bad, good2, counting, good}
	err := m.Emit(ev())
	if err == nil {
		t.Fatal("MultiSink should surface the first sink error")
	}
	if delivered.Load() != 1 {
		t.Fatal("a failing sink must not prevent later sinks from receiving the event")
	}
	if bad.calls.Load() != 1 || good.calls.Load() != 1 {
		t.Fatal("every sink must be attempted despite an earlier failure")
	}
}

func TestWebhookHangReturnsWithinBound(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		<-release
	}))
	// Defers are LIFO: unblock the handler (close release) before srv.Close drains it.
	defer srv.Close()
	defer close(release)

	sink := NewWebhook(srv.URL, 50*time.Millisecond, event.SeverityInfo)
	start := time.Now()
	err := sink.Emit(ev())
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("a hung server must surface a timeout error")
	}
	if elapsed > 2*time.Second {
		t.Fatalf("Emit blocked too long on a hung server: %v", elapsed)
	}
}

func TestWebhookReturns500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	sink := NewWebhook(srv.URL, time.Second, event.SeverityInfo)
	if err := sink.Emit(ev()); err == nil {
		t.Fatal("expected an error on 500")
	}
}

func TestWebhookRetryAfter429ThenSucceeds(t *testing.T) {
	var calls int32
	var sawBackoff []time.Duration
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.Header().Set("Retry-After", "2")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sink := NewWebhook(srv.URL, time.Second, event.SeverityInfo)
	WithHTTPRetry(sink, RetryOptions{MaxAttempts: 3, BaseBackoff: 100 * time.Millisecond, MaxBackoff: 5 * time.Second})
	// Inject a no-sleep recorder (same package) so the test is instant yet still
	// asserts the Retry-After value was honored.
	sink.sleep = func(_ context.Context, d time.Duration) error { sawBackoff = append(sawBackoff, d); return nil }
	sink.now = func() time.Time { return time.Unix(0, 0) }

	if err := sink.Emit(ev()); err != nil {
		t.Fatalf("expected success after honoring Retry-After, got %v", err)
	}
	if atomic.LoadInt32(&calls) != 2 {
		t.Fatalf("expected 2 attempts (429 then 200), got %d", calls)
	}
	if len(sawBackoff) != 1 || sawBackoff[0] != 2*time.Second {
		t.Fatalf("expected a single 2s Retry-After backoff, got %v", sawBackoff)
	}
}

func TestMultiSinkDeliversDespiteHangingSink(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		<-release
	}))
	// Defers are LIFO: unblock the handler (close release) before srv.Close drains it.
	defer srv.Close()
	defer close(release)

	hung := NewWebhook(srv.URL, 50*time.Millisecond, event.SeverityInfo)
	var delivered atomic.Int64
	good := sinkFunc(func(event.Event) error { delivered.Add(1); return nil })

	m := MultiSink{hung, good}
	start := time.Now()
	_ = m.Emit(ev())
	if time.Since(start) > 2*time.Second {
		t.Fatal("a hanging sink stalled the fan-out beyond its timeout")
	}
	if delivered.Load() != 1 {
		t.Fatal("the healthy sink must still receive the event despite a hanging peer")
	}
}
