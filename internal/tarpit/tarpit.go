// Package tarpit implements legitimate active defense: once a source is
// *confirmed* malicious (it tripped a wire, hit a canary, exhausted a decoy),
// make its own future requests to us crawl — dripping our response out slowly —
// to burn an automated attacker's wall-clock and budget.
//
// This never attacks, scans, or sends traffic to the client. A tarpit only
// delays and limits the attacker's own request against OUR endpoint, serving
// bounded, synthetic content. Every wait honors context cancellation and spawns
// no goroutine of its own, so a disconnecting client leaks nothing.
package tarpit

import (
	"context"
	"io"
	"math/rand"
	"net/http"
	"sync"
	"time"
)

// Config controls how slowly a Tarpit drips a response.
type Config struct {
	ChunkSize int           // bytes written per drip (default 1)
	Interval  time.Duration // base delay before each drip after the first
	Jitter    time.Duration // uniform random in [0,Jitter) added to each Interval
	MaxTotal  time.Duration // optional cap on total drip time (0 = no cap)
}

func (c Config) normalized() Config {
	if c.ChunkSize <= 0 {
		c.ChunkSize = 1
	}
	if c.Interval < 0 {
		c.Interval = 0
	}
	if c.Jitter < 0 {
		c.Jitter = 0
	}
	return c
}

// Tarpit drips responses on a slow schedule. It is safe for concurrent use.
type Tarpit struct {
	cfg Config

	mu  sync.Mutex
	rng *rand.Rand
}

// New returns a Tarpit with the given config (zero values are normalized to
// sane defaults: 1 byte per Interval).
func New(cfg Config) *Tarpit {
	return &Tarpit{
		cfg: cfg.normalized(),
		rng: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// NewSeeded is New with a fixed RNG seed, so jitter is reproducible in tests.
func NewSeeded(cfg Config, seed int64) *Tarpit {
	return &Tarpit{cfg: cfg.normalized(), rng: rand.New(rand.NewSource(seed))}
}

// jitter returns a random delay in [0, Jitter).
func (t *Tarpit) jitter() time.Duration {
	if t.cfg.Jitter <= 0 {
		return 0
	}
	t.mu.Lock()
	n := t.rng.Int63n(int64(t.cfg.Jitter))
	t.mu.Unlock()
	return time.Duration(n)
}

// Drip writes body to w one ChunkSize chunk at a time, waiting Interval(+jitter)
// before every chunk after the first and flushing w if it supports it. It runs
// synchronously in the caller's goroutine — no goroutine is spawned, so nothing
// can leak. It returns the number of bytes written and an error: ctx.Err() if
// the context is cancelled or the MaxTotal budget elapses, or any write error.
func (t *Tarpit) Drip(ctx context.Context, w io.Writer, body []byte) (int, error) {
	flusher, _ := w.(http.Flusher)
	deadline := time.Time{}
	if t.cfg.MaxTotal > 0 {
		deadline = time.Now().Add(t.cfg.MaxTotal)
	}

	written := 0
	first := true
	for off := 0; off < len(body); off += t.cfg.ChunkSize {
		if !first {
			if err := t.wait(ctx, t.cfg.Interval+t.jitter()); err != nil {
				return written, err
			}
			if !deadline.IsZero() && time.Now().After(deadline) {
				return written, context.DeadlineExceeded
			}
		}
		first = false

		end := off + t.cfg.ChunkSize
		if end > len(body) {
			end = len(body)
		}
		n, err := w.Write(body[off:end])
		written += n
		if err != nil {
			return written, err
		}
		if flusher != nil {
			flusher.Flush()
		}
	}
	return written, nil
}

// wait blocks for d or until ctx is done, whichever comes first, without leaking
// a timer. A non-positive d still checks ctx so a cancelled context is honored.
func (t *Tarpit) wait(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			return nil
		}
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// ServeHandler returns an http.Handler that drips bodyFor(r) out slowly. bodyFor
// lets the caller choose what synthetic content to serve (e.g. a labyrinth page
// or a canned fake). The request's context cancellation stops the drip.
func (t *Tarpit) ServeHandler(contentType string, bodyFor func(*http.Request) []byte) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if contentType != "" {
			w.Header().Set("Content-Type", contentType)
		}
		_, _ = t.Drip(r.Context(), w, bodyFor(r))
	})
}
