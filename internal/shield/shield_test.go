package shield

import (
	"context"
	"testing"
)

type fakeScanner struct {
	name string
	res  Result
}

func (f fakeScanner) Name() string                        { return f.name }
func (f fakeScanner) Scan(_ context.Context, _ Target) Result { return f.res }

func TestRegistryRunCollectsEveryScanner(t *testing.T) {
	r := New(
		fakeScanner{"a", Result{Tool: "a", Status: StatusFindings, Findings: 2}},
		fakeScanner{"b", Result{Tool: "b", Status: StatusUnavailable, Detail: "not installed"}},
		fakeScanner{"c", Result{Tool: "c", Status: StatusClean}},
	)
	got := r.Run(context.Background(), Target{Path: "/tmp/x"})
	if len(got) != 3 {
		t.Fatalf("want 3 results, got %d", len(got))
	}
	if got[0].Findings != 2 || got[1].Status != StatusUnavailable || got[2].Status != StatusClean {
		t.Fatalf("unexpected results: %+v", got)
	}
}

func TestDefaultSuiteReportsGapsWhenToolsAbsent(t *testing.T) {
	// With no defense binaries installed, every tool must report an explicit
	// unavailable gap — never a silent clean result.
	t.Setenv("PATH", "") // no binaries resolvable
	got := Default().Run(context.Background(), Target{Path: "."})
	names := map[string]Status{}
	for _, r := range got {
		names[r.Tool] = r.Status
	}
	for _, tool := range []string{"agat", "byakugan", "sharingan", "skillscan"} {
		if names[tool] != StatusUnavailable {
			t.Errorf("%s: want unavailable gap, got %q", tool, names[tool])
		}
	}
}

func TestParseAgatJSON(t *testing.T) {
	in := []byte(`[{"verdict":{"severity":"malicious"}},{"verdict":{"severity":"clean"}},{"verdict":{"severity":"suspicious"}}]`)
	vs, err := parseAgatJSON(in)
	if err != nil {
		t.Fatal(err)
	}
	flagged := 0
	for _, v := range vs {
		if v.Verdict.Severity == "malicious" || v.Verdict.Severity == "suspicious" {
			flagged++
		}
	}
	if flagged != 2 {
		t.Fatalf("want 2 flagged, got %d", flagged)
	}
	if _, err := parseAgatJSON(nil); err == nil {
		t.Error("empty input should error, not look clean")
	}
}

func TestCountJSONArray(t *testing.T) {
	if n, ok := countJSONArray([]byte(`[1,2,3]`)); !ok || n != 3 {
		t.Fatalf("want 3,true; got %d,%v", n, ok)
	}
	if _, ok := countJSONArray([]byte(`{"not":"array"}`)); ok {
		t.Error("object should not parse as array")
	}
}
