package main

import (
	"testing"

	"github.com/Armur-Ai/heramba/internal/event"
	"github.com/Armur-Ai/heramba/internal/strategy"
)

func TestShadowAdvisorProposesButNeverEnforces(t *testing.T) {
	a := newShadowAdvisor()
	// A source that trips a decoy (zero-FP) and looks agentic.
	_ = a.Emit(event.Deception(event.KindDecoyHit, "203.0.113.9", "/.env", "decoy probe"))
	_ = a.Emit(event.New(event.KindAgenticTraffic, event.SeverityMedium, "203.0.113.9", "/api", "agentic-suspected"))

	proposals := a.Propose()
	if len(proposals) == 0 {
		t.Fatal("expected the planner to propose actions for a decoy-tripping agentic source")
	}
	var sawRateLimit, sawDecoy bool
	for _, ex := range proposals {
		// SHADOW mode: nothing is ever enforced.
		if ex.Status == strategy.StatusEnforced {
			t.Fatalf("shadow mode must never enforce, got %+v", ex.Status)
		}
		switch ex.Action.Type {
		case strategy.ActionRateLimit:
			sawRateLimit = true
		case strategy.ActionActivateDecoy:
			sawDecoy = true
		}
	}
	if !sawRateLimit {
		t.Error("an agentic source (score 0.85) should yield a rate-limit proposal")
	}
	if !sawDecoy {
		t.Error("a decoy hit should yield an activate-decoy proposal")
	}
}

func TestShadowAdvisorIgnoresEmptySource(t *testing.T) {
	a := newShadowAdvisor()
	_ = a.Emit(event.New(event.KindAgenticTraffic, event.SeverityMedium, "", "/", "no source"))
	if len(a.Propose()) != 0 {
		t.Fatal("events with no source must not produce proposals")
	}
}
