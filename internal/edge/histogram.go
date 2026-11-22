package edge

import (
	"math"
	"sort"
	"sync/atomic"
)

// scoreBucketBounds are the inclusive upper bounds for the agentic-score
// histogram (scores are 0..1). A final implicit +Inf bucket catches the rest.
var scoreBucketBounds = []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0}

// ScoreHistogram is a lock-free histogram of per-request agentic classifier
// scores. Observe is called on the request hot path, so it does only a handful
// of atomic ops. It is cumulative across config reloads (it lives on the Engine,
// not the reloadable snapshot).
type ScoreHistogram struct {
	counts  []atomic.Uint64 // one per bucket bound, plus a trailing +Inf bucket
	sumBits atomic.Uint64   // float64 bits of the running score sum
	total   atomic.Uint64
}

func newScoreHistogram() *ScoreHistogram {
	return &ScoreHistogram{counts: make([]atomic.Uint64, len(scoreBucketBounds)+1)}
}

// Observe records one score.
func (h *ScoreHistogram) Observe(score float64) {
	i := sort.SearchFloat64s(scoreBucketBounds, score)
	if i >= len(h.counts) {
		i = len(h.counts) - 1 // the +Inf bucket
	}
	h.counts[i].Add(1)
	h.total.Add(1)
	for {
		old := h.sumBits.Load()
		nw := math.Float64bits(math.Float64frombits(old) + score)
		if h.sumBits.CompareAndSwap(old, nw) {
			break
		}
	}
}

// ScoreSnapshot is a point-in-time view of the histogram, with cumulative
// counts (Prometheus histogram semantics: bucket le=b counts all scores <= b).
type ScoreSnapshot struct {
	Bounds           []float64
	CumulativeCounts []uint64
	Sum              float64
	Count            uint64
}

// Snapshot returns the cumulative histogram.
func (h *ScoreHistogram) Snapshot() ScoreSnapshot {
	cum := make([]uint64, len(h.counts))
	var running uint64
	for i := range h.counts {
		running += h.counts[i].Load()
		cum[i] = running
	}
	return ScoreSnapshot{
		Bounds:           append([]float64(nil), scoreBucketBounds...),
		CumulativeCounts: cum, // len == len(Bounds)+1; last entry is the +Inf total
		Sum:              math.Float64frombits(h.sumBits.Load()),
		Count:            h.total.Load(),
	}
}

// ScoreSnapshot returns the engine's agentic-score histogram.
func (e *Engine) ScoreSnapshot() ScoreSnapshot { return e.hist.Snapshot() }
