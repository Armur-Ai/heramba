package alert

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"strconv"
	"time"
)

// httpSender is the shared, resilient HTTP-delivery core used by every
// webhook-style push sink (webhook/slack/splunk/datadog/…). It centralizes the
// behavior those sinks must get right in front of a production security product:
//
//   - a per-attempt timeout (context-based) so a hung server can never block the
//     caller past a bounded wall-clock budget;
//   - bounded retries with exponential backoff + jitter, honoring Retry-After on
//     429/503, and treating other 4xx as permanent (no retry);
//   - a final error surfaced to the caller without ever panicking.
//
// Retries are OFF by default (MaxAttempts == 1) so that wrapping a sink onto
// this helper preserves the exact observable behavior of the hand-rolled POST it
// replaces. A caller (e.g. cmd/heramba/serve.go) opts a sink into retries with
// WithHTTPRetry, which keeps the existing constructor signatures untouched.
//
// httpSender is safe for concurrent use: it holds no per-request mutable state.
type httpSender struct {
	client  *http.Client
	timeout time.Duration
	policy  retryPolicy

	// Injectable seams for deterministic tests (never nil in a live sink).
	now   func() time.Time
	sleep func(ctx context.Context, d time.Duration) error
	rand  func() float64 // returns a value in [0,1) for jitter
}

// retryPolicy bounds the retry loop. MaxAttempts counts the first try, so
// MaxAttempts == 1 means "no retries".
type retryPolicy struct {
	MaxAttempts int
	BaseBackoff time.Duration
	MaxBackoff  time.Duration
}

// RetryOptions configures retries for an HTTP push sink. Zero/negative fields
// fall back to safe defaults. It is the public shape callers pass to
// WithHTTPRetry.
type RetryOptions struct {
	// MaxAttempts is the total number of attempts including the first. Values
	// <= 1 disable retries; <= 0 uses the default (3).
	MaxAttempts int
	// BaseBackoff is the delay before the second attempt; it doubles each
	// subsequent attempt up to MaxBackoff. <= 0 uses the default (200ms).
	BaseBackoff time.Duration
	// MaxBackoff caps any single backoff (and a server-supplied Retry-After), so
	// delivery can never block for an unbounded time. <= 0 uses the default (5s).
	MaxBackoff time.Duration
}

const (
	defaultHTTPTimeout      = 5 * time.Second
	defaultRetryAttempts    = 3
	defaultRetryBaseBackoff = 200 * time.Millisecond
	defaultRetryMaxBackoff  = 5 * time.Second
)

// newHTTPSender builds a sender with retries disabled (single attempt). timeout
// <= 0 falls back to defaultHTTPTimeout. The underlying *http.Client also carries
// the timeout, matching the pre-refactor sinks (which used client.Timeout).
func newHTTPSender(timeout time.Duration) *httpSender {
	if timeout <= 0 {
		timeout = defaultHTTPTimeout
	}
	return &httpSender{
		client:  &http.Client{Timeout: timeout},
		timeout: timeout,
		policy:  retryPolicy{MaxAttempts: 1, BaseBackoff: defaultRetryBaseBackoff, MaxBackoff: defaultRetryMaxBackoff},
		now:     time.Now,
		sleep:   sleepCtx,
		rand:    rand.Float64,
	}
}

// setRetry applies a retry policy. It is promoted onto every HTTP sink (which
// embeds *httpSender), so WithHTTPRetry can reach it generically.
func (s *httpSender) setRetry(o RetryOptions) {
	p := retryPolicy{
		MaxAttempts: o.MaxAttempts,
		BaseBackoff: o.BaseBackoff,
		MaxBackoff:  o.MaxBackoff,
	}
	if p.MaxAttempts == 0 {
		p.MaxAttempts = defaultRetryAttempts
	}
	if p.MaxAttempts < 1 {
		p.MaxAttempts = 1
	}
	if p.BaseBackoff <= 0 {
		p.BaseBackoff = defaultRetryBaseBackoff
	}
	if p.MaxBackoff <= 0 {
		p.MaxBackoff = defaultRetryMaxBackoff
	}
	if p.MaxBackoff < p.BaseBackoff {
		p.MaxBackoff = p.BaseBackoff
	}
	s.policy = p
}

