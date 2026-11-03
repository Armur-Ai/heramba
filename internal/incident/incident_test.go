package incident

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/Armur-Ai/heramba/internal/event"
	"github.com/Armur-Ai/heramba/internal/evidence"
	"github.com/Armur-Ai/heramba/internal/strategy"
)

func sampleEvent(id, src, target string, at time.Time) evidence.SecurityEvent {
	ev := evidence.FromDetection(event.Deception(event.KindDecoyHit, src, target, "decoy probe"), "edge", "raw-"+id)
	ev.ID = evidence.ID(id)
	ev.Provenance.ObservedAt = at
	return ev
}

func enforcedAction(id string, at time.Time) strategy.ActionExecution {
	a := strategy.Action{
		ID:        evidence.ID(id),
		Type:      strategy.ActionRateLimit,
		Scope:     strategy.Scope{SourceKey: "203.0.113.5"},
		Impact:    strategy.ImpactLow,
		Duration:  10 * time.Minute,
		CreatedAt: at,
		Evidence:  []evidence.EvidenceRef{{ID: evidence.ID("ev-1")}},
	}
	return strategy.ActionExecution{
		Action:        a,
		Status:        strategy.StatusEnforced,
		Decision:      strategy.Decision{Verdict: strategy.VerdictAllow},
		EnforcedAt:    at.Add(time.Second),
		RollbackToken: "rbk-123",
	}
}

func TestTimelineLinksAllStagesInOrder(t *testing.T) {
	t0 := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	ev := sampleEvent("ev-1", "203.0.113.5", "/.env", t0)
	f := evidence.Fact{ID: "fact-1", Statement: "203.0.113.5 requested decoy /.env", CreatedAt: t0.Add(time.Second), DerivedFrom: []evidence.ID{"ev-1"}}
	h := evidence.NewHypothesis("203.0.113.5 is enumerating", 0.7, time.Hour)
	h.ID = "hyp-1"
	h.CreatedAt = t0.Add(2 * time.Second)
	h.Contradicting = []evidence.EvidenceRef{{ID: "ev-x"}}
	ax := enforcedAction("act-1", t0.Add(3*time.Second))

	inc := New("inc-1", "decoy enumeration", RetentionPolicy{MaxAge: 90 * 24 * time.Hour}).
		Observe(ev, "ok").
		Fact(f, "ev-1").
		Hypothesis(h, "fact-1").
		Action(ax, "hyp-1").
		Build()

	stages := []Stage{}
	for _, e := range inc.Entries {
		stages = append(stages, e.Stage)
	}
	// Observation, fact, hypothesis, then proposed/approved/outcome for the action.
	wantFirst := []Stage{StageObservation, StageFact, StageHypothesis, StageProposed}
	for i, s := range wantFirst {
		if inc.Entries[i].Stage != s {
			t.Fatalf("entry %d stage = %s, want %s (stages: %v)", i, inc.Entries[i].Stage, s, stages)
		}
	}
	// The hypothesis must show its contradicting-evidence count and uncertainty.
	var sawHyp bool
	for _, e := range inc.Entries {
		if e.Stage == StageHypothesis {
			sawHyp = true
			if e.Observed {
				t.Error("hypothesis must be marked inferred, not observed")
			}
			if e.Contradicts != 1 {
				t.Errorf("hypothesis should show 1 contradicting ref, got %d", e.Contradicts)
			}
		}
	}
	if !sawHyp {
		t.Fatal("no hypothesis entry")
	}
}

func TestHumanControlGates(t *testing.T) {
	t0 := time.Now().UTC()
	ax := enforcedAction("act-1", t0)
	inc := New("inc", "t", RetentionPolicy{}).Action(ax, "ev-1").Build()
	if !inc.AllConsequentialReversible() {
		t.Error("enforced action with a rollback token must be reversible")
	}
	if !inc.AllConsequentialHaveEvidence() {
		t.Error("rate_limit action cites evidence; gate should pass")
	}

	// An enforced action WITHOUT a rollback token must fail the reversibility gate,
	// and with no evidence it must fail the evidence gate.
	bad := enforcedAction("act-2", t0)
	bad.RollbackToken = ""
	bad.Action.Evidence = nil
	incBad := New("inc", "t", RetentionPolicy{}).Action(bad).Build()
	if incBad.AllConsequentialReversible() {
		t.Error("enforced action without a rollback token must NOT be reversible")
	}
	// And it cites no evidence (we passed none) -> evidence gate fails.
	if incBad.AllConsequentialHaveEvidence() {
		t.Error("action without evidence must fail the evidence gate")
	}
}

func TestExplainReturnsCausalChain(t *testing.T) {
	t0 := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	h := evidence.NewHypothesis("enumerating", 0.7, time.Hour)
	h.ID = "hyp-1"
	h.CreatedAt = t0
	ax := enforcedAction("act-1", t0.Add(time.Second))
	inc := New("inc", "t", RetentionPolicy{}).Hypothesis(h).Action(ax, "hyp-1").Build()

	chain := inc.Explain("act-1")
	var ids []string
	for _, e := range chain {
		ids = append(ids, e.ID)
	}
	// The chain must include the action AND the hypothesis it derived from.
	if !contains(ids, "act-1") || !contains(ids, "hyp-1") {
		t.Fatalf("Explain chain missing linked evidence: %v", ids)
	}
}

func TestBlastRadiusAndImpact(t *testing.T) {
	t0 := time.Now().UTC()
	ax := enforcedAction("act-1", t0)
	ax.Action.Impact = strategy.ImpactHigh
	inc := New("inc", "t", RetentionPolicy{}).Action(ax, "ev-1").Build()
	a := inc.Actions[0]
	if a.Blast.Subjects != 1 || a.Blast.ScopeKind != "source_key" {
		t.Fatalf("blast radius should be bounded to one source_key: %+v", a.Blast)
	}
	if !a.Blast.CanAffectLegit {
		t.Error("high-impact action should be flagged as able to affect legit users")
	}
}

func TestRetentionPrune(t *testing.T) {
	now := time.Now().UTC()
	old := sampleEvent("ev-old", "x", "/a", now.Add(-100*24*time.Hour))
	recent := sampleEvent("ev-new", "x", "/b", now.Add(-1*time.Hour))
	inc := New("inc", "t", RetentionPolicy{MaxAge: 30 * 24 * time.Hour}).
		Observe(old, "ok").Observe(recent, "ok").Build()
	pruned := inc.Prune(now)
	if len(pruned.Entries) != 1 || pruned.Entries[0].ID != "ev-new" {
		t.Fatalf("retention should drop the old entry, kept: %+v", pruned.Entries)
	}
}

func TestReportRenders(t *testing.T) {
	t0 := time.Now().UTC()
	inc := New("inc-1", "t", RetentionPolicy{MaxAge: time.Hour}).
		Observe(sampleEvent("ev-1", "x", "/.env", t0), "ok").
		Action(enforcedAction("act-1", t0), "ev-1").Build()
	var md, js bytes.Buffer
	if err := inc.WriteMarkdown(&md); err != nil {
		t.Fatal(err)
	}
	if err := inc.WriteJSON(&js); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(md.String(), "Investigation timeline") || !strings.Contains(md.String(), "reversible") {
		t.Fatal("markdown report missing expected sections")
	}
}

func contains(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}
