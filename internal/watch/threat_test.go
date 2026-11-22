package watch

import (
	"testing"
	"time"

	"github.com/Armur-Ai/heramba/internal/event"
)

func TestThreatScoreEmptyIsZero(t *testing.T) {
	s := NewState(50)
	if s.ThreatScore() != 0 || s.ThreatLevel() != ThreatCalm {
		t.Fatalf("empty state should be calm/0, got %v/%v", s.ThreatScore(), s.ThreatLevel())
	}
}

func TestThreatCanaryFloorsCritical(t *testing.T) {
	s := NewState(50)
	s.Add(event.Deception(event.KindCanaryTriggered, "1.1.1.1", "tok", "used"))
	if got := s.ThreatScore(); got < 0.92 {
		t.Fatalf("a canary hit must floor the score high, got %v", got)
	}
	if s.ThreatLevel() != ThreatCritical {
		t.Errorf("canary hit should be CRITICAL, got %v", s.ThreatLevel())
	}
}

func TestThreatSwarmFloorsHigh(t *testing.T) {
	s := NewState(50)
	for _, ip := range []string{"1.1.1.1", "2.2.2.2"} {
		e := event.Deception(event.KindDecoyHit, ip, "/.env", "d").WithLabel("campaign", "sweep")
		s.Add(e)
	}
	if s.SwarmCount() == 0 {
		t.Fatal("precondition: expected a swarm")
	}
	if got := s.ThreatScore(); got < 0.80 {
		t.Errorf("an active swarm should floor the score high, got %v", got)
	}
}

func TestThreatLevelBands(t *testing.T) {
	cases := []struct {
		score float64
		want  ThreatLevel
	}{
		{0.0, ThreatCalm},
		{0.14, ThreatCalm},
		{0.2, ThreatGuarded},
		{0.5, ThreatElevated},
		{0.7, ThreatHigh},
		{0.95, ThreatCritical},
	}
	for _, c := range cases {
		if got := threatLevelOf(c.score); got != c.want {
			t.Errorf("threatLevelOf(%v) = %v, want %v", c.score, got, c.want)
		}
	}
}

func TestThreatScoreRisesWithSeverity(t *testing.T) {
	low := NewState(50)
	low.Add(event.New(event.KindProbe, event.SeverityLow, "1.1.1.1", "/x", "d"))

	high := NewState(50)
	e := event.New(event.KindProbe, event.SeverityCritical, "1.1.1.1", "/x", "d")
	e.Time = time.Now()
	high.Add(e)

	if !(high.ThreatScore() > low.ThreatScore()) {
		t.Errorf("critical severity should score above low: %v vs %v", high.ThreatScore(), low.ThreatScore())
	}
}

func TestThreatLevelColorsDistinct(t *testing.T) {
	seen := map[string]bool{}
	for _, l := range []ThreatLevel{ThreatCalm, ThreatGuarded, ThreatElevated, ThreatHigh, ThreatCritical} {
		c := string(l.Color())
		if seen[c] {
			t.Errorf("threat level %v reuses color %s", l, c)
		}
		seen[c] = true
		if l.String() == "" {
			t.Errorf("level %d has empty label", l)
		}
	}
}
