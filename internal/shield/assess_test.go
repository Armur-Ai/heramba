package shield

import "testing"

func TestAssess_OverallIsWorstOutcome(t *testing.T) {
	cases := []struct {
		name    string
		results []Result
		want    Status
		finds   int
	}{
		{
			name: "findings wins over everything",
			results: []Result{
				{Tool: "agat", Status: StatusFindings, Findings: 2},
				{Tool: "byakugan", Status: StatusError},
				{Tool: "sharingan", Status: StatusUnavailable},
			},
			want: StatusFindings, finds: 2,
		},
		{
			name: "error when no findings",
			results: []Result{
				{Tool: "agat", Status: StatusClean},
				{Tool: "byakugan", Status: StatusError},
			},
			want: StatusError,
		},
		{
			name: "all clean",
			results: []Result{
				{Tool: "agat", Status: StatusClean},
				{Tool: "byakugan", Status: StatusClean},
			},
			want: StatusClean,
		},
		{
			name: "nothing ran is unavailable, not clean",
			results: []Result{
				{Tool: "agat", Status: StatusUnavailable},
				{Tool: "byakugan", Status: StatusUnavailable},
			},
			want: StatusUnavailable,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := Assess(c.results)
			if a.Overall != c.want {
				t.Errorf("Overall = %q, want %q", a.Overall, c.want)
			}
			if a.TotalFindings != c.finds {
				t.Errorf("TotalFindings = %d, want %d", a.TotalFindings, c.finds)
			}
		})
	}
}

func TestAssess_CoverageAndGaps(t *testing.T) {
	a := Assess([]Result{
		{Tool: "agat", Status: StatusFindings, Findings: 1},
		{Tool: "byakugan", Status: StatusUnavailable},
		{Tool: "sharingan", Status: StatusUnavailable},
		{Tool: "skillscan", Status: StatusError},
	})
	if a.Coverage.Total != 4 || a.Coverage.Ran != 1 || a.Coverage.Unavailable != 2 || a.Coverage.Errored != 1 {
		t.Errorf("coverage wrong: %+v", a.Coverage)
	}
	if len(a.Gaps) != 2 || a.Gaps[0] != "byakugan" || a.Gaps[1] != "sharingan" {
		t.Errorf("gaps wrong (want sorted byakugan,sharingan): %v", a.Gaps)
	}
	if len(a.Errors) != 1 || a.Errors[0] != "skillscan" {
		t.Errorf("errors wrong: %v", a.Errors)
	}
}

func TestShouldFail_Policies(t *testing.T) {
	findings := Assess([]Result{{Tool: "a", Status: StatusFindings, Findings: 1}})
	errored := Assess([]Result{{Tool: "a", Status: StatusClean}, {Tool: "b", Status: StatusError}})
	gapped := Assess([]Result{{Tool: "a", Status: StatusClean}, {Tool: "b", Status: StatusUnavailable}})
	clean := Assess([]Result{{Tool: "a", Status: StatusClean}})

	check := func(name string, a Assessment, p FailPolicy, want bool) {
		if got := a.ShouldFail(p); got != want {
			t.Errorf("%s under %q: ShouldFail = %v, want %v", name, p, got, want)
		}
	}
	// none never fails
	check("findings", findings, FailNever, false)
	// findings policy
	check("findings", findings, FailOnFindings, true)
	check("errored", errored, FailOnFindings, false)
	// error policy
	check("errored", errored, FailOnError, true)
	check("gapped", gapped, FailOnError, false)
	// gap policy (strictest)
	check("gapped", gapped, FailOnGap, true)
	check("clean", clean, FailOnGap, false)
}

func TestParseFailPolicy(t *testing.T) {
	if p, ok := ParseFailPolicy("gap"); !ok || p != FailOnGap {
		t.Errorf("parse gap failed: %v %v", p, ok)
	}
	if _, ok := ParseFailPolicy("bogus"); ok {
		t.Error("bogus policy should not parse")
	}
}
