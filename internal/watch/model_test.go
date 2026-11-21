package watch

import (
	"testing"
	"time"

	"github.com/Armur-Ai/heramba/internal/event"
)

func TestStateFoldsTalliesAndRecent(t *testing.T) {
	s := NewState(3)
	s.Add(event.Deception(event.KindDecoyHit, "a", "/.env", "d"))
	s.Add(event.Deception(event.KindCanaryTriggered, "a", "tok", "d"))
	s.Add(event.New(event.KindAgenticTraffic, event.SeverityMedium, "a", "/x", "d"))
	s.Add(event.New(event.KindEgressBlocked, event.SeverityCritical, "a", "evil.test", "d"))

	if s.Total != 4 {
		t.Fatalf("Total=%d want 4", s.Total)
	}
	if s.DecoyHits() != 1 || s.CanaryHits() != 1 || s.AgenticFlags() != 1 || s.EgressBlocks() != 1 {
		t.Errorf("headline counts wrong: %+v", s.ByKind)
	}
	if s.HighestSeverity != event.SeverityCritical {
		t.Errorf("HighestSeverity=%v want critical", s.HighestSeverity)
	}
	// Recent is capped at 3 and newest-first.
	r := s.Recent(10)
	if len(r) != 3 {
		t.Fatalf("recent len=%d want 3 (capped)", len(r))
	}
	if r[0].Kind != event.KindEgressBlocked {
		t.Errorf("recent[0]=%v want egress_blocked (newest first)", r[0].Kind)
	}
}

func TestSourceAggregation(t *testing.T) {
	s := NewState(50)
	s.Add(event.Deception(event.KindDecoyHit, "10.0.0.1", "/.env", "d"))
	s.Add(event.Deception(event.KindDecoyHit, "10.0.0.1", "/admin", "d"))
	s.Add(event.New(event.KindProbe, event.SeverityLow, "10.0.0.2", "/", "d"))

	if s.SourceCount() != 2 {
		t.Fatalf("SourceCount=%d want 2", s.SourceCount())
	}
	top := s.TopSources(10)
	// 10.0.0.1 has higher severity (High from deception) → ranked first.
	if top[0].Addr != "10.0.0.1" || top[0].Decoys != 2 {
		t.Errorf("top source wrong: %+v", top[0])
	}
	if top[0].MaxSeverity != event.SeverityHigh {
		t.Errorf("source max severity=%v want high", top[0].MaxSeverity)
	}
}

func TestSwarmDetectionByCampaignLabel(t *testing.T) {
	s := NewState(50)
	// Three distinct sources, same campaign label → one coordinated swarm.
	for _, src := range []string{"1.1.1.1", "2.2.2.2", "3.3.3.3"} {
		e := event.Deception(event.KindDecoyHit, src, "/.env", "d").WithLabel("campaign", "artex-sweep")
		s.Add(e)
	}
	if s.SwarmCount() != 1 {
		t.Fatalf("SwarmCount=%d want 1", s.SwarmCount())
	}
	camps := s.Campaigns(10)
	if len(camps) == 0 || !camps[0].Swarm() {
		t.Fatalf("expected a swarm campaign, got %+v", camps)
	}
	if len(camps[0].Sources) != 3 || camps[0].ID != "artex-sweep" {
		t.Errorf("campaign wrong: %+v", camps[0])
	}
}

func TestLoneSourceIsNotSwarm(t *testing.T) {
	s := NewState(50)
	s.Add(event.Deception(event.KindDecoyHit, "9.9.9.9", "/.env", "d"))
	if s.SwarmCount() != 0 {
		t.Errorf("single source must not be a swarm")
	}
	if s.CampaignCount() != 1 {
		t.Errorf("CampaignCount=%d want 1 (per-source campaign)", s.CampaignCount())
	}
}

func TestTopDecoysRanked(t *testing.T) {
	s := NewState(50)
	for i := 0; i < 3; i++ {
		s.Add(event.Deception(event.KindDecoyHit, "x", "/.env", "d"))
	}
	s.Add(event.Deception(event.KindCanaryTriggered, "x", "tok-1", "d"))
	d := s.TopDecoys(10)
	if len(d) != 2 || d[0].Target != "/.env" || d[0].Hits != 3 {
		t.Fatalf("top decoy wrong: %+v", d)
	}
}

func TestScoreFromLabelBeatsConfidence(t *testing.T) {
	s := NewState(10)
	e := event.New(event.KindAgenticTraffic, event.SeverityMedium, "z", "/x", "d").WithLabel("agentic_score", "0.97")
	s.Add(e)
	if got := s.TopSources(1)[0].MaxScore; got != 0.97 {
		t.Errorf("MaxScore=%v want 0.97 (from label)", got)
	}
}

func TestAddIgnoresEmptySource(t *testing.T) {
	s := NewState(10)
	e := event.New(event.KindProbe, event.SeverityInfo, "", "/x", "d")
	e.Time = time.Now()
	s.Add(e)
	if s.SourceCount() != 0 || s.CampaignCount() != 0 {
		t.Errorf("empty source must not create entities")
	}
	if s.Total != 1 {
		t.Errorf("event should still count in totals")
	}
}
