package swarm

import (
	"fmt"
	"testing"
	"time"

	"github.com/Armur-Ai/heramba/internal/campaign"
	"github.com/Armur-Ai/heramba/internal/evidence"
)

// mkSession fabricates a session with controllable features for the benchmark.
func mkSession(src string, start time.Time, dur time.Duration, n int, targets []string, decoy bool) campaign.Session {
	s := campaign.Session{
		ID:           evidence.NewID("sess"),
		SourceKey:    src,
		Start:        start,
		End:          start.Add(dur),
		Targets:      map[string]int{},
		DeceptionHit: decoy,
	}
	for i := 0; i < n; i++ {
		s.EventIDs = append(s.EventIDs, evidence.NewID("se"))
	}
	for _, t := range targets {
		s.Targets[t]++
	}
	return s
}

func paths(prefix string, from, to int) []string {
	var out []string
	for i := from; i < to; i++ {
		out = append(out, fmt.Sprintf("%s/%d", prefix, i))
	}
	return out
}

// genCohort builds a labeled synthetic cohort of the requested class.
func genCohort(class Class, base time.Time, variant int) []campaign.Session {
	switch class {
	case ClassSequential:
		// one source enumerating a wide, novel surface, trips a decoy
		return []campaign.Session{
			mkSession("198.51.100.10", base, 2*time.Minute, 30, append(paths("/api/items", 0, 20+variant), "/.env"), true),
		}
	case ClassLegitAutomation:
		// one source hitting a few known endpoints repeatedly, no decoy
		return []campaign.Session{
			mkSession("10.0.0.5", base, 10*time.Minute, 300, []string{"/health", "/metrics", "/api/status"}, false),
		}
	case ClassIndependent:
		// several sources, DIFFERENT surfaces, spread far apart in time
		return []campaign.Session{
			mkSession("203.0.113.1", base, time.Minute, 10, paths("/wp-login", 0, 5), false),
			mkSession("203.0.113.2", base.Add(40*time.Minute), time.Minute, 10, paths("/phpmyadmin", 0, 5), false),
			mkSession("203.0.113.3", base.Add(2*time.Hour), time.Minute, 10, paths("/cgi-bin", 0, 5), false),
		}
	case ClassCoordinated:
		// several sources, synchronized starts, dividing one surface between
		// them (division of labor), some trip decoys
		return []campaign.Session{
			mkSession("192.0.2.1", base, time.Minute, 15, paths("/api/users", 0, 10), false),
			mkSession("192.0.2.2", base.Add(15*time.Second), time.Minute, 15, paths("/api/users", 10, 20), true),
			mkSession("192.0.2.3", base.Add(30*time.Second), time.Minute, 15, paths("/api/users", 20, 30), false),
		}
	}
	return nil
}

// TestFourClassBenchmark is the CP6 acceptance test: it generates labeled
// cohorts of all four classes, classifies them, and reports a confusion matrix
// with per-class precision/recall and the overall false-positive rate (benign
// misclassified as a threat). These are REAL measured numbers, not asserted.
func TestFourClassBenchmark(t *testing.T) {
	d := DefaultDetector()
	base := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	classes := []Class{ClassSequential, ClassLegitAutomation, ClassIndependent, ClassCoordinated}

	const trials = 25
	// confusion[actual][predicted]
	confusion := map[Class]map[Class]int{}
	for _, a := range classes {
		confusion[a] = map[Class]int{}
	}
	var totalLatency time.Duration
	for _, actual := range classes {
		for v := 0; v < trials; v++ {
			cohort := genCohort(actual, base.Add(time.Duration(v)*time.Hour), v)
			start := time.Now()
			verdict := d.Classify(cohort)
			totalLatency += time.Since(start)
			confusion[actual][verdict.Class]++
		}
	}

	// Report + assert per-class recall and the benign false-positive rate.
	t.Logf("mean classify latency: %s", totalLatency/time.Duration(len(classes)*trials))
	var worstRecall = 1.0
	for _, actual := range classes {
		correct := confusion[actual][actual]
		recall := float64(correct) / float64(trials)
		// precision across classes
		var predictedTotal int
		for _, a2 := range classes {
			predictedTotal += confusion[a2][actual]
		}
		precision := 0.0
		if predictedTotal > 0 {
			precision = float64(confusion[actual][actual]) / float64(predictedTotal)
		}
		t.Logf("class=%-20s recall=%.2f precision=%.2f (row=%v)", actual, recall, precision, confusion[actual])
		if recall < worstRecall {
			worstRecall = recall
		}
	}

	// False positives: benign (legit automation) predicted as any threat class.
	benign := confusion[ClassLegitAutomation]
	fp := benign[ClassSequential] + benign[ClassIndependent] + benign[ClassCoordinated]
	fpr := float64(fp) / float64(trials)
	t.Logf("benign false-positive rate: %.2f", fpr)

	if worstRecall < 0.8 {
		t.Fatalf("a class recall dropped below 0.8 (worst=%.2f) — detector regressed", worstRecall)
	}
	if fpr > 0.1 {
		t.Fatalf("benign false-positive rate too high: %.2f", fpr)
	}
}

func TestCoordinatedIsExplainable(t *testing.T) {
	d := DefaultDetector()
	base := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	v := d.Classify(genCohort(ClassCoordinated, base, 0))
	if v.Class != ClassCoordinated {
		t.Fatalf("expected coordinated, got %s (%v)", v.Class, v.Reasons)
	}
	if len(v.Reasons) == 0 {
		t.Fatal("coordinated verdict must be explainable")
	}
	if v.Confidence <= 0 || v.Confidence > 1 {
		t.Fatalf("confidence out of range: %v", v.Confidence)
	}
}

func TestDoesNotRelyOnRateAlone(t *testing.T) {
	// A very high request rate on a narrow, known surface with no decoy contact
	// must NOT be called a threat — guarding against naive rate-only detection.
	d := DefaultDetector()
	base := time.Now()
	cohort := []campaign.Session{
		mkSession("10.1.1.1", base, time.Second, 5000, []string{"/api/poll"}, false),
	}
	v := d.Classify(cohort)
	if v.Class != ClassLegitAutomation {
		t.Fatalf("high-rate narrow benign traffic misclassified as %s: %v", v.Class, v.Reasons)
	}
}
