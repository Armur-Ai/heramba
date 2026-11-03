package assetgraph

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func buildReal() *Graph {
	g := New(RealmReal)
	g.AddNode(Node{ID: "edge", Kind: KindAsset, Label: "edge proxy", Boundary: "dmz", Risk: 0.2})
	g.AddNode(Node{ID: "api", Kind: KindAsset, Label: "app api", Boundary: "app", Risk: 0.4})
	g.AddNode(Node{ID: "db", Kind: KindAsset, Label: "customer db", Boundary: "data", Risk: 1.0, CrownJewel: true})
	g.AddNode(Node{ID: "cache", Kind: KindAsset, Label: "cache", Boundary: "app", Risk: 0.3})
	_ = g.AddEdge(Edge{From: "edge", To: "api", Kind: EdgeReaches, Weight: 0.9})
	_ = g.AddEdge(Edge{From: "api", To: "db", Kind: EdgeReaches, Weight: 0.5})
	_ = g.AddEdge(Edge{From: "api", To: "cache", Kind: EdgeReaches, Weight: 0.95})
	_ = g.AddEdge(Edge{From: "cache", To: "db", Kind: EdgeReaches, Weight: 0.2})
	return g
}

func TestRealGraphNeverExposed(t *testing.T) {
	real := buildReal()
	var buf bytes.Buffer
	if err := real.WriteAdversaryView(&buf); !errors.Is(err, ErrRealGraphExposure) {
		t.Fatalf("real graph must fail closed on adversary export, got err=%v", err)
	}
	if buf.Len() != 0 {
		t.Fatal("real graph wrote bytes to an adversary sink")
	}
}

func TestSyntheticGraphExportsSanitized(t *testing.T) {
	g := New(RealmSynthetic)
	g.AddNode(Node{ID: "s_api", Kind: KindAPI, Label: "fake api", Boundary: "zone", Risk: 0.9, CrownJewel: true})
	g.AddNode(Node{ID: "s_db", Kind: KindAsset, Label: "fake db"})
	_ = g.AddEdge(Edge{From: "s_api", To: "s_db", Kind: EdgeReaches, Weight: 0.7})
	var buf bytes.Buffer
	if err := g.WriteAdversaryView(&buf); err != nil {
		t.Fatalf("synthetic export failed: %v", err)
	}
	out := buf.String()
	// Defender-only fields must not appear in the adversary view.
	for _, leak := range []string{"risk", "crown_jewel", "boundary", "weight"} {
		if strings.Contains(out, leak) {
			t.Fatalf("adversary view leaked defender field %q: %s", leak, out)
		}
	}
}

func TestSeparationValidation(t *testing.T) {
	real := buildReal()
	syn := New(RealmSynthetic)
	syn.AddNode(Node{ID: "s_only", Kind: KindAsset, Label: "synthetic"})
	if err := ValidateSeparation(real, syn); err != nil {
		t.Fatalf("clean separation rejected: %v", err)
	}
	// Introduce an overlapping id → must be caught.
	syn.AddNode(Node{ID: "db", Kind: KindAsset, Label: "oops real id"})
	if err := ValidateSeparation(real, syn); err == nil {
		t.Fatal("overlapping node id between real and synthetic not caught")
	}
}

func TestBestPathPrefersEasiestRouteToJewel(t *testing.T) {
	g := buildReal()
	p, ok := g.BestPathToCrownJewels("edge")
	if !ok {
		t.Fatal("no path to crown jewel found")
	}
	if p.Nodes[0] != "edge" || p.Nodes[len(p.Nodes)-1] != "db" {
		t.Fatalf("path should run edge→…→db, got %v", p.Nodes)
	}
	// Direct api→db (0.5) beats api→cache→db (0.95*0.2=0.19), so the direct
	// route should be chosen.
	if len(p.Nodes) != 3 {
		t.Fatalf("expected direct edge→api→db (3 nodes), got %v", p.Nodes)
	}
	if p.Risk <= 0 || p.Risk > 1 {
		t.Fatalf("path risk out of range: %v", p.Risk)
	}
}
