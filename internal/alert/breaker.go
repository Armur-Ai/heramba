package alert

import (
	"errors"
	"sync"
	"time"

	"github.com/Armur-Ai/heramba/internal/event"
)

// ErrCircuitOpen is returned by a Breaker that is fast-failing because its inner
// sink has been failing. It lets a MultiSink record the drop while the engine
// stays fail-soft — the detection is never delivered through this sink until the
// circuit recovers, but the caller never pays the inner sink's timeout.
var ErrCircuitOpen = errors.New("alert: circuit open")

// breakerState is the circuit's phase.
type breakerState int

const (
	breakerClosed   breakerState = iota // healthy: calls pass through
	breakerOpen                         // tripped: calls fast-fail until cooldown elapses
	breakerHalfOpen                     // probing: a single call is allowed through
)

// BreakerOptions configures a Breaker. Zero values fall back to safe defaults.
type BreakerOptions struct {
	// FailureThreshold is the number of consecutive failures that trips the
	// circuit open. <= 0 uses the default (5).
	FailureThreshold int
	// Cooldown is how long the circuit stays open before a half-open probe.
	// <= 0 uses the default (30s).
	Cooldown time.Duration
}

const (
	defaultBreakerThreshold = 5
	defaultBreakerCooldown  = 30 * time.Second
)

// Breaker wraps any Sink with a circuit breaker so a dead SIEM/webhook endpoint
// doesn't make every detection pay a full timeout. After FailureThreshold
// consecutive failures it opens and fast-fails (returning ErrCircuitOpen without
// calling the inner sink) for Cooldown, then half-opens to let a single probe
// through: success closes the circuit, failure reopens it.
//
// It is an optional, additive wrapper — the inner sink's constructor is
// unchanged — and is safe for concurrent use.
type Breaker struct {
	inner     Sink
	threshold int
	cooldown  time.Duration
	now       func() time.Time // injectable clock for tests

	mu        sync.Mutex
	state     breakerState
	failures  int
	openUntil time.Time
	probing   bool // a half-open probe is currently in flight
}

// NewBreaker wraps inner with a circuit breaker. A nil inner becomes Discard.
func NewBreaker(inner Sink, opts BreakerOptions) *Breaker {
	if inner == nil {
		inner = Discard
	}
	threshold := opts.FailureThreshold
	if threshold <= 0 {
		threshold = defaultBreakerThreshold
	}
	cooldown := opts.Cooldown
	if cooldown <= 0 {
		cooldown = defaultBreakerCooldown
	}
	return &Breaker{
		inner:     inner,
		threshold: threshold,
		cooldown:  cooldown,
		now:       time.Now,
		state:     breakerClosed,
	}
}

// Emit forwards to the inner sink unless the circuit is open, in which case it
// fast-fails with ErrCircuitOpen.
func (b *Breaker) Emit(e event.Event) error {
	proceed, halfOpenProbe := b.beforeCall()
	if !proceed {
		return ErrCircuitOpen
	}
	err := b.inner.Emit(e)
	b.afterCall(halfOpenProbe, err)
	return err
}

// beforeCall decides whether this Emit may call the inner sink. It returns
// proceed, and whether this call is the single half-open probe.
func (b *Breaker) beforeCall() (proceed, halfOpenProbe bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	switch b.state {
	case breakerClosed:
		return true, false
	case breakerOpen:
		if !b.now().Before(b.openUntil) {
			// Cooldown elapsed: promote to half-open and take the probe slot.
			b.state = breakerHalfOpen
			b.probing = true
			return true, true
		}
		return false, false
	case breakerHalfOpen:
		if b.probing {
			return false, false // another probe is already in flight
		}
		b.probing = true
		return true, true
	default:
		return true, false
	}
}

// afterCall records the result and advances the state machine.
func (b *Breaker) afterCall(halfOpenProbe bool, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if halfOpenProbe {
		b.probing = false
		if err == nil {
			b.state = breakerClosed
			b.failures = 0
		} else {
			b.state = breakerOpen
			b.openUntil = b.now().Add(b.cooldown)
		}
		return
	}
	// Closed-state accounting.
	if err == nil {
		b.failures = 0
		return
	}
	b.failures++
	if b.failures >= b.threshold {
		b.state = breakerOpen
		b.openUntil = b.now().Add(b.cooldown)
	}
}

// Flush forwards to the inner sink's Flush (if any), so buffered sinks behind a
// Breaker are still drained on shutdown.
func (b *Breaker) Flush() error { return FlushAll(b.inner) }

// isOpen reports whether the circuit is currently open (test/metrics helper).
func (b *Breaker) isOpen() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state == breakerOpen && b.now().Before(b.openUntil)
}
