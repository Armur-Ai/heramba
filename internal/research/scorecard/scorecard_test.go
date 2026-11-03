package scorecard

import (
	"bytes"
	"strings"
	"testing"
)

func TestScorecardBuildsAllRows(t *testing.T) {
	sc := Build("test")
	if len(sc.Rows) < 5 {
		t.Fatalf("expected >=5 scored rows, got %d", len(sc.Rows))
	}
	want := map[string]bool{
		"recall (held-out coordinated)":                                      false,
		"false-positive rate":                                                false,
		"objective-completion reduction (adaptive vs unprotected)":           false,
		"policy-safe response latency (response component, worst archetype)": false,
		"valid-artifact run rate":                                            false,
	}
	for _, r := range sc.Rows {
		if _, ok := want[r.Metric]; ok {
			want[r.Metric] = true
		}
	}
	for m, found := range want {
		if !found {
			t.Errorf("missing scorecard row %q", m)
		}
	}
}

// The scorecard must report honest outcomes: recall and objective-reduction meet
// their targets; the FPR target is a MISS (reported, not hidden).
func TestScorecardHonestOutcomes(t *testing.T) {
	sc := Build("test")
	by := map[string]Row{}
	for _, r := range sc.Rows {
		by[r.Metric] = r
	}
	if !by["recall (held-out coordinated)"].Met {
		t.Errorf("recall target should be met (observed %.3f)", by["recall (held-out coordinated)"].Observed.Point)
	}
	if !by["objective-completion reduction (adaptive vs unprotected)"].Met {
		t.Errorf("objective-reduction target should be met")
	}
	if by["false-positive rate"].Met {
		t.Errorf("FPR target is expected to MISS at 1%% on hard synthetic benign; must be reported honestly, not marked met")
	}
	if !by["policy-safe response latency (response component, worst archetype)"].Met {
		t.Errorf("response-latency target should be met (<=5s)")
	}
}

// The adaptive-vs-static effect size must be clearly positive (adaptive prevents
// more objectives) with a CI above zero.
func TestEffectSizePositive(t *testing.T) {
	sc := Build("test")
	if len(sc.EffectSizes) == 0 {
		t.Fatal("expected at least one effect size")
	}
	e := sc.EffectSizes[0]
	if e.Estimate.Point <= 0 || e.Estimate.Lo <= 0 {
		t.Fatalf("expected positive effect with CI above 0, got %+v", e.Estimate)
	}
}

func TestScorecardRenders(t *testing.T) {
	sc := Build("test")
	var md, js bytes.Buffer
	if err := sc.WriteMarkdown(&md); err != nil {
		t.Fatal(err)
	}
	if err := sc.WriteJSON(&js); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(md.String(), "Targets vs. observed") {
		t.Fatal("markdown missing title")
	}
	if js.Len() == 0 {
		t.Fatal("empty json")
	}
}
