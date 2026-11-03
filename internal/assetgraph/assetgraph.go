// Package assetgraph maintains two strictly separated graphs: the defender's
// real authorized asset/workflow graph, and an adversary-visible synthetic
// graph that mirrors the deception world. The separation is the whole point —
// the real graph must never be exposed to an adversary-facing interface.
//
// That rule is enforced structurally, not by convention: a graph carries its
// Realm, and the only serialization path for adversary-facing output refuses to
// emit a Real graph. Risk scoring and path hypotheses run over either graph for
// the defender; only Synthetic graphs are ever rendered outward.
package assetgraph

import (
	"errors"
	"sort"
)

// Realm marks whether a graph describes real, authorized infrastructure (which
// stays defender-only) or the synthetic world shown to adversaries.
type Realm string

const (
	RealmReal      Realm = "real"
	RealmSynthetic Realm = "synthetic"
)

// NodeKind classifies a vertex.
type NodeKind string

const (
	KindAsset    NodeKind = "asset"    // service, datastore, host
	KindIdentity NodeKind = "identity" // user, role, service account
	KindAPI      NodeKind = "api"      // an endpoint / operation
	KindWorkflow NodeKind = "workflow" // a multi-step business process
)

// EdgeKind classifies a directed relationship.
type EdgeKind string

const (
	EdgeOwns        EdgeKind = "owns"
	EdgeMemberOf    EdgeKind = "member_of"
	EdgeCalls       EdgeKind = "calls"
	EdgeTrusts      EdgeKind = "trusts"
	EdgeTransitions EdgeKind = "transitions_to"
	EdgeReaches     EdgeKind = "reaches" // network/logical reachability
)

// Node is a vertex with a risk weight and an optional trust-boundary tag.
type Node struct {
	ID         string   `json:"id"`
	Kind       NodeKind `json:"kind"`
	Label      string   `json:"label"`
	Boundary   string   `json:"boundary,omitempty"` // trust-zone name
	Risk       float64  `json:"risk"`               // intrinsic risk weight [0,1]
	CrownJewel bool     `json:"crown_jewel,omitempty"`
}

// Edge is a directed relationship with a traversal weight.
type Edge struct {
	From   string   `json:"from"`
	To     string   `json:"to"`
	Kind   EdgeKind `json:"kind"`
	Weight float64  `json:"weight"` // ease of traversal [0,1]; higher = easier
}

// Graph is a realm-tagged asset/workflow graph.
type Graph struct {
	Realm Realm           `json:"realm"`
	Nodes map[string]Node `json:"nodes"`
	out   map[string][]Edge
}

// New returns an empty graph in the given realm.
func New(realm Realm) *Graph {
	return &Graph{Realm: realm, Nodes: map[string]Node{}, out: map[string][]Edge{}}
}

// AddNode inserts or replaces a node.
func (g *Graph) AddNode(n Node) { g.Nodes[n.ID] = n }

// AddEdge inserts a directed edge (both endpoints must exist).
func (g *Graph) AddEdge(e Edge) error {
	if _, ok := g.Nodes[e.From]; !ok {
		return errors.New("assetgraph: edge from unknown node " + e.From)
	}
	if _, ok := g.Nodes[e.To]; !ok {
		return errors.New("assetgraph: edge to unknown node " + e.To)
	}
	g.out[e.From] = append(g.out[e.From], e)
	return nil
}

// Out returns edges leaving a node.
func (g *Graph) Out(id string) []Edge { return g.out[id] }

// CrossesBoundary reports whether an edge crosses a trust boundary.
func (g *Graph) CrossesBoundary(e Edge) bool {
	a, b := g.Nodes[e.From], g.Nodes[e.To]
	return a.Boundary != b.Boundary
}

// CrownJewels returns the IDs of crown-jewel nodes, sorted.
func (g *Graph) CrownJewels() []string {
	var out []string
	for id, n := range g.Nodes {
		if n.CrownJewel {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}
