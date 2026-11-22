package watch

import (
	"testing"
	"time"

	"github.com/Armur-Ai/heramba/internal/event"
)

func TestStateRecordsEventRate(t *testing.T) {
	s := NewState(50)
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		e := event.Deception(event.KindDecoyHit, "1.1.1.1", "/.env", "d")
		e.Time = t0
		s.Add(e)
	}
	if s.RateNow() != 3 {
		t.Fatalf("RateNow = %v, want 3", s.RateNow())
	}
	if len(s.RateSeries()) == 0 {
		t.Fatal("RateSeries should be non-empty after events")
	}
	if s.RatePeak() < 3 {
		t.Errorf("RatePeak = %v, want >= 3", s.RatePeak())
	}
}

func TestStateTickAdvancesRate(t *testing.T) {
	s := NewState(50)
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	e := event.Deception(event.KindDecoyHit, "1.1.1.1", "/.env", "d")
	e.Time = t0
	s.Add(e)
	s.Tick(t0.Add(2 * time.Second))
	if s.RateNow() != 0 {
		t.Errorf("after a quiet tick RateNow should be 0, got %v", s.RateNow())
	}
}

func TestClearResetsRate(t *testing.T) {
	// A cleared State (new rate series) starts empty.
	s := NewState(50)
	e := event.Deception(event.KindDecoyHit, "1.1.1.1", "/.env", "d")
	s.Add(e)
	s2 := NewState(s.maxRecent)
	if len(s2.RateSeries()) != 0 {
		t.Error("a fresh State should have an empty rate series")
	}
}
