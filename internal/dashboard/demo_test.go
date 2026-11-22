package dashboard

import (
	"testing"
	"time"
)

func TestDemoFeedProducesEvents(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	d := NewDemoFeedAt(1, now)
	d.Seed(300)

	ev := d.Recent(50)
	if len(ev) != 50 {
		t.Fatalf("Recent(50) = %d events, want 50", len(ev))
	}
	// Newest first.
	for i := 1; i < len(ev); i++ {
		if ev[i-1].Time.Before(ev[i].Time) {
			t.Fatalf("events not newest-first at %d", i)
		}
	}
	m := d.Metrics()
	if m.Total < 300 {
		t.Errorf("metrics total = %d, want >= 300", m.Total)
	}
	if len(m.ByKind) == 0 || len(m.BySeverity) == 0 {
		t.Error("metrics missing by-kind / by-severity breakdowns")
	}
	if m.Sources == 0 {
		t.Error("metrics reports zero sources")
	}
}

func TestDemoFeedDeterministic(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	a := NewDemoFeedAt(42, now)
	b := NewDemoFeedAt(42, now)
	a.Seed(100)
	b.Seed(100)
	ea, eb := a.Recent(100), b.Recent(100)
	if len(ea) != len(eb) {
		t.Fatalf("lengths differ: %d vs %d", len(ea), len(eb))
	}
	for i := range ea {
		if ea[i].Kind != eb[i].Kind || ea[i].Source != eb[i].Source || !ea[i].Time.Equal(eb[i].Time) {
			t.Fatalf("event %d differs between identically-seeded feeds", i)
		}
	}
}

func TestDemoFeedSwarmDetected(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	d := NewDemoFeedAt(7, now)
	d.Seed(500)
	camps := detectCampaigns(d.Recent(1500))
	coordinated := false
	for _, c := range camps {
		if c.Coordinated {
			coordinated = true
			break
		}
	}
	if !coordinated {
		t.Error("demo feed produced no coordinated swarm campaign")
	}
}

func TestDemoFeedScoreHistogram(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	d := NewDemoFeedAt(3, now)
	d.Seed(400)
	m := d.Metrics()
	if m.Score.Count == 0 {
		t.Fatal("no agentic-score samples in histogram")
	}
	if len(m.Score.CumCounts) != len(m.Score.Bounds)+1 {
		t.Errorf("cum counts len = %d, want bounds+1 = %d", len(m.Score.CumCounts), len(m.Score.Bounds)+1)
	}
	// Cumulative counts are non-decreasing.
	for i := 1; i < len(m.Score.CumCounts); i++ {
		if m.Score.CumCounts[i] < m.Score.CumCounts[i-1] {
			t.Errorf("cum counts not non-decreasing at %d", i)
		}
	}
}

func TestDemoFeedMetricsPayloadNeverNilMaps(t *testing.T) {
	d := NewDemoFeedAt(5, time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC))
	p := d.metricsPayload() // empty feed
	if p.ByKind == nil || p.BySeverity == nil {
		t.Fatal("metricsPayload maps must never be nil")
	}
	d.Seed(50)
	p = d.metricsPayload()
	if p.Total < 50 {
		t.Errorf("payload total = %d, want >= 50", p.Total)
	}
}

func TestDemoFeedTickAdvances(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	d := NewDemoFeedAt(9, now)
	before := len(d.Recent(2000))
	gen := d.Tick(time.Second)
	if len(gen) == 0 {
		t.Fatal("Tick produced no events")
	}
	after := len(d.Recent(2000))
	if after != before+len(gen) {
		t.Errorf("after Tick len = %d, want %d", after, before+len(gen))
	}
}
