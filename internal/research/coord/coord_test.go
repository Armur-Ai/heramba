package coord

import (
	"bytes"
	"testing"

	"github.com/Armur-Ai/heramba/internal/campaign/swarm"
	"github.com/Armur-Ai/heramba/internal/research"
)

func TestCohortDeterministic(t *testing.T) {
	p := research.DefaultProtocol()
	fam := p.TestFamilies()[0]
	actor, _ := p.ActorByID(fam.ActorModel)
	a := GenerateCohort(fam, actor, 9000, DefaultParams())
	b := GenerateCohort(fam, actor, 9000, DefaultParams())
	if len(a.Sessions) != len(b.Sessions) {
		t.Fatalf("non-deterministic cohort size: %d vs %d", len(a.Sessions), len(b.Sessions))
	}
	for i := range a.Sessions {
		if a.Sessions[i].SourceKey != b.Sessions[i].SourceKey || !a.Sessions[i].Start.Equal(b.Sessions[i].Start) {
			t.Fatalf("session %d differs across runs", i)
		}
	}
}

// The fair-workload invariant: aggregate request count is matched across every
// family, so request volume is not a confounder.
func TestWorkloadsAreRequestMatched(t *testing.T) {
	p := research.DefaultProtocol()
	params := DefaultParams()
	var want int = -1
	for _, fam := range p.Families {
		actor, _ := p.ActorByID(fam.ActorModel)
		c := GenerateCohort(fam, actor, fam.Seeds[0], params)
		total := 0
		for _, s := range c.Sessions {
			total += len(s.EventIDs)
		}
		if want < 0 {
			want = total
		} else if total != want {
			t.Fatalf("family %s has %d attacker requests, want matched %d", fam.Name, total, want)
		}
	}
}

// FullHeramba's thresholded decision must agree with the shipped swarm detector's
// coordinated/not verdict on the same cohorts — the experiment exercises the
// real detector's signal set, not a divergent reimplementation.
func TestFullHerambaAgreesWithShippedDetector(t *testing.T) {
	p := research.DefaultProtocol()
	det := swarm.DefaultDetector()
	full := FullHeramba()
	checked := 0
	for _, fam := range p.Families {
		actor, _ := p.ActorByID(fam.ActorModel)
		for _, seed := range fam.Seeds {
			c := GenerateCohort(fam, actor, seed, DefaultParams())
			f := det.Extract(c.Sessions)
			v := det.Classify(c.Sessions)
			shippedCoord := v.Class == swarm.ClassCoordinated
			ourCoord := full.Score(f, det.SyncWindow) >= OperatingThreshold
			if shippedCoord != ourCoord {
				t.Errorf("%s seed %d: shipped=%v ours=%v (score path diverged)", fam.Name, seed, shippedCoord, ourCoord)
			}
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("no cohorts checked")
	}
}

func TestRunProducesCompleteReport(t *testing.T) {
	p := research.DefaultProtocol()
	rep := Run(p, Options{CodeRevision: "test"})
	if rep.Cohorts == 0 || rep.Positives == 0 || rep.Negatives == 0 {
		t.Fatalf("expected mixed cohorts, got %+v", rep)
	}
	if len(rep.Methods) != 4 {
		t.Fatalf("expected 4 methods, got %d", len(rep.Methods))
	}
	if len(rep.Ablations) != 4 {
		t.Fatalf("expected 4 ablations, got %d", len(rep.Ablations))
	}
	// Markdown and JSON must render.
	var md, js bytes.Buffer
	if err := rep.WriteMarkdown(&md); err != nil {
		t.Fatal(err)
	}
	if err := rep.WriteJSON(&js); err != nil {
		t.Fatal(err)
	}
	if md.Len() == 0 || js.Len() == 0 {
		t.Fatal("empty report output")
	}
}

// H1: the full detector must strictly beat every weaker baseline on the
// threshold-free PR-AUC — otherwise coordination features add nothing (the null).
func TestFullRejectsH1Null(t *testing.T) {
	p := research.DefaultProtocol()
	rep := Run(p, Options{CodeRevision: "test"})
	byName := map[string]MethodResult{}
	for _, m := range rep.Methods {
		byName[m.Method] = m
	}
	full := byName["full_heramba"]
	t.Logf("PR-AUC full=%.3f request_only=%.3f static=%.3f graph=%.3f",
		full.PRAUC, byName["request_only"].PRAUC, byName["static_correlation"].PRAUC, byName["graph_correlation"].PRAUC)
	for _, base := range []string{"request_only", "static_correlation", "graph_correlation"} {
		if full.PRAUC <= byName[base].PRAUC {
			t.Fatalf("H1 null not rejected vs %s: full PR-AUC %.3f <= %.3f", base, full.PRAUC, byName[base].PRAUC)
		}
	}
	// Full detector must recall every coordinated cohort and keep FPR low.
	if full.Recall.Point < 1.0 {
		t.Fatalf("expected full recall 1.0, got %.3f", full.Recall.Point)
	}
	if full.FPR.Point >= 0.2 {
		t.Fatalf("expected full FPR < 0.2, got %.3f", full.FPR.Point)
	}
}

// The synchrony gate is what suppresses false positives: removing cross-source
// correlation must collapse recall, and removing campaign memory (synchrony)
// must inflate FPR — the ablations prove each signal earns its place.
func TestAblationsAttributePerformance(t *testing.T) {
	rep := Run(research.DefaultProtocol(), Options{CodeRevision: "test"})
	ab := map[string]MethodResult{}
	for _, m := range rep.Ablations {
		ab[m.Method] = m
	}
	if ab["ablate_no_cross_source"].Recall.Point > 0.1 {
		t.Errorf("removing cross-source correlation should collapse recall, got %.3f", ab["ablate_no_cross_source"].Recall.Point)
	}
	if ab["ablate_no_campaign_memory"].FPR.Point < 0.5 {
		t.Errorf("removing synchrony should inflate FPR, got %.3f", ab["ablate_no_campaign_memory"].FPR.Point)
	}
}

// Metrics (everything but the timing-dependent throughput) must be deterministic
// across runs, so the experiment is reproducible.
func TestReportMetricsDeterministic(t *testing.T) {
	p := research.DefaultProtocol()
	a := Run(p, Options{CodeRevision: "x"})
	b := Run(p, Options{CodeRevision: "x"})
	a.ThroughputRPS, b.ThroughputRPS = 0, 0
	var ba, bb bytes.Buffer
	if err := a.WriteJSON(&ba); err != nil {
		t.Fatal(err)
	}
	if err := b.WriteJSON(&bb); err != nil {
		t.Fatal(err)
	}
	if ba.String() != bb.String() {
		t.Fatal("report metrics are not deterministic across runs")
	}
}
