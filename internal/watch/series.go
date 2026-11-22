package watch

import "time"

// timeSeries buckets event counts into fixed-width time slots (e.g. one per
// second) and keeps the most recent `width` of them, so the dashboard can draw
// an events/sec sparkline and a short-term rate. It is not safe for concurrent
// use; the Model folds events on a single goroutine.
type timeSeries struct {
	width  int           // number of buckets to retain
	size   time.Duration // width of one bucket
	counts []float64     // oldest..newest, len <= width
	cur    time.Time     // start instant of the newest bucket
}

// newTimeSeries returns a series of `width` buckets each `size` wide. Sensible
// defaults are applied for non-positive arguments.
func newTimeSeries(width int, size time.Duration) *timeSeries {
	if width <= 0 {
		width = 60
	}
	if size <= 0 {
		size = time.Second
	}
	return &timeSeries{width: width, size: size}
}

// add records n events at instant `at`, advancing (and trimming) buckets to
// cover any elapsed time since the last call. Events older than the current
// bucket are counted into the current bucket rather than dropped, which keeps a
// replayed stream — whose timestamps may not be monotonic to the wall clock —
// from losing samples.
func (t *timeSeries) add(at time.Time, n float64) {
	slot := at.Truncate(t.size)
	if t.cur.IsZero() {
		t.cur = slot
		t.counts = append(t.counts, 0)
	}
	// Advance forward in whole buckets until the newest bucket covers `at`.
	for slot.After(t.cur) {
		t.cur = t.cur.Add(t.size)
		t.counts = append(t.counts, 0)
		if len(t.counts) > t.width {
			t.counts = t.counts[len(t.counts)-t.width:]
		}
	}
	t.counts[len(t.counts)-1] += n
}

// tick advances the series to `now` without recording an event, so a quiet
// stream still scrolls (trailing zeros) instead of freezing on the last value.
func (t *timeSeries) tick(now time.Time) {
	if t.cur.IsZero() {
		return
	}
	t.add(now, 0)
}

// series returns the retained bucket counts, oldest first. The caller must not
// mutate the returned slice.
func (t *timeSeries) series() []float64 { return t.counts }

// rate returns the count in the most recent bucket (events in the last `size`
// window). Zero when no events have been recorded.
func (t *timeSeries) rate() float64 {
	if len(t.counts) == 0 {
		return 0
	}
	return t.counts[len(t.counts)-1]
}

// peak returns the largest single-bucket count retained, the natural y-scale
// for a rate readout.
func (t *timeSeries) peak() float64 {
	max := 0.0
	for _, c := range t.counts {
		if c > max {
			max = c
		}
	}
	return max
}
