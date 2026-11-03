package experiment

import (
	"math"
	"testing"
)

func agentVsHuman() *HypothesisSet {
	h, _ := NewHypothesisSet(map[string]float64{"agent": 0.5, "human": 0.5})
	return h
}

// a decoy link only an automated crawler tends to follow
func crawlerLinkExperiment() Experiment {
	return Experiment{
		ID:   "hidden-link",
		Desc: "inject an invisible link only a crawler would fetch",
		Likelihood: map[string]map[string]float64{
			"agent": {"followed": 0.85, "ignored": 0.15},
			"human": {"followed": 0.05, "ignored": 0.95},
		},
	}
}

func TestEntropyAndNormalization(t *testing.T) {
	h := agentVsHuman()
	if math.Abs(h.Entropy()-1.0) > 1e-9 {
		t.Fatalf("50/50 entropy should be 1 bit, got %v", h.Entropy())
	}
	if math.Abs(h.Prob("agent")+h.Prob("human")-1.0) > 1e-9 {
		t.Fatal("distribution not normalized")
	}
}

func TestBayesUpdateShiftsBelief(t *testing.T) {
	h := agentVsHuman()
	r := NewRegistry(h, NewBudget(3))
	exp := crawlerLinkExperiment()

	if err := r.Run(&exp, "followed"); err != nil {
		t.Fatalf("run: %v", err)
	}
	// posterior P(agent) = .5*.85 / (.5*.85 + .5*.05) = .425/.45 ≈ 0.944
	if p := h.Prob("agent"); math.Abs(p-0.9444) > 0.01 {
		t.Fatalf("posterior agent prob wrong: %v", p)
	}
	id, _ := h.MostLikely()
	if id != "agent" {
		t.Fatalf("most likely should be agent, got %s", id)
	}
	// entropy dropped (we learned something)
	if h.Entropy() >= 1.0 {
		t.Fatalf("entropy should have dropped below 1 bit, got %v", h.Entropy())
	}
}

func TestExpectedInfoGainPositiveAndRanked(t *testing.T) {
	h := agentVsHuman()
	informative := crawlerLinkExperiment()
	// A useless experiment: same likelihood under both explanations → no info.
	useless := Experiment{
		ID: "useless",
		Likelihood: map[string]map[string]float64{
			"agent": {"a": 0.5, "b": 0.5},
			"human": {"a": 0.5, "b": 0.5},
		},
	}
	gi := informative.ExpectedInfoGain(h)
	gu := useless.ExpectedInfoGain(h)
	if gi <= gu {
		t.Fatalf("informative experiment should out-gain useless one: %v vs %v", gi, gu)
	}
	if gu > 1e-9 {
		t.Fatalf("useless experiment should have ~0 info gain, got %v", gu)
	}
}

func TestPickChoosesHighestGain(t *testing.T) {
	h := agentVsHuman()
	r := NewRegistry(h, NewBudget(2))
	useless := Experiment{ID: "useless", Likelihood: map[string]map[string]float64{
		"agent": {"a": 0.5, "b": 0.5}, "human": {"a": 0.5, "b": 0.5}}}
	best, ok := r.Pick([]Experiment{useless, crawlerLinkExperiment()})
	if !ok || best.ID != "hidden-link" {
		t.Fatalf("Pick should choose the informative experiment, got %v %v", ok, best)
	}
}

func TestBudgetIsEnforced(t *testing.T) {
	h := agentVsHuman()
	r := NewRegistry(h, NewBudget(1))
	exp := crawlerLinkExperiment()
	if err := r.Run(&exp, "followed"); err != nil {
		t.Fatalf("first run: %v", err)
	}
	if r.Budget.Remaining() != 0 {
		t.Fatalf("budget should be spent, remaining=%d", r.Budget.Remaining())
	}
	// Pick must refuse when budget is exhausted.
	if _, ok := r.Pick([]Experiment{crawlerLinkExperiment()}); ok {
		t.Fatal("Pick returned an experiment with no budget left")
	}
	// Run must error when budget is exhausted.
	if err := r.Run(&exp, "followed"); err == nil {
		t.Fatal("Run should error with no budget left")
	}
	if len(r.History()) != 1 {
		t.Fatalf("history should record exactly one run, got %d", len(r.History()))
	}
}

func TestUncertaintyReportedNotHidden(t *testing.T) {
	// Even after a weakly-informative outcome, uncertainty must remain visible
	// (entropy > 0) rather than collapsing to false certainty.
	h, _ := NewHypothesisSet(map[string]float64{"agent": 0.5, "human": 0.5})
	r := NewRegistry(h, NewBudget(5))
	weak := Experiment{ID: "weak", Likelihood: map[string]map[string]float64{
		"agent": {"yes": 0.55, "no": 0.45}, "human": {"yes": 0.45, "no": 0.55}}}
	_ = r.Run(&weak, "yes")
	if h.Entropy() <= 0 {
		t.Fatal("a weak experiment must not collapse uncertainty to zero")
	}
	if _, p := h.MostLikely(); p >= 0.99 {
		t.Fatalf("weak evidence should not produce near-certainty, got %v", p)
	}
}
