package assetgraph

import (
	"math"
	"sort"
)

// AttackPath is a hypothesized route from an entry node to a crown jewel, with a
// per-step explanation and an aggregate risk. It is a defender-side hypothesis
// used to prioritize hardening and to design plausible synthetic lures — never
// a claim that the route was taken.
type AttackPath struct {
	Nodes []string `json:"nodes"`
	Edges []Edge   `json:"edges"`
	Risk  float64  `json:"risk"` // product of edge weights × target risk, [0,1]
}

// BestPathToCrownJewels finds the highest-risk path from entry to any
// crown-jewel node. "Risk" multiplies edge traversal weights by the target's
// intrinsic risk, so the easiest route to the most valuable asset wins. It uses
// a max-product search (Dijkstra-like over log-weights) and is deterministic.
func (g *Graph) BestPathToCrownJewels(entry string) (AttackPath, bool) {
	jewels := map[string]bool{}
	for _, id := range g.CrownJewels() {
		jewels[id] = true
	}
	if len(jewels) == 0 {
		return AttackPath{}, false
	}
	return g.bestPath(entry, jewels)
}

// bestPath returns the max-product path from entry to the best reachable goal.
func (g *Graph) bestPath(entry string, goals map[string]bool) (AttackPath, bool) {
	if _, ok := g.Nodes[entry]; !ok {
		return AttackPath{}, false
	}
	// Work in negative-log space so "max product" becomes "min sum".
	dist := map[string]float64{entry: 0}
	prevNode := map[string]string{}
	prevEdge := map[string]Edge{}
	visited := map[string]bool{}

	for {
		// pick unvisited node with smallest dist (simple O(V^2); graphs are small)
		cur := ""
		best := math.Inf(1)
		for id, d := range dist {
			if !visited[id] && d < best {
				best, cur = d, id
			}
		}
		if cur == "" {
			break
		}
		visited[cur] = true
		for _, e := range g.out[cur] {
			w := e.Weight
			if w <= 0 {
				w = 0.01 // avoid log(0); a near-impossible but non-zero step
			}
			nd := dist[cur] - math.Log(w)
			if d, ok := dist[e.To]; !ok || nd < d {
				dist[e.To] = nd
				prevNode[e.To] = cur
				prevEdge[e.To] = e
			}
		}
	}

	// choose the reachable goal with the highest path-risk
	bestGoal := ""
	bestRisk := -1.0
	for goal := range goals {
		d, ok := dist[goal]
		if !ok {
			continue
		}
		edgeProd := math.Exp(-d)              // product of edge weights
		risk := edgeProd * g.Nodes[goal].Risk // weight by target value
		if risk > bestRisk {
			bestRisk, bestGoal = risk, goal
		}
	}
	if bestGoal == "" {
		return AttackPath{}, false
	}

	// reconstruct
	var nodes []string
	var edges []Edge
	for n := bestGoal; n != ""; {
		nodes = append([]string{n}, nodes...)
		e, ok := prevEdge[n]
		if !ok {
			break
		}
		edges = append([]Edge{e}, edges...)
		n = prevNode[n]
	}
	return AttackPath{Nodes: nodes, Edges: edges, Risk: bestRisk}, true
}

// RiskRanking returns all crown jewels ranked by best-path risk from an entry,
// a defender prioritization aid.
func (g *Graph) RiskRanking(entry string) []AttackPath {
	var paths []AttackPath
	for _, id := range g.CrownJewels() {
		if p, ok := g.bestPath(entry, map[string]bool{id: true}); ok {
			paths = append(paths, p)
		}
	}
	sort.SliceStable(paths, func(i, j int) bool { return paths[i].Risk > paths[j].Risk })
	return paths
}
