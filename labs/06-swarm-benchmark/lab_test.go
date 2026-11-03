package lab06

import (
	"bytes"
	"testing"

	"github.com/Armur-Ai/heramba/internal/replay"
	"github.com/Armur-Ai/heramba/labs/labkit"
)

// Lab 06 — Swarm-versus-defense benchmarking.
//
// Objective: run the controlled four-baseline experiment yourself and read the
// measured comparison. This is the same path as `heramba experiment`.
func TestLab06(t *testing.T) {
	labkit.Banner(t, "Lab 06 — Swarm-versus-defense benchmarking")

	labkit.Step(t, 1, "run all four baselines over identical seeded scenarios")
	seeds := make([]int64, 15)
	for i := range seeds {
		seeds[i] = int64(i + 1)
	}
	exp := replay.RunFourBaselineExperiment(seeds)
	labkit.Require(t, len(exp.Results) == 4, "expected four baselines")

	by := map[string]replay.BaselineResult{}
	for _, r := range exp.Results {
		by[r.Defense] = r
	}

	labkit.Step(t, 2, "read the measured outcomes")
	labkit.Require(t, by["no_defense"].ObjectiveCompletion.Mean > 0.99,
		"with no defense the attacker should complete the objective")
	labkit.Require(t, by["fixed_deception"].FPR.Mean == 0 && by["adaptive_deception"].FPR.Mean == 0,
		"deception baselines must have zero false positives")
	labkit.Observe(t, "no_defense objective completion: %.2f", by["no_defense"].ObjectiveCompletion.Mean)
	labkit.Observe(t, "adaptive detect latency: %.1fs vs fixed %.1fs",
		by["adaptive_deception"].DetectionLatencySec.Mean, by["fixed_deception"].DetectionLatencySec.Mean)

	labkit.Step(t, 3, "the experiment is reproducible")
	var a, b bytes.Buffer
	_ = exp.WriteJSON(&a)
	_ = replay.RunFourBaselineExperiment(seeds).WriteJSON(&b)
	labkit.Require(t, a.String() == b.String(), "same seeds must reproduce the same report")

	var md bytes.Buffer
	_ = exp.WriteMarkdown(&md)
	t.Logf("\n%s", md.String())
}
