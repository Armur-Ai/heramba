package lab08

import (
	"testing"

	"github.com/Armur-Ai/heramba/internal/research/coevolve"
	"github.com/Armur-Ai/heramba/labs/labkit"
)

// Lab 08 — Adaptation race & ablation study.
//
// Objective: measure the defender's adaptation latency against a bounded
// attacker strategy shift, decompose it into detection and response components,
// and run the no-adaptation / adaptive / instant ablation to see where adaptation
// helps — and, against a feinting adversary, where it hurts.
func TestLab08(t *testing.T) {
	labkit.Banner(t, "Lab 08 — Adaptation race & ablation study")
	seeds := make([]int64, 60)
	for i := range seeds {
		seeds[i] = int64(100 + i)
	}
	rep := coevolve.Run(6, seeds, coevolve.DefaultParams(), "lab08")
	by := map[string]coevolve.ArchetypeResult{}
	for _, r := range rep.Results {
		by[r.Archetype+"/"+r.Mode] = r
	}

	labkit.Step(t, 1, "adaptive defender counters a detectable shift with positive utility")
	co := by["coordinated/adaptive"]
	labkit.Require(t, co.CounteredRate.Point >= 0.9, "a detectable shift should be countered")
	labkit.Require(t, co.MeanUtility.Point > 0, "net defender utility should be positive")
	labkit.Observe(t, "coordinated: countered=%.2f utility=%.2f latency=%.1fs", co.CounteredRate.Point, co.MeanUtility.Point, co.AdaptationLatency.Point)

	labkit.Step(t, 2, "adaptation latency decomposes into detection + response")
	sum := co.DetectionComponent.Point + co.ResponseComponent.Point
	labkit.Require(t, abs(sum-co.AdaptationLatency.Point) < 0.5, "latency should equal detection + response")
	labkit.Observe(t, "detection=%.1fs response=%.1fs (response is the fixed rollout cost)", co.DetectionComponent.Point, co.ResponseComponent.Point)

	labkit.Step(t, 3, "a stealthier adversary costs more detection latency (graceful, not catastrophic)")
	aware := by["deception_aware/adaptive"]
	labkit.Require(t, aware.DetectionComponent.Point > co.DetectionComponent.Point, "lower detectability should raise detection latency")
	labkit.Observe(t, "deception_aware detection=%.1fs vs coordinated %.1fs", aware.DetectionComponent.Point, co.DetectionComponent.Point)

	labkit.Step(t, 4, "ablation: no-adaptation counters nothing; a feinting adversary induces oscillation")
	none := by["coordinated/no_adaptation"]
	labkit.Require(t, none.MeanUtility.Point < co.MeanUtility.Point, "no-adaptation utility should be lower")
	feint := by["adversarial_varied/adaptive"]
	labkit.Require(t, feint.OscillationRate.Point > 0, "a feinting adversary should induce policy oscillation")
	labkit.Observe(t, "no_adaptation utility=%.2f; feinting adversary oscillation=%.2f utility=%.2f",
		none.MeanUtility.Point, feint.OscillationRate.Point, feint.MeanUtility.Point)
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
