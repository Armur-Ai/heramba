package bizlogic

import (
	"testing"
	"time"
)

func testSpec() *Spec {
	return &Spec{
		Operations: []Operation{
			{ID: "login", Method: "POST", PathTemplate: "/api/login"},
			{ID: "getProject", Method: "GET", PathTemplate: "/api/projects/{id}",
				OwnershipParam: "id", ResourceType: "project"},
			{ID: "createCart", Method: "POST", PathTemplate: "/api/cart"},
			{ID: "checkout", Method: "POST", PathTemplate: "/api/cart/checkout"},
			{ID: "adminPurge", Method: "DELETE", PathTemplate: "/api/admin/purge",
				RequiredRole: "admin"},
		},
		Workflows: []Workflow{
			{
				ID:    "purchase",
				Start: "anon",
				Transitions: map[string][]Transition{
					"anon":   {{OpID: "login", To: "authed"}},
					"authed": {{OpID: "createCart", To: "cart"}},
					"cart":   {{OpID: "checkout", To: "done"}},
				},
			},
		},
	}
}

func TestPathTemplateMatching(t *testing.T) {
	s := testSpec()
	op, params, ok := s.Match("GET", "/api/projects/prj_0001")
	if !ok || op.ID != "getProject" || params["id"] != "prj_0001" {
		t.Fatalf("template match failed: %v %v %v", ok, op.ID, params)
	}
	if _, _, ok := s.Match("GET", "/api/nope/1/2"); ok {
		t.Fatal("non-matching path matched")
	}
}

// TestBOLADetectedButAuthorizedAllowed is the CP4 acceptance test: an object
// authorization violation (accessing a project the actor can't access) is
// detected, while a structurally identical authorized access is allowed.
func TestBOLADetectedButAuthorizedAllowed(t *testing.T) {
	d := NewDetector(testSpec())
	now := time.Now()

	// Authorized access: ObjectOwned=true → no finding.
	ok := d.Observe(Request{Time: now, Method: "GET", Path: "/api/projects/prj_mine",
		ActorID: "viewer1", Role: "viewer", ObjectOwned: true})
	if len(ok) != 0 {
		t.Fatalf("authorized access wrongly flagged: %+v", ok)
	}

	// Unauthorized access to another object: same shape, ObjectOwned=false.
	bad := d.Observe(Request{Time: now.Add(time.Second), Method: "GET", Path: "/api/projects/prj_other",
		ActorID: "viewer1", Role: "viewer", ObjectOwned: false})
	if len(bad) != 1 || bad[0].Kind != FindingBOLA {
		t.Fatalf("BOLA not detected: %+v", bad)
	}
	if bad[0].Explanation == "" || len(bad[0].Sequence) == 0 {
		t.Fatal("finding must be explainable and carry the request sequence")
	}
}

func TestPrivilegeViolation(t *testing.T) {
	d := NewDetector(testSpec())
	f := d.Observe(Request{Time: time.Now(), Method: "DELETE", Path: "/api/admin/purge",
		ActorID: "member1", Role: "member"})
	if len(f) != 1 || f[0].Kind != FindingPrivilege {
		t.Fatalf("privilege violation not detected: %+v", f)
	}
	// An admin doing the same is allowed.
	d2 := NewDetector(testSpec())
	if f := d2.Observe(Request{Time: time.Now(), Method: "DELETE", Path: "/api/admin/purge",
		ActorID: "admin1", Role: "admin"}); len(f) != 0 {
		t.Fatalf("admin wrongly flagged: %+v", f)
	}
}

func TestSequenceViolationVsCorrectOrder(t *testing.T) {
	// Correct order: login → createCart → checkout — no sequence findings.
	d := NewDetector(testSpec())
	now := time.Now()
	steps := []struct{ m, p string }{
		{"POST", "/api/login"},
		{"POST", "/api/cart"},
		{"POST", "/api/cart/checkout"},
	}
	for i, s := range steps {
		f := d.Observe(Request{Time: now.Add(time.Duration(i) * time.Second), Method: s.m, Path: s.p,
			ActorID: "buyer", Role: "member"})
		for _, x := range f {
			if x.Kind == FindingSequence {
				t.Fatalf("valid step %d wrongly flagged as out-of-order: %+v", i, x)
			}
		}
	}

	// Out of order: checkout before a cart exists → sequence violation.
	d2 := NewDetector(testSpec())
	d2.Observe(Request{Time: now, Method: "POST", Path: "/api/login", ActorID: "skip", Role: "member"})
	f := d2.Observe(Request{Time: now.Add(time.Second), Method: "POST", Path: "/api/cart/checkout",
		ActorID: "skip", Role: "member"})
	found := false
	for _, x := range f {
		if x.Kind == FindingSequence {
			found = true
		}
	}
	if !found {
		t.Fatalf("out-of-order checkout not detected: %+v", f)
	}
}

func TestUnknownOperationIsProbe(t *testing.T) {
	d := NewDetector(testSpec())
	f := d.Observe(Request{Time: time.Now(), Method: "GET", Path: "/wp-admin/setup-config.php",
		ActorID: "scanner", Role: ""})
	if len(f) != 1 || f[0].Kind != FindingUnknownOp {
		t.Fatalf("undocumented-surface probe not flagged: %+v", f)
	}
}

func TestImportOpenAPI(t *testing.T) {
	doc := []byte(`{
      "paths": {
        "/api/projects/{id}": {
          "get": {
            "operationId": "getProject",
            "x-heramba-ownership-param": "id",
            "x-heramba-resource-type": "project"
          }
        },
        "/api/admin/purge": {
          "delete": { "operationId": "adminPurge", "x-heramba-required-role": "admin" }
        }
      }
    }`)
	s, err := ImportOpenAPI(doc)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if len(s.Operations) != 2 {
		t.Fatalf("expected 2 ops, got %d", len(s.Operations))
	}
	op, _, ok := s.Match("GET", "/api/projects/abc")
	if !ok || op.OwnershipParam != "id" || op.ResourceType != "project" {
		t.Fatalf("x-heramba extensions not ingested: %+v", op)
	}
}
