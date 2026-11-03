// Package signal implements signal denial: making the target give an attacker
// (and an autonomous agent's oracles) as little reliable ground truth as
// possible. Two mechanisms ship here:
//
//   - Normalizer strips/standardizes response headers that leak stack, version,
//     and framework details an attacker fingerprints on.
//   - Jitter adds bounded randomized latency so timing-based differential
//     oracles (blind SQLi, user-enumeration via response time) lose their
//     clean signal.
//
// These are deliberately conservative and deterministic in behavior (not the
// response *content* — content-level deception is the decoy package). Richer
// response-shaping recipes are a commercial plugin.
package signal

import (
	"context"
	"math/rand"
	"net/http"
	"time"
)

// leakyHeaders are response headers that commonly reveal stack/version.
var leakyHeaders = []string{
	"Server", "X-Powered-By", "X-AspNet-Version", "X-AspNetMvc-Version",
	"X-Runtime", "X-Version", "X-Generator", "X-Drupal-Cache", "X-Backend-Server",
}

// Normalizer rewrites response headers to deny fingerprinting signal.
type Normalizer struct {
	Strip     []string // header names to remove
	SetServer string   // generic value for Server (empty = remove)
}

// DefaultNormalizer removes the common leaky headers and presents a generic
// Server banner.
func DefaultNormalizer() *Normalizer {
	return &Normalizer{Strip: append([]string(nil), leakyHeaders...), SetServer: "heramba"}
}

// Apply mutates a response header set in place.
func (n *Normalizer) Apply(h http.Header) {
	for _, name := range n.Strip {
		h.Del(name)
	}
	if n.SetServer != "" {
		h.Set("Server", n.SetServer)
	}
}

// Jitter adds a bounded random delay. Min==Max gives a fixed delay; the zero
// value adds nothing.
type Jitter struct {
	Min, Max time.Duration
	rng      *rand.Rand
}

// NewJitter builds a jitter over [min, max]. If max < min, max is clamped to
// min. A per-instance PRNG keeps it cheap and concurrency-isolated by caller.
func NewJitter(min, max time.Duration) *Jitter {
	if max < min {
		max = min
	}
	return &Jitter{Min: min, Max: max, rng: rand.New(rand.NewSource(time.Now().UnixNano()))}
}

// Delay returns the next random delay in [Min, Max]. Exposed so it is testable
// without sleeping.
func (j *Jitter) Delay() time.Duration {
	if j == nil || j.Max <= 0 {
		return 0
	}
	span := j.Max - j.Min
	if span <= 0 {
		return j.Min
	}
	return j.Min + time.Duration(j.rng.Int63n(int64(span)+1))
}

// Sleep blocks for a random delay or until ctx is done, whichever is first.
func (j *Jitter) Sleep(ctx context.Context) {
	d := j.Delay()
	if d <= 0 {
		return
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}
