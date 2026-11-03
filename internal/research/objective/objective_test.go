package objective

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Armur-Ai/heramba/internal/assetgraph"
	"github.com/Armur-Ai/heramba/internal/deception"
)

func TestGeneratedWorldIsConsistent(t *testing.T) {
	for _, seed := range []int64{1, 2, 42, 1000} {
		w := deception.GenerateWorld(seed, 8, 5, 20)
		if err := ValidateWorld(w); err != nil {
			t.Fatalf("seed %d: generated world must be consistent: %v", seed, err)
		}
	}
}

func TestWorldIsReproducible(t *testing.T) {
	if err := Reproducible(7, 8, 5, 20); err != nil {
		t.Fatalf("world must regenerate identically: %v", err)
	}
}

// Real/synthetic isolation: a synthetic world must never carry a real-looking
// email domain (so a synthetic secret can never be mistaken for a real one).
func TestNoRealEmailDomains(t *testing.T) {
	w := deception.GenerateWorld(3, 10, 6, 30)
	for id, u := range w.Users {
		if !hasReservedSuffix(u.Email) {
			t.Errorf("user %s has non-reserved email %q (could resolve to a real system)", id, u.Email)
		}
	}
}

// The validator must CATCH injected inconsistencies — the kind a deception-aware
// adversary probes for.
func TestValidatorCatchesInconsistencies(t *testing.T) {
	// Dangling resource -> project reference.
	w := deception.GenerateWorld(5, 6, 4, 10)
	w.Resources["res_bad"] = &deception.Resource{ID: "res_bad", ProjectID: "prj_does_not_exist", Synthetic: true}
	if err := ValidateWorld(w); err == nil || !strings.Contains(err.Error(), "non-existent") {
		t.Fatalf("expected dangling-reference error, got: %v", err)
	}

	// Non-synthetic record.
	w2 := deception.GenerateWorld(6, 6, 4, 10)
	for _, u := range w2.Users {
		u.Synthetic = false
		break
	}
	if err := ValidateWorld(w2); err == nil || !strings.Contains(err.Error(), "not tagged synthetic") {
		t.Fatalf("expected not-synthetic error, got: %v", err)
	}

	// Real-looking email domain.
	w3 := deception.GenerateWorld(8, 6, 4, 10)
	for _, u := range w3.Users {
		u.Email = "ceo@real-company.com"
		break
	}
	if err := ValidateWorld(w3); err == nil || !strings.Contains(err.Error(), "non-reserved") {
		t.Fatalf("expected real-email error, got: %v", err)
	}
}

// Asset-graph isolation: the real graph must fail closed on adversary export,
// and real/synthetic separation must validate.
func TestAssetGraphFailsClosedOnReal(t *testing.T) {
	real := assetgraph.New(assetgraph.Realm("real"))
	real.AddNode(assetgraph.Node{ID: "db"})
	var buf bytes.Buffer
	if err := real.WriteAdversaryView(&buf); err == nil {
		t.Fatal("expected WriteAdversaryView to fail closed on a real graph")
	}
	synth := assetgraph.New(assetgraph.Realm("synthetic"))
	synth.AddNode(assetgraph.Node{ID: "decoy-db"})
	if err := synth.WriteAdversaryView(&buf); err != nil {
		t.Fatalf("synthetic graph should export fine: %v", err)
	}
}

// H2 (naive attacker): every deception arm reduces objective completion vs
// unprotected, and adaptive deception is the strongest.
func TestH2NaiveAttacker(t *testing.T) {
	cfg := DefaultConfig(seeds(200))
	rep := Run(cfg)
	comp := completionByArm(rep)
	t.Logf("naive completion: %v", comp)
	if comp["unprotected"] < 0.99 {
		t.Fatalf("unprotected should almost always complete, got %.3f", comp["unprotected"])
	}
	for _, arm := range []string{"static_decoys", "stateful_deception", "adaptive_deception"} {
		if comp[arm] >= comp["unprotected"] {
			t.Errorf("%s should reduce completion vs unprotected (%.3f vs %.3f)", arm, comp[arm], comp["unprotected"])
		}
	}
	if comp["adaptive_deception"] > comp["stateful_deception"] {
		t.Errorf("adaptive should be at least as strong as stateful: %.3f vs %.3f",
			comp["adaptive_deception"], comp["stateful_deception"])
	}
}

// H2 (deception-aware attacker): the substantive finding — static decoys
// collapse against an adversary that rejects inconsistent lures, but stateful
// and adaptive deception (consistent lures) still suppress completion. This is
// why the consistency validator matters.
func TestH2DeceptionAwareAttacker(t *testing.T) {
	cfg := DefaultConfig(seeds(200))
	cfg.Attacker.DeceptionAware = true
	rep := Run(cfg)
	comp := completionByArm(rep)
	disc := discoveryByArm(rep)
	t.Logf("aware completion: %v", comp)
	t.Logf("aware discovery: %v", disc)

	// The aware attacker discovers (rejects) the low-consistency static lures...
	if disc["static_decoys"] < 0.5 {
		t.Errorf("aware attacker should discover most static lures, got %.3f", disc["static_decoys"])
	}
	// ...but NOT the consistent stateful/adaptive ones.
	if disc["stateful_deception"] > 0.1 || disc["adaptive_deception"] > 0.1 {
		t.Errorf("consistent lures should rarely be discovered: stateful=%.3f adaptive=%.3f",
			disc["stateful_deception"], disc["adaptive_deception"])
	}
	// Static decoys lose most of their protection; adaptive keeps completion low.
	if comp["adaptive_deception"] >= comp["static_decoys"] {
		t.Errorf("against an aware attacker, adaptive should beat static decoys: adaptive=%.3f static=%.3f",
			comp["adaptive_deception"], comp["static_decoys"])
	}
}

// Decoy-based arms must never disrupt benign traffic; a blunt rate limit does.
func TestBenignDisruptionOnlyFromRateLimit(t *testing.T) {
	rep := Run(DefaultConfig(seeds(50)))
	for _, a := range rep.Arms {
		switch a.Arm {
		case "conventional":
			if a.BenignDisruption <= 0 {
				t.Errorf("conventional rate limiting should disrupt some benign traffic")
			}
		default:
			if a.BenignDisruption != 0 {
				t.Errorf("%s should have zero benign disruption, got %.3f", a.Arm, a.BenignDisruption)
			}
		}
	}
}

func TestReportRenders(t *testing.T) {
	rep := Run(DefaultConfig(seeds(20)))
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

// helpers

func seeds(n int) []int64 {
	out := make([]int64, n)
	for i := range out {
		out[i] = int64(1000 + i)
	}
	return out
}

func completionByArm(r Report) map[string]float64 {
	m := map[string]float64{}
	for _, a := range r.Arms {
		m[a.Arm] = a.ObjectiveCompletion.Point
	}
	return m
}

func discoveryByArm(r Report) map[string]float64 {
	m := map[string]float64{}
	for _, a := range r.Arms {
		m[a.Arm] = a.DeceptionDiscovery.Point
	}
	return m
}
