package coevolve

import (
	"bytes"
	"testing"
)

// Event sourcing must replay deterministically from a seed.
func TestReplayDeterministic(t *testing.T) {
	arch := Archetypes()[2] // coordinated
	a := &Sim{Archetype: arch, Mode: Adaptive, Params: DefaultParams()}
	b := &Sim{Archetype: arch, Mode: Adaptive, Params: DefaultParams()}
	ra := a.Run(5, 42)
	rb := b.Run(5, 42)
	if len(a.Log) != len(b.Log) {
		t.Fatalf("event log length differs: %d vs %d", len(a.Log), len(b.Log))
	}
	for i := range a.Log {
		if a.Log[i] != b.Log[i] {
			t.Fatalf("event %d differs: %+v vs %+v", i, a.Log[i], b.Log[i])
		}
	}
	for i := range ra {
		if ra[i] != rb[i] {
			t.Fatalf("round %d differs across replay", i)
		}
	}
}

// The event log must contain the full pipeline for a countered round, in order.
func TestEventLogHasFullPipeline(t *testing.T) {
	s := &Sim{Archetype: Archetypes()[0], Mode: Adaptive, Params: DefaultParams()} // scripted, always countered
	s.Run(1, 1)
	want := []Phase{PhaseChange, PhaseObserved, PhaseDetected, PhaseDecision, PhaseApplied, PhaseEffective}
	got := map[Phase]Tick{}
	for _, e := range s.Log {
		if _, seen := got[e.Phase]; !seen {
			got[e.Phase] = e.Tick
		}
	}
	var last Tick = -1
	for _, ph := range want {
		tk, ok := got[ph]
		if !ok {
			t.Fatalf("missing phase %s in log", ph)
		}
		if tk < last {
			t.Fatalf("phase %s at tick %d out of order (prev %d)", ph, tk, last)
		}
		last = tk
	}
}

// H3: against a detectable adversary the adaptive defender counters the shift
// and earns clearly positive net utility, strictly beating no-adaptation.
func TestH3AdaptiveBeatsNoAdaptation(t *testing.T) {
	rep := Run(6, seeds(80), DefaultParams(), "test")
	byKey := map[string]ArchetypeResult{}
	for _, res := range rep.Results {
		byKey[res.Archetype+"/"+res.Mode] = res
	}
	for _, arch := range []string{"scripted", "independent", "coordinated"} {
		adaptive := byKey[arch+"/adaptive"]
		none := byKey[arch+"/no_adaptation"]
		if adaptive.MeanUtility.Point <= none.MeanUtility.Point {
			t.Errorf("%s: adaptive utility %.3f should beat no-adaptation %.3f",
				arch, adaptive.MeanUtility.Point, none.MeanUtility.Point)
		}
		if adaptive.MeanUtility.Point <= 0 {
			t.Errorf("%s: adaptive utility should be positive, got %.3f", arch, adaptive.MeanUtility.Point)
		}
		if adaptive.CounteredRate.Point < 0.9 {
			t.Errorf("%s: adaptive should counter a detectable shift, got %.3f", arch, adaptive.CounteredRate.Point)
		}
	}
}

// Adaptation latency decomposes correctly and is higher for a less detectable
// adversary (the detection component grows).
func TestLatencyDecompositionAndOrdering(t *testing.T) {
	rep := Run(6, seeds(80), DefaultParams(), "test")
	byKey := map[string]ArchetypeResult{}
	for _, res := range rep.Results {
		byKey[res.Archetype+"/"+res.Mode] = res
	}
	scripted := byKey["scripted/adaptive"]
	aware := byKey["deception_aware/adaptive"]
	// latency = detection + response (within rounding/aggregation).
	sum := scripted.DetectionComponent.Point + scripted.ResponseComponent.Point
	if abs(sum-scripted.AdaptationLatency.Point) > 0.5 {
		t.Errorf("latency %.2f != detection %.2f + response %.2f", scripted.AdaptationLatency.Point,
			scripted.DetectionComponent.Point, scripted.ResponseComponent.Point)
	}
	// Harder-to-detect adversary -> larger detection component.
	if aware.DetectionComponent.Point <= scripted.DetectionComponent.Point {
		t.Errorf("deception-aware detection component %.2f should exceed scripted %.2f",
			aware.DetectionComponent.Point, scripted.DetectionComponent.Point)
	}
}

// Where adaptation hurts: the feinting adversarial-varied archetype induces
// policy oscillation, which the lab surfaces (honest negative result).
func TestAdversarialVariedShowsOscillation(t *testing.T) {
	rep := Run(8, seeds(80), DefaultParams(), "test")
	for _, res := range rep.Results {
		if res.Archetype == "adversarial_varied" && res.Mode == "adaptive" {
			if res.OscillationRate.Point <= 0 {
				t.Fatalf("expected the feinting adversary to induce oscillation, got %.3f", res.OscillationRate.Point)
			}
			return
		}
	}
	t.Fatal("adversarial_varied/adaptive result missing")
}

func TestReportRenders(t *testing.T) {
	rep := Run(4, seeds(10), DefaultParams(), "test")
	var md, js bytes.Buffer
	if err := rep.WriteMarkdown(&md); err != nil {
		t.Fatal(err)
	}
	if err := rep.WriteJSON(&js); err != nil {
		t.Fatal(err)
	}
	if md.Len() == 0 || js.Len() == 0 {
		t.Fatal("empty report")
	}
}

func seeds(n int) []int64 {
	out := make([]int64, n)
	for i := range out {
		out[i] = int64(100 + i)
	}
	return out
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
