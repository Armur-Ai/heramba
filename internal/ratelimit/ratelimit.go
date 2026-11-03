// Package ratelimit is a per-source token-bucket limiter used by the edge in
// PROTECT mode to throttle abusive sources. It is never engaged in observe mode
// (observe must not alter traffic), and it is off unless a rate is configured.
package ratelimit

import (
	"sync"
	"time"
)

// Limiter enforces a per-key (per-source) request rate with a burst allowance.
// Safe for concurrent use.
type Limiter struct {
	rps   float64
	burst float64
	now   func() time.Time

	mu      sync.Mutex
	buckets map[string]*bucket
}

type bucket struct {
	tokens float64
	last   time.Time
}

// New builds a limiter allowing rps requests/second per key with the given
// burst (minimum 1). A non-positive rps yields a nil-behavior limiter that
// allows everything (see Allow).
func New(rps, burst float64) *Limiter {
	if burst < 1 {
		burst = 1
	}
	return &Limiter{rps: rps, burst: burst, now: time.Now, buckets: map[string]*bucket{}}
}

// Allow reports whether a request from key may proceed, consuming a token.
// A limiter with rps <= 0 always allows (disabled).
func (l *Limiter) Allow(key string) bool {
	if l == nil || l.rps <= 0 {
		return true
	}
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()
	b := l.buckets[key]
	if b == nil {
		b = &bucket{tokens: l.burst, last: now}
		l.buckets[key] = b
	}
	// Refill by elapsed time, capped at burst.
	elapsed := now.Sub(b.last).Seconds()
	if elapsed > 0 {
		b.tokens += elapsed * l.rps
		if b.tokens > l.burst {
			b.tokens = l.burst
		}
		b.last = now
	}
	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}
