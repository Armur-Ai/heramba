package tarpit

import (
	"sync"
	"time"
)

// Growth selects how a source's tarpit delay grows with repeated offenses.
type Growth int

const (
	// Linear: delay = base * offenses.
	Linear Growth = iota
	// Exponential: delay = base * 2^(offenses-1).
	Exponential
)

// Escalator tracks per-source offense counts and maps them to a tarpit delay
// that grows with repetition, capped at Max. The longer an automated attacker
// keeps hammering confirmed-bad surface, the slower we answer — without ever
// growing unbounded. Safe for concurrent use.
type Escalator struct {
	base   time.Duration
	max    time.Duration
	growth Growth

	mu     sync.Mutex
	counts map[string]int
}

// NewEscalator returns an Escalator. base is the first-offense delay; max caps
// the delay (0 or negative max means "no cap"). A zero base disables delay.
func NewEscalator(base, max time.Duration, growth Growth) *Escalator {
	return &Escalator{base: base, max: max, growth: growth, counts: map[string]int{}}
}

// Offense records one more offense by source and returns the delay that now
// applies to it.
func (e *Escalator) Offense(source string) time.Duration {
	e.mu.Lock()
	e.counts[source]++
	n := e.counts[source]
	e.mu.Unlock()
	return e.delayFor(n)
}

// Delay returns the current delay for source without recording a new offense.
// A source with no recorded offenses gets zero.
func (e *Escalator) Delay(source string) time.Duration {
	e.mu.Lock()
	n := e.counts[source]
	e.mu.Unlock()
	if n == 0 {
		return 0
	}
	return e.delayFor(n)
}

// Count returns how many offenses have been recorded for source.
func (e *Escalator) Count(source string) int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.counts[source]
}

// Reset clears a source's offense history (e.g. after an operator clears it).
func (e *Escalator) Reset(source string) {
	e.mu.Lock()
	delete(e.counts, source)
	e.mu.Unlock()
}

// ConfigFor returns a tarpit Config whose Interval is the escalated delay for
// source, so callers can build or tune a Tarpit per source. jitter and
// chunkSize are passed through; a zero chunkSize normalizes to 1 in the Tarpit.
func (e *Escalator) ConfigFor(source string, chunkSize int, jitter time.Duration) Config {
	return Config{ChunkSize: chunkSize, Interval: e.Delay(source), Jitter: jitter}
}

func (e *Escalator) delayFor(n int) time.Duration {
	if n <= 0 || e.base <= 0 {
		return 0
	}
	var d time.Duration
	switch e.growth {
	case Exponential:
		d = e.base
		for i := 1; i < n; i++ {
			d *= 2
			if e.max > 0 && d >= e.max {
				return e.max
			}
		}
	default: // Linear
		// Guard against int64 overflow on pathological counts.
		if e.max > 0 && n > 0 && time.Duration(n) > e.max/e.base+1 {
			return e.max
		}
		d = e.base * time.Duration(n)
	}
	if e.max > 0 && d > e.max {
		return e.max
	}
	return d
}
