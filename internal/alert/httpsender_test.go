package alert

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// recordingSleeper is a deterministic stand-in for sleepCtx: it records each
// requested backoff and returns immediately, so retry tests never sleep.
type recordingSleeper struct{ slept []time.Duration }

func (r *recordingSleeper) sleep(_ context.Context, d time.Duration) error {
	r.slept = append(r.slept, d)
	return nil
}

// newTestSender returns a sender with deterministic jitter (always 0.5 -> *1.0)
// and a recording sleeper, retrying up to attempts times.
func newTestSender(attempts int) (*httpSender, *recordingSleeper) {
	s := newHTTPSender(time.Second)
	s.setRetry(RetryOptions{MaxAttempts: attempts, BaseBackoff: 100 * time.Millisecond, MaxBackoff: time.Second})
	rs := &recordingSleeper{}
	s.sleep = rs.sleep
	s.rand = func() float64 { return 0.5 } // jitter multiplier -> 1.0
	s.now = func() time.Time { return time.Unix(0, 0) }
	return s, rs
}

func TestHTTPSenderSuccessNoRetry(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s, rs := newTestSender(3)
	if err := s.deliver("test", httpPost{url: srv.URL, body: []byte("{}")}); err != nil {
		t.Fatalf("deliver: %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected 1 call, got %d", got)
	}
	if len(rs.slept) != 0 {
		t.Fatalf("no backoff expected on success, got %v", rs.slept)
	}
}

func TestHTTPSenderPermanent4xxNoRetry(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	s, rs := newTestSender(5)
	err := s.deliver("test", httpPost{url: srv.URL, body: []byte("{}")})
	if err == nil {
		t.Fatal("expected error on 403")
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("4xx must not retry: expected 1 call, got %d", got)
	}
	if len(rs.slept) != 0 {
		t.Fatalf("no backoff expected for permanent error, got %v", rs.slept)
	}
}

func TestHTTPSenderRetries5xxThenGivesUp(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	s, rs := newTestSender(3)
	err := s.deliver("test", httpPost{url: srv.URL, body: []byte("{}")})
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Fatalf("expected 3 attempts, got %d", got)
	}
	// Two backoffs between three attempts: 100ms then 200ms (jitter *1.0).
	if len(rs.slept) != 2 || rs.slept[0] != 100*time.Millisecond || rs.slept[1] != 200*time.Millisecond {
		t.Fatalf("unexpected backoff schedule: %v", rs.slept)
	}
}

func TestHTTPSenderRecoversAfterRetry(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&calls, 1) < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s, _ := newTestSender(5)
	if err := s.deliver("test", httpPost{url: srv.URL, body: []byte("{}")}); err != nil {
		t.Fatalf("expected success on 3rd attempt, got %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Fatalf("expected 3 attempts, got %d", got)
	}
}

func TestHTTPSenderHonorsRetryAfter(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.Header().Set("Retry-After", "1") // 1 second
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s, rs := newTestSender(3)
	if err := s.deliver("test", httpPost{url: srv.URL, body: []byte("{}")}); err != nil {
		t.Fatalf("deliver: %v", err)
	}
	if len(rs.slept) != 1 || rs.slept[0] != time.Second {
		t.Fatalf("expected a single 1s Retry-After backoff, got %v", rs.slept)
	}
}

func TestHTTPSenderDefaultIsSingleAttempt(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	s := newHTTPSender(time.Second) // retries OFF by default
	if err := s.deliver("test", httpPost{url: srv.URL, body: []byte("{}")}); err == nil {
		t.Fatal("expected error on 500")
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("default must be a single attempt, got %d calls", got)
	}
}

func TestHTTPSenderHangReturnsWithinBound(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		<-release // hang until the test finishes
	}))
	// Order matters: close(release) must run before srv.Close() (which drains
	// in-flight handlers), so unblock the handler first. Defers are LIFO.
	defer srv.Close()
	defer close(release)

	s := newHTTPSender(50 * time.Millisecond) // short per-attempt timeout, no retries
	start := time.Now()
	err := s.deliver("test", httpPost{url: srv.URL, body: []byte("{}")})
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected a timeout error from a hung server")
	}
	if elapsed > 2*time.Second {
		t.Fatalf("deliver blocked too long on a hung server: %v", elapsed)
	}
}

func TestParseRetryAfter(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		in   string
		want time.Duration
	}{
		{"", 0},
		{"0", 0},
		{"-5", 0},
		{"3", 3 * time.Second},
		{"garbage", 0},
		{now.Add(10 * time.Second).UTC().Format(http.TimeFormat), 10 * time.Second},
		{now.Add(-10 * time.Second).UTC().Format(http.TimeFormat), 0}, // past => 0
	}
	for _, c := range cases {
		if got := parseRetryAfter(c.in, now); got != c.want {
			t.Errorf("parseRetryAfter(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestBackoffBounded(t *testing.T) {
	s := newHTTPSender(time.Second)
	s.setRetry(RetryOptions{MaxAttempts: 10, BaseBackoff: 100 * time.Millisecond, MaxBackoff: time.Second})
	s.rand = func() float64 { return 0.999 } // max jitter (~1.5x)
	for attempt := 1; attempt <= 10; attempt++ {
		if d := s.backoff(attempt, 0); d > time.Second || d < 0 {
			t.Fatalf("attempt %d backoff out of bounds: %v", attempt, d)
		}
	}
	// A huge Retry-After is still capped at MaxBackoff.
	if d := s.backoff(1, time.Hour); d > time.Second {
		t.Fatalf("Retry-After not capped: %v", d)
	}
}
