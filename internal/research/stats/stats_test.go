package stats

import (
	"math"
	"testing"
)

func approx(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

func TestWilsonKnownValues(t *testing.T) {
	// 0/10 and 10/10 must stay in [0,1] and not be degenerate at the boundary.
	lo := Wilson95(0, 10)
	if lo.Point != 0 || lo.Lo != 0 || lo.Hi <= 0 || lo.Hi > 1 {
		t.Fatalf("Wilson 0/10 = %+v", lo)
	}
	hi := Wilson95(10, 10)
	if hi.Point != 1 || !approx(hi.Hi, 1, 1e-9) || hi.Lo >= 1 || hi.Lo < 0 {
		t.Fatalf("Wilson 10/10 = %+v", hi)
	}
	// 50/100 ~ 0.5, interval roughly [0.404, 0.596].
	mid := Wilson95(50, 100)
	if !approx(mid.Point, 0.5, 1e-9) || !approx(mid.Lo, 0.404, 0.01) || !approx(mid.Hi, 0.596, 0.01) {
		t.Fatalf("Wilson 50/100 = %+v", mid)
	}
}

func TestWilsonWiderForSmallerN(t *testing.T) {
	small := Wilson95(1, 10)
	big := Wilson95(10, 100)
	if (small.Hi - small.Lo) <= (big.Hi - big.Lo) {
		t.Fatalf("smaller n should give wider interval: small=%+v big=%+v", small, big)
	}
}

func TestBootstrapContainsMeanDeterministic(t *testing.T) {
	xs := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	a := Bootstrap(xs, 2000, 42)
	b := Bootstrap(xs, 2000, 42)
	if a != b {
		t.Fatalf("bootstrap not deterministic: %+v vs %+v", a, b)
	}
	if !(a.Lo <= a.Point && a.Point <= a.Hi) {
		t.Fatalf("mean not inside interval: %+v", a)
	}
	if a.Lo >= a.Hi {
		t.Fatalf("degenerate interval: %+v", a)
	}
}

func TestBootstrapDiffSign(t *testing.T) {
	a := []float64{10, 11, 9, 10, 10}
	b := []float64{1, 2, 0, 1, 1}
	d := BootstrapDiff(a, b, 2000, 7)
	if d.Point <= 0 || d.Lo <= 0 {
		t.Fatalf("expected clearly positive difference with CI above 0: %+v", d)
	}
}

func TestConfusionAndRates(t *testing.T) {
	scores := []float64{0.9, 0.8, 0.2, 0.1}
	labels := []bool{true, false, true, false}
	c := ConfusionAt(scores, labels, 0.5)
	if c.TP != 1 || c.FP != 1 || c.FN != 1 || c.TN != 1 {
		t.Fatalf("confusion = %+v", c)
	}
	if !approx(c.Recall(), 0.5, 1e-9) || !approx(c.FPR(), 0.5, 1e-9) || !approx(c.Precision(), 0.5, 1e-9) {
		t.Fatalf("rates wrong: %+v", c)
	}
}

func TestPRAUCPerfectAndRandom(t *testing.T) {
	// Perfect separation -> AUC 1.
	scores := []float64{0.9, 0.8, 0.2, 0.1}
	labels := []bool{true, true, false, false}
	if got := PRAUC(scores, labels); !approx(got, 1.0, 1e-9) {
		t.Fatalf("perfect PR-AUC = %v, want 1", got)
	}
	// All-negative labels -> 0 (undefined positive class).
	if got := PRAUC([]float64{0.5, 0.4}, []bool{false, false}); got != 0 {
		t.Fatalf("no-positive PR-AUC = %v, want 0", got)
	}
}

func TestPRAUCInformativeBeatsInverted(t *testing.T) {
	labels := []bool{true, true, false, false, true, false}
	good := []float64{0.95, 0.9, 0.3, 0.2, 0.8, 0.1}
	bad := []float64{0.1, 0.2, 0.8, 0.9, 0.15, 0.95}
	if PRAUC(good, labels) <= PRAUC(bad, labels) {
		t.Fatal("informative scores should have higher PR-AUC than anti-correlated")
	}
}
