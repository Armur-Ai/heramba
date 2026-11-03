package lab02

import (
	"testing"
	"time"

	"github.com/Armur-Ai/heramba/internal/campaign"
	"github.com/Armur-Ai/heramba/internal/event"
	"github.com/Armur-Ai/heramba/internal/evidence"
	"github.com/Armur-Ai/heramba/labs/labkit"
)

// Lab 02 — Distributed campaign reconstruction.
//
// Objective: reconstruct a coordinated multi-source campaign from raw evidence
// and see how Heramba separates what it OBSERVED (sessions) from what it
// INFERRED (actors, campaign).
func TestLab02(t *testing.T) {
	labkit.Banner(t, "Lab 02 — Distributed campaign reconstruction")
	store := evidence.NewStore()
	base := time.Now().UTC().Add(-time.Hour)

	labkit.Step(t, 1, "feed two synchronized scanners hitting the same surface")
	for i, src := range []string{"192.0.2.10", "192.0.2.11"} {
		for j, p := range []string{"/api/users/1", "/api/users/2", "/admin", "/.env"} {
			e := evidence.NewSecurityEvent(event.KindProbe, event.SeverityMedium, src, evidence.Provenance{
				Sensor: "edge", ObservedAt: base.Add(time.Duration(i)*time.Second + time.Duration(j)*2*time.Second),
			}).WithAttr("ua", "nuclei/3.1").WithAttr("path", p)
			e.Target = p
			e.Confidence = 0.5
			if p == "/.env" {
				e.Stage = evidence.StageDeceptionTouch
				e.Confidence = 1.0
			}
			labkit.Require(t, store.PutEvent(e) == nil, "evidence should store")
		}
	}

	labkit.Step(t, 2, "run the correlator")
	res := campaign.DefaultCorrelator().Reconstruct(store)
	labkit.Observe(t, "reconstructed %d sessions, %d actors, %d campaigns",
		len(res.Sessions), len(res.Actors), len(res.Campaigns))

	labkit.Require(t, len(res.Sessions) == 2, "two sources → two observed sessions")
	labkit.Require(t, len(res.Actors) == 2, "distinct IPs stay distinct actors (no identity collapse on shared tool)")
	labkit.Require(t, len(res.Campaigns) == 1, "shared targeting in one window → one inferred campaign")

	labkit.Step(t, 3, "the explanation separates observed facts from inferred links")
	var actor evidence.ID
	for id := range res.Actors {
		actor = id
		break
	}
	exp := res.Explain(actor)
	labkit.Require(t, contains(exp, "observed (facts)") && contains(exp, "inferred (hypothesis)"),
		"the explanation must separate facts from inference")
	t.Logf("\n%s", exp)
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (indexOf(s, sub) >= 0)
}
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