// WithHTTPRetry enables bounded retries (exponential backoff + jitter, honoring
// Retry-After on 429/503) on an HTTP push sink created by this package, and
// returns s for chaining. It is a no-op for sinks that do not perform HTTP
// delivery (e.g. the file, ring or counting sinks), so it is safe to apply
// uniformly. This is the opt-in path for serve.go — the sink constructors are
// unchanged.
func WithHTTPRetry(s Sink, o RetryOptions) Sink {
	if c, ok := s.(interface{ setRetry(RetryOptions) }); ok {
		c.setRetry(o)
	}
	return s
}

// httpPost describes one delivery. The caller marshals the body; the sender owns
// the transport, timeout and retry concerns.
type httpPost struct {
	url         string
	body        []byte
	contentType string
	headers     map[string]string // extra headers; empty values are skipped
	basicUser   string
	basicPass   string
	useBasic    bool
}

// deliver POSTs p, retrying per the policy. name prefixes errors (e.g. "webhook")
// so messages match the pre-refactor sinks. It never panics and never blocks
// past attempts*(timeout+maxBackoff).
func (s *httpSender) deliver(name string, p httpPost) error {
	attempts := s.policy.MaxAttempts
	if attempts < 1 {
		attempts = 1
	}
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		err, retryable, retryAfter := s.attempt(name, p)
		if err == nil {
			return nil
		}
		lastErr = err
		if !retryable || attempt == attempts {
			return lastErr
		}
		wait := s.backoff(attempt, retryAfter)
		if serr := s.sleep(context.Background(), wait); serr != nil {
			return lastErr
		}
	}
	return lastErr
}

// attempt performs a single request. It returns the error (nil on success),
// whether the failure is worth retrying, and any server-requested Retry-After.
func (s *httpSender) attempt(name string, p httpPost) (err error, retryable bool, retryAfter time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()

	req, rerr := http.NewRequestWithContext(ctx, http.MethodPost, p.url, bytes.NewReader(p.body))
	if rerr != nil {
		// A malformed URL/request can't be fixed by retrying.
		return fmt.Errorf("%s: %w", name, rerr), false, 0
	}
	if p.contentType != "" {
		req.Header.Set("Content-Type", p.contentType)
	}
	for k, v := range p.headers {
		if v != "" {
			req.Header.Set(k, v)
		}
	}
	if p.useBasic {
		req.SetBasicAuth(p.basicUser, p.basicPass)
	}

	resp, derr := s.client.Do(req)
	if derr != nil {
		// Network errors / timeouts are transient: retry.
		return fmt.Errorf("%s: %w", name, derr), true, 0
	}
	defer resp.Body.Close()
	// Drain (bounded) so the connection can be reused by keep-alive.
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))

	if resp.StatusCode < 300 {
		return nil, false, 0
	}
	statusErr := fmt.Errorf("%s: non-2xx status %d", name, resp.StatusCode)
	switch {
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable:
		return statusErr, true, parseRetryAfter(resp.Header.Get("Retry-After"), s.now())
	case resp.StatusCode >= 500:
		return statusErr, true, 0
	default:
		// 3xx (redirect not followed) and 4xx other than 429: permanent.
		return statusErr, false, 0
	}
}

// backoff computes the delay before the given (1-based) attempt's retry:
// exponential base*2^(attempt-1) capped at MaxBackoff, multiplied by jitter in
// [0.5,1.5). A server Retry-After (also capped) takes precedence when larger, so
// we never hammer a throttling endpoint — but never block longer than MaxBackoff.
func (s *httpSender) backoff(attempt int, retryAfter time.Duration) time.Duration {
	base := float64(s.policy.BaseBackoff) * math.Pow(2, float64(attempt-1))
	if max := float64(s.policy.MaxBackoff); base > max {
		base = max
	}
	jitter := 0.5 + s.rand() // [0.5, 1.5)
	d := time.Duration(base * jitter)
	if d > s.policy.MaxBackoff {
		d = s.policy.MaxBackoff
	}
	if retryAfter > 0 {
		if retryAfter > s.policy.MaxBackoff {
			retryAfter = s.policy.MaxBackoff
		}
		if retryAfter > d {
			d = retryAfter
		}
	}
	if d < 0 {
		d = 0
	}
	return d
}

// parseRetryAfter parses an HTTP Retry-After header: either delta-seconds or an
// HTTP-date. It returns 0 for empty/unparseable/past values.
func parseRetryAfter(v string, now time.Time) time.Duration {
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil {
		if secs <= 0 {
			return 0
		}
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := t.Sub(now); d > 0 {
			return d
		}
	}
	return 0
}

// sleepCtx sleeps for d or until ctx is done, whichever is first. It returns
// ctx.Err() if the context ended early.
func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
