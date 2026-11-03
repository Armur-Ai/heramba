package assetgraph

import (
	"encoding/json"
	"errors"
	"io"
)

// ErrRealGraphExposure is returned if anything attempts to serialize a real
// asset graph to an adversary-facing sink. This is the structural guarantee
// that the defender's real topology never leaks outward.
var ErrRealGraphExposure = errors.New("assetgraph: refusing to expose a REAL graph to an adversary-facing interface")

// AdversaryView is the sanitized, adversary-facing projection of a graph. It
// deliberately drops defender-only fields (intrinsic risk, crown-jewel flags,
// trust-boundary names) so that even a synthetic graph reveals no defender
// reasoning — the attacker sees topology, not our valuation of it.
type AdversaryView struct {
	Realm Realm           `json:"realm"`
	Nodes []adversaryNode `json:"nodes"`
	Edges []adversaryEdge `json:"edges"`
}

type adversaryNode struct {
	ID    string   `json:"id"`
	Kind  NodeKind `json:"kind"`
	Label string   `json:"label"`
}

type adversaryEdge struct {
	From string   `json:"from"`
	To   string   `json:"to"`
	Kind EdgeKind `json:"kind"`
}

// WriteAdversaryView serializes a graph for an adversary-facing interface. It
// FAILS CLOSED on a real graph: the only graphs that may ever be rendered
// outward are synthetic.
func (g *Graph) WriteAdversaryView(w io.Writer) error {
	if g.Realm != RealmSynthetic {
		return ErrRealGraphExposure
	}
	view := AdversaryView{Realm: g.Realm}
	for id, n := range g.Nodes {
		view.Nodes = append(view.Nodes, adversaryNode{ID: id, Kind: n.Kind, Label: n.Label})
		for _, e := range g.out[id] {
			view.Edges = append(view.Edges, adversaryEdge{From: e.From, To: e.To, Kind: e.Kind})
		}
	}
	return json.NewEncoder(w).Encode(view)
}

// ValidateSeparation checks that a real and a synthetic graph share no node
// IDs — a cross-contamination guard run in tests and at startup. Overlapping
// IDs would risk a synthetic lure accidentally referencing a real asset.
func ValidateSeparation(real, synthetic *Graph) error {
	if real.Realm != RealmReal {
		return errors.New("assetgraph: first graph is not REAL")
	}
	if synthetic.Realm != RealmSynthetic {
		return errors.New("assetgraph: second graph is not SYNTHETIC")
	}
	for id := range synthetic.Nodes {
		if _, ok := real.Nodes[id]; ok {
			return errors.New("assetgraph: synthetic graph shares node id with real graph: " + id)
		}
	}
	return nil
}
