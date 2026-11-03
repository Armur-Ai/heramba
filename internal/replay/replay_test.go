package replay

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestScenarioIsDeterministic(t *testing.T) {
	a := GenerateScenario(2026)
	b := GenerateScenario(2026)
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	if !bytes.Equal(ja, jb) {
		t.Fatal("same seed produced different scenarios — replay not reproducible")
	}
	if len(a.Actions) == 0 || len(a.AttackerSources) == 0 || len(a.BenignSources) == 0 {
		t.Fatal("scenario missing actions or labeled sources")
	}
	// actions are chronologically ordered
	for i := 1; i < len(a.Actions); i++ {
		if a.Actions[i].Offset < a.Actions[i-1].Offset {
			t.Fatal("scenario actions not chronologically ordered")
		}
	}
}

func TestNoDefenseLetsObjectiveComplete(t *testing.T) {
	s := GenerateScenario(1)
	m := RunScenario(s, NoDefense{})
	if !m.AttackerObjectiveDone {
		t.Fatal("with no defense the attacker should complete the objective")
	}
	if m.DetectedAttacker {
		t.Fatal("no-defense should detect nothing")
	}
}

func TestDeceptionIsZeroFalsePositive(t *testing.T) {
	// Across many seeds, fixed deception must never contain a benign source.
	for seed := int64(0); seed < 30; seed++ {
		s := GenerateScenario(seed)
		m := RunScenario(s, FixedDeception{})
		if m.FalsePositiveSources != 0 {
			t.Fatalf("seed %d: deception produced a false positive (FPR=%.2f)", seed, m.FPR())
		}
	}
}

// TestFourBaselineExperiment is the CP8 acceptance test: one call runs the
// controlled four-baseline comparison over identical seeded scenarios and
// produces reproducible machine + human reports with real measured metrics.
func TestFourBaselineExperiment(t *testing.T) {
	seeds := []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	exp := RunFourBaselineExperiment(seeds)
	if len(exp.Results) != 4 {
		t.Fatalf("expected 4 baselines, got %d", len(exp.Results))
	}

	by := map[string]BaselineResult{}
	for _, r := range exp.Results {
		by[r.Defense] = r
	}

	// Measured expectations (asserted because the scenario generator guarantees
	// them, not presumed):
	// - no_defense completes the objective every trial; deception baselines
	//   prevent it (they contain on the decoy trip, which precedes the goal).
	if by["no_defense"].ObjectiveCompletion.Mean < 0.99 {
		t.Fatalf("no_defense objective completion should be ~1.0, got %.2f", by["no_defense"].ObjectiveCompletion.Mean)
	}
	if by["fixed_deception"].ObjectiveCompletion.Mean > 0.01 {
		t.Fatalf("fixed_deception should prevent the objective, got %.2f", by["fixed_deception"].ObjectiveCompletion.Mean)
	}
	// - deception baselines keep FPR at 0.
	if by["fixed_deception"].FPR.Mean != 0 || by["adaptive_deception"].FPR.Mean != 0 {
		t.Fatal("deception baselines must have zero FPR")
	}
	// - adaptive detects at least as early as fixed (behavioral escalation can
	//   fire before the decoy trip). We MEASURE, not presume: assert <=.
	if by["adaptive_deception"].DetectionLatencySec.Mean > by["fixed_deception"].DetectionLatencySec.Mean+1e-9 {
		t.Fatalf("adaptive latency (%.1f) should be <= fixed (%.1f)",
			by["adaptive_deception"].DetectionLatencySec.Mean, by["fixed_deception"].DetectionLatencySec.Mean)
	}

	// Reproducibility: a second run yields identical JSON.
	var j1, j2 bytes.Buffer
	_ = exp.WriteJSON(&j1)
	_ = RunFourBaselineExperiment(seeds).WriteJSON(&j2)
	if j1.String() != j2.String() {
		t.Fatal("experiment not reproducible across runs")
	}

	// Reports render and carry the honesty note.
	var md bytes.Buffer
	if err := exp.WriteMarkdown(&md); err != nil {
		t.Fatalf("markdown: %v", err)
	}
	for _, want := range []string{"four-baseline", "no_defense", "adaptive_deception", "Not measured"} {
		if !strings.Contains(md.String(), want) {
			t.Fatalf("markdown report missing %q", want)
		}
	}
	t.Logf("\n%s", md.String())
}
