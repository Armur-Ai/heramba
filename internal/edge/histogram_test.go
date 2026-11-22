package edge

import (
	"testing"

	"github.com/Armur-Ai/heramba/internal/config"
)

func TestScoreHistogramBucketsAndSum(t *testing.T) {
	h := newScoreHistogram()
	// Scores land in buckets by upper bound: 0.05->le0.1, 0.25->le0.3, 0.95->le1.0, 1.5->+Inf.
	for _, s := range []float64{0.05, 0.25, 0.95, 1.5} {
		h.Observe(s)
	}
	snap := h.Snapshot()
	if snap.Count != 4 {
		t.Fatalf("count = %d, want 4", snap.Count)
	}
	if got := snap.Sum; got < 2.74 || got > 2.76 {
		t.Errorf("sum = %v, want ~2.75", got)
	}
	// Cumulative: le=0.1 should include the 0.05 observation (==1).
	if snap.CumulativeCounts[0] != 1 {
		t.Errorf("le<=0.1 cumulative = %d, want 1", snap.CumulativeCounts[0])
	}
	// The final +Inf bucket must equal the total count.
	last := snap.CumulativeCounts[len(snap.CumulativeCounts)-1]
	if last != 4 {
		t.Errorf("+Inf cumulative = %d, want 4", last)
	}
	// Cumulative counts must be non-decreasing.
	for i := 1; i < len(snap.CumulativeCounts); i++ {
		if snap.CumulativeCounts[i] < snap.CumulativeCounts[i-1] {
			t.Errorf("cumulative counts decreased at %d: %v", i, snap.CumulativeCounts)
		}
	}
}

func TestScoreHistogramEmpty(t *testing.T) {
	snap := newScoreHistogram().Snapshot()
	if snap.Count != 0 || snap.Sum != 0 {
		t.Errorf("empty histogram should be zero: %+v", snap)
	}
	if len(snap.CumulativeCounts) != len(snap.Bounds)+1 {
		t.Errorf("cumcounts must be bounds+1: %d vs %d", len(snap.CumulativeCounts), len(snap.Bounds))
	}
}

func TestServeHTTPFeedsScoreHistogram(t *testing.T) {
	e, err := New(configDefault(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		serveGet(e, "/some/path")
	}
	if got := e.ScoreSnapshot().Count; got != 3 {
		t.Errorf("histogram count = %d, want 3 after 3 requests", got)
	}
}

func configDefault() config.Config { return config.Default() }
