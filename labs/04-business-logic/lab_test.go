package lab04

import (
	"testing"
	"time"

	"github.com/Armur-Ai/heramba/internal/bizlogic"
	"github.com/Armur-Ai/heramba/labs/labkit"
)

// Lab 04 — Business-logic attack detection.
//
// Objective: detect a multi-request authorization violation (BOLA) and a
// workflow sequence skip, and confirm structurally-identical AUTHORIZED
// activity is allowed.
func TestLab04(t *testing.T) {
	labkit.Banner(t, "Lab 04 — Business-logic attack detection")

	spec := &bizlogic.Spec{
		Operations: []bizlogic.Operation{
			{ID: "getProject", Method: "GET", PathTemplate: "/api/projects/{id}", OwnershipParam: "id", ResourceType: "project"},
			{ID: "createCart", Method: "POST", PathTemplate: "/api/cart"},
			{ID: "checkout", Method: "POST", PathTemplate: "/api/cart/checkout"},
		},
		Workflows: []bizlogic.Workflow{{
			ID: "purchase", Start: "start",
			Transitions: map[string][]bizlogic.Transition{
				"start": {{OpID: "createCart", To: "cart"}},
				"cart":  {{OpID: "checkout", To: "done"}},
			},
		}},
	}

	labkit.Step(t, 1, "authorized object access is allowed")
	d := bizlogic.NewDetector(spec)
	ok := d.Observe(bizlogic.Request{Time: time.Now(), Method: "GET", Path: "/api/projects/mine",
		ActorID: "u1", Role: "viewer", ObjectOwned: true})
	labkit.Require(t, len(ok) == 0, "owned-object access must not be flagged")

	labkit.Step(t, 2, "accessing another tenant's object (BOLA) is detected")
	bad := d.Observe(bizlogic.Request{Time: time.Now(), Method: "GET", Path: "/api/projects/theirs",
		ActorID: "u1", Role: "viewer", ObjectOwned: false})
	labkit.Require(t, len(bad) == 1 && bad[0].Kind == bizlogic.FindingBOLA, "BOLA must be detected")
	labkit.Observe(t, "finding: %s", bad[0].Explanation)

	labkit.Step(t, 3, "a workflow skip (checkout before cart) is detected")
	d2 := bizlogic.NewDetector(spec)
	seq := d2.Observe(bizlogic.Request{Time: time.Now(), Method: "POST", Path: "/api/cart/checkout",
		ActorID: "u2", Role: "member"})
	found := false
	for _, f := range seq {
		if f.Kind == bizlogic.FindingSequence {
			found = true
			labkit.Observe(t, "finding: %s", f.Explanation)
		}
	}
	labkit.Require(t, found, "out-of-order checkout must be detected")
}
