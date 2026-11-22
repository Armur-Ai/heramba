package watch

import (
	"testing"
	"time"
)

func base() time.Time { return time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC) }

func TestTimeSeriesCountsWithinBucket(t *testing.T) {
	ts := newTimeSeries(10, time.Second)
	t0 := base()
	ts.add(t0, 1)
	ts.add(t0.Add(200*time.Millisecond), 1)
	ts.add(t0.Add(900*time.Millisecond), 1)
	if got := ts.rate(); got != 3 {
		t.Fatalf("same-bucket events should sum: rate=%v want 3", got)
	}
	if len(ts.series()) != 1 {
		t.Fatalf("expected a single bucket, got %d", len(ts.series()))
	}
}

func TestTimeSeriesAdvancesBuckets(t *testing.T) {
	ts := newTimeSeries(10, time.Second)
	t0 := base()
	ts.add(t0, 5)
	ts.add(t0.Add(3*time.Second), 2) // 3s later → gap buckets of 0
	s := ts.series()
	if len(s) != 4 {
		t.Fatalf("expected 4 buckets (t, t+1, t+2, t+3), got %d: %v", len(s), s)
	}
	if s[0] != 5 || s[1] != 0 || s[2] != 0 || s[3] != 2 {
		t.Fatalf("bucket contents wrong: %v", s)
	}
	if ts.rate() != 2 {
		t.Errorf("rate should be the newest bucket, got %v", ts.rate())
	}
}

func TestTimeSeriesTrimsToWidth(t *testing.T) {
	ts := newTimeSeries(3, time.Second)
	t0 := base()
	for i := 0; i < 10; i++ {
		ts.add(t0.Add(time.Duration(i)*time.Second), float64(i))
	}
	s := ts.series()
	if len(s) != 3 {
		t.Fatalf("series should be capped at width 3, got %d", len(s))
	}
	// Last three buckets carry counts 7,8,9.
	if s[0] != 7 || s[1] != 8 || s[2] != 9 {
		t.Errorf("trim kept wrong buckets: %v", s)
	}
}

func TestTimeSeriesTickAdvancesQuietStream(t *testing.T) {
	ts := newTimeSeries(5, time.Second)
	t0 := base()
	ts.add(t0, 4)
	ts.tick(t0.Add(2 * time.Second))
	s := ts.series()
	if len(s) != 3 || s[2] != 0 {
		t.Fatalf("tick should append trailing zero buckets: %v", s)
	}
	if ts.rate() != 0 {
		t.Errorf("rate after a quiet tick should be 0, got %v", ts.rate())
	}
}

func TestTimeSeriesPeak(t *testing.T) {
	ts := newTimeSeries(10, time.Second)
	t0 := base()
	ts.add(t0, 2)
	ts.add(t0.Add(time.Second), 9)
	ts.add(t0.Add(2*time.Second), 4)
	if ts.peak() != 9 {
		t.Errorf("peak = %v, want 9", ts.peak())
	}
}

func TestTimeSeriesTickBeforeAnyEventIsNoop(t *testing.T) {
	ts := newTimeSeries(5, time.Second)
	ts.tick(base())
	if len(ts.series()) != 0 {
		t.Error("tick before the first event should not create buckets")
	}
}
