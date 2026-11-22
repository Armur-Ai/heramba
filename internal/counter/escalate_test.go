package counter

import (
	"strings"
	"testing"

	"github.com/Armur-Ai/heramba/internal/event"
)

func TestGenerateAdvancedDrawsExtendedPool(t *testing.T) {
	base := len(leadTemplates)
	all := len(allTemplates())
	if all <= base {
		t.Fatalf("extended pool (%d) must exceed base (%d)", all, base)
	}
	// An index into the extended range must produce a valid lure.
	l := GenerateAdvanced(base) // first of the extended templates
	if len(l.Key) != 12 || !strings.Contains(l.Comment, l.Key) || len(l.Paths) == 0 {
		t.Fatalf("bad advanced lure: %+v", l)
	}
}

func TestCampaignEscalates(t *testing.T) {
	c := NewCampaign()
	want := []Tier{TierSingle, TierMulti, TierMulti, TierFlood, TierFlood}
	for i, w := range want {
		if got := c.Escalate("4.3.2.1"); got != w {
			t.Fatalf("offense %d: tier %v want %v", i+1, got, w)
		}
	}
	if c.Count("4.3.2.1") != len(want) {
		t.Fatalf("count = %d", c.Count("4.3.2.1"))
	}
	if c.Tier("unknown") != TierNone {
		t.Fatal("unknown source should be TierNone")
	}
	c.Reset("4.3.2.1")
	if c.Tier("4.3.2.1") != TierNone {
		t.Fatal("reset should clear")
	}
}

func TestLuresForTier(t *testing.T) {
	cases := map[Tier]int{TierNone: 0, TierSingle: 1, TierMulti: 3, TierFlood: 6}
	for tier, n := range cases {
		lures := LuresFor(tier)
		if len(lures) != n {
			t.Fatalf("tier %v: %d lures, want %d", tier, len(lures), n)
		}
		keys := map[string]bool{}
		for _, l := range lures {
			if keys[l.Key] {
				t.Fatal("lure keys must be unique within a deployment")
			}
			keys[l.Key] = true
		}
	}
}

func TestInjectHTMLAll(t *testing.T) {
	lures := LuresFor(TierMulti)
	out := string(InjectHTMLAll([]byte("<html><body>x</body></html>"), lures))
	for _, l := range lures {
		if !strings.Contains(out, l.Key) {
			t.Fatalf("missing lure key %s in output", l.Key)
		}
	}
	if !strings.Contains(out, "<body>x") || !strings.Contains(out, "</body></html>") {
		t.Fatalf("original content lost: %s", out)
	}
}

func TestKeysAndDecoyPaths(t *testing.T) {
	lures := LuresFor(TierFlood)
	keys := Keys(lures)
	if len(keys) != len(lures) {
		t.Fatalf("keys = %d, lures = %d", len(keys), len(lures))
	}
	for i := 1; i < len(keys); i++ {
		if keys[i-1] > keys[i] {
			t.Fatal("keys not sorted")
		}
	}
	paths := DecoyPaths(lures)
	if len(paths) == 0 {
		t.Fatal("expected decoy paths")
	}
	for i := 1; i < len(paths); i++ {
		if paths[i-1] == paths[i] {
			t.Fatal("decoy paths must be deduped")
		}
	}
}

func TestDeployEventShape(t *testing.T) {
	lures := LuresFor(TierMulti)
	ev := DeployEvent("9.9.9.9", TierMulti, lures)
	if ev.Kind != event.KindCounterDeployed {
		t.Fatalf("kind = %q", ev.Kind)
	}
	if ev.Source != "9.9.9.9" {
		t.Fatalf("source = %q", ev.Source)
	}
	if ev.Labels["tier"] != "multi" || ev.Labels["lure_count"] != "3" {
		t.Fatalf("labels = %+v", ev.Labels)
	}
	if ev.Labels["lure_keys"] == "" || ev.Labels["decoy_paths"] == "" {
		t.Fatalf("missing key/path labels: %+v", ev.Labels)
	}
}
