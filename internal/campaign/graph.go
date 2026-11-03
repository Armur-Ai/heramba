package campaign

import (
	"sort"
	"time"

	"github.com/Armur-Ai/heramba/internal/evidence"
)

// NodeKind labels a node in the campaign graph.
type NodeKind string

const (
	NodeEvent    NodeKind = "event"
	NodeSession  NodeKind = "session"
	NodeActor    NodeKind = "actor"
	NodeCampaign NodeKind = "campaign"
)

// Node is a vertex in the temporal campaign graph. It holds just enough to
// navigate and explain; the full record lives in the evidence store / the
// Session/Actor/Campaign maps.
type Node struct {
	ID    evidence.ID `json:"id"`
	Kind  NodeKind    `json:"kind"`
	Label string      `json:"label"`
	Time  time.Time   `json:"time"`
}

// Graph is the temporal graph: typed nodes and typed directed edges
// (evidence.Relationship), with adjacency for walking. It is the structure the
// investigation API queries. It distinguishes observational edges from
// inference edges so an explanation can mark which links are inferred.
type Graph struct {
	nodes map[evidence.ID]Node
	out   map[evidence.ID][]evidence.Relationship
	in    map[evidence.ID][]evidence.Relationship
}

// NewGraph returns an empty graph.
func NewGraph() *Graph {
	return &Graph{
		nodes: map[evidence.ID]Node{},
		out:   map[evidence.ID][]evidence.Relationship{},
		in:    map[evidence.ID][]evidence.Relationship{},
	}
}

// AddNode inserts or replaces a node.
func (g *Graph) AddNode(n Node) { g.nodes[n.ID] = n }

// AddEdge inserts a typed edge (no-op if invalid).
func (g *Graph) AddEdge(r evidence.Relationship) error {
	if err := r.Validate(); err != nil {
		return err
	}
	g.out[r.From] = append(g.out[r.From], r)
	g.in[r.To] = append(g.in[r.To], r)
	return nil
}

// Node returns a node by ID.
func (g *Graph) Node(id evidence.ID) (Node, bool) { n, ok := g.nodes[id]; return n, ok }

// NodeCount / EdgeCount report graph size (edge count counts each edge once).
func (g *Graph) NodeCount() int { return len(g.nodes) }
func (g *Graph) EdgeCount() int {
	n := 0
	for _, es := range g.out {
		n += len(es)
	}
	return n
}

// Out returns edges leaving a node.
func (g *Graph) Out(id evidence.ID) []evidence.Relationship { return g.out[id] }

// In returns edges entering a node.
func (g *Graph) In(id evidence.ID) []evidence.Relationship { return g.in[id] }

// NodesByKind returns all nodes of a kind, time-sorted.
func (g *Graph) NodesByKind(k NodeKind) []Node {
	var out []Node
	for _, n := range g.nodes {
		if n.Kind == k {
			out = append(out, n)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Time.Before(out[j].Time) })
	return out
}
