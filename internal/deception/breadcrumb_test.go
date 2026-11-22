package deception

import (
	"strings"
	"testing"

	"github.com/Armur-Ai/heramba/internal/event"
)

func TestTrailDeterministic(t *testing.T) {
	a := BuildTrail(42, 5)
	b := BuildTrail(42, 5)
	if strings.Join(a.Paths(), ",") != strings.Join(b.Paths(), ",") {
		t.Fatal("same seed must yield identical trail")
	}
	if BuildTrail(43, 5).Paths()[0] == a.Paths()[0] {
		t.Fatal("different seed should change the trail")
	}
}

func TestTrailCoherent(t *testing.T) {
	tr := BuildTrail(7, 6)
	steps := tr.Steps()
	if len(steps) != 6 {
		t.Fatalf("len = %d", len(steps))
	}
	for i := 0; i < len(steps)-1; i++ {
		if steps[i].Next != steps[i+1].Path {
			t.Fatalf("step %d Next %q != step %d Path %q", i, steps[i].Next, i+1, steps[i+1].Path)
		}
		if steps[i].Secret != "" {
			t.Fatalf("non-final step %d should carry no secret", i)
		}
		if !strings.Contains(steps[i].Hint, steps[i].Next) {
			t.Fatalf("step %d hint should mention next path", i)
		}
	}
	last := steps[len(steps)-1]
	if last.Next != "" || last.Secret == "" {
		t.Fatalf("final step should be a dead-end with a secret: %+v", last)
	}
}

func TestTrailMinLength(t *testing.T) {
	if len(BuildTrail(1, 0).Steps()) < 2 {
		t.Fatal("trail should clamp to a minimum length of 2")
	}
}

func TestTrailLookupAndPaths(t *testing.T) {
	tr := BuildTrail(9, 4)
	for _, p := range tr.Paths() {
		if _, ok := tr.Lookup(p); !ok {
			t.Fatalf("path %q should be on the trail", p)
		}
	}
	if _, ok := tr.Lookup("/not/on/trail"); ok {
		t.Fatal("off-trail path must not match")
	}
	if len(tr.Secrets()) != 1 {
		t.Fatalf("expected exactly one payoff secret, got %d", len(tr.Secrets()))
	}
}

func TestRenderHint(t *testing.T) {
	tr := BuildTrail(3, 3)
	steps := tr.Steps()
	mid := tr.RenderHint(steps[0])
	if !strings.Contains(mid, steps[0].Next) || !strings.Contains(mid, "<a href") {
		t.Fatalf("intermediate hint should link onward: %q", mid)
	}
	fin := tr.RenderHint(steps[len(steps)-1])
	if !strings.Contains(fin, "api_token") || !strings.Contains(fin, "synthetic") {
		t.Fatalf("final hint should be a synthetic secret blob: %q", fin)
	}
}

func TestTrailEvent(t *testing.T) {
	tr := BuildTrail(5, 3)
	steps := tr.Steps()
	ev := tr.Event(steps[0], "1.1.1.1")
	if ev.Kind != event.KindDecoyHit || ev.Confidence != 1.0 {
		t.Fatalf("expected zero-FP decoy hit, got %q conf=%v", ev.Kind, ev.Confidence)
	}
	if ev.Labels["breadcrumb"] != "true" || ev.Labels["trail_step"] != "0" {
		t.Fatalf("labels = %+v", ev.Labels)
	}
	if ev.Labels["trail_final"] != "" {
		t.Fatal("step 0 should not be marked final")
	}
	finEv := tr.Event(steps[len(steps)-1], "1.1.1.1")
	if finEv.Labels["trail_final"] != "true" {
		t.Fatalf("last step should be marked final: %+v", finEv.Labels)
	}
}
