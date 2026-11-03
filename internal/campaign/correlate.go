package campaign

import (
	"fmt"
	"sort"
	"time"

	"github.com/Armur-Ai/heramba/internal/evidence"
)

// Campaign is an inferred cluster of actors pursuing a coordinated objective
// over a time window — e.g. several sources enumerating the same surface in
// parallel. Like an Actor link, a Campaign is a Hypothesis, not a fact.
type Campaign struct {
	ID         evidence.ID          `json:"id"`
	ActorIDs   []evidence.ID        `json:"actor_ids"`
	Start      time.Time            `json:"start"`
	End        time.Time            `json:"end"`
	Targets    map[string]int       `json:"targets"`
	Hypothesis *evidence.Hypothesis `json:"hypothesis"`
}

// Result is the full reconstruction: the graph plus the derived sessions,
// actors, and campaigns, keyed for lookup.
type Result struct {
	Graph     *Graph
	Sessions  map[evidence.ID]Session
	Actors    map[evidence.ID]*Actor
	Campaigns map[evidence.ID]*Campaign
}

// Correlator reconstructs campaigns from an evidence stream: sessionize →
// resolve entities → build graph → cluster campaigns. It is deterministic given
// the same events and configuration.
type Correlator struct {
	Sessionizer Sessionizer
	Resolver    *Resolver
	// CampaignOverlap: actors whose active windows overlap and who share at
	// least MinSharedTargets targets are hypothesized to be one campaign.
	MinSharedTargets int
	CampaignConf     evidence.Confidence
}

// DefaultCorrelator returns a correlator with calibrated defaults.
func DefaultCorrelator() *Correlator {
	return &Correlator{
		Sessionizer:      DefaultSessionizer(),
		Resolver:         DefaultResolver(),
		MinSharedTargets: 1,
		CampaignConf:     0.55,
	}
}

// Reconstruct runs the full pipeline over the store's events.
func (c *Correlator) Reconstruct(store *evidence.Store) *Result {
	events := store.EventsInWindow(time.Time{}, time.Now().UTC().Add(time.Hour))
	return c.ReconstructEvents(events)
}

// ReconstructEvents runs the pipeline over an explicit event slice (used by
// tests and replay).
func (c *Correlator) ReconstructEvents(events []evidence.SecurityEvent) *Result {
	g := NewGraph()
	res := &Result{
		Graph:     g,
		Sessions:  map[evidence.ID]Session{},
		Actors:    map[evidence.ID]*Actor{},
		Campaigns: map[evidence.ID]*Campaign{},
	}

	// Event nodes.
	for _, e := range events {
		g.AddNode(Node{ID: e.ID, Kind: NodeEvent,
			Label: string(e.Kind) + " " + e.Target, Time: e.Provenance.ObservedAt})
	}

	// Sessionize (observation) and link same-source events.
	sessions := c.Sessionizer.Sessionize(events)
	for _, s := range sessions {
		res.Sessions[s.ID] = s
		g.AddNode(Node{ID: s.ID, Kind: NodeSession,
			Label: "session " + s.SourceKey, Time: s.Start})
		for _, eid := range s.EventIDs {
			// event part_of session (observational, conf 1.0)
			_ = g.AddEdge(evidence.NewRelationship(evidence.RelPartOf, eid, s.ID, 1.0))
		}
	}

	// Entity resolution (inference): assign sessions to actors in time order.
	for _, s := range sessions {
		a, conf := c.Resolver.Assign(s)
		// actor part_of edge for the session, confidence = link confidence.
		_ = g.AddEdge(evidence.NewRelationship(evidence.RelSameActor, s.ID, a.ID, conf))
	}
	for _, a := range c.Resolver.Actors() {
		res.Actors[a.ID] = a
		g.AddNode(Node{ID: a.ID, Kind: NodeActor,
			Label: fmt.Sprintf("actor (%d sources)", len(a.SourceKeys)), Time: a.FirstSeen})
	}

	// Campaign clustering (inference).
	c.clusterCampaigns(res)
	return res
}

// clusterCampaigns groups actors with overlapping active windows and shared
// targets into campaigns. Simple, explainable union-find over a pairwise
// overlap test. (Full coordination/motif analysis is CP6.)
func (c *Correlator) clusterCampaigns(res *Result) {
	actors := make([]*Actor, 0, len(res.Actors))
	for _, a := range res.Actors {
		actors = append(actors, a)
	}
	sort.SliceStable(actors, func(i, j int) bool { return actors[i].FirstSeen.Before(actors[j].FirstSeen) })

	parent := map[evidence.ID]evidence.ID{}
	var find func(evidence.ID) evidence.ID
	find = func(x evidence.ID) evidence.ID {
		if parent[x] == "" {
			parent[x] = x
		}
		if parent[x] != x {
			parent[x] = find(parent[x])
		}
		return parent[x]
	}
	union := func(a, b evidence.ID) { parent[find(a)] = find(b) }

	actorTargets := map[evidence.ID]map[string]int{}
	for _, a := range actors {
		tg := map[string]int{}
		for _, sid := range a.SessionIDs {
			for t, n := range res.Sessions[sid].Targets {
				tg[t] += n
			}
		}
		actorTargets[a.ID] = tg
		find(a.ID)
	}

	for i := 0; i < len(actors); i++ {
		for j := i + 1; j < len(actors); j++ {
			a, b := actors[i], actors[j]
			if !windowsOverlap(a, b) {
				continue
			}
			if sharedTargets(actorTargets[a.ID], actorTargets[b.ID]) >= c.MinSharedTargets {
				union(a.ID, b.ID)
			}
		}
	}

	groups := map[evidence.ID][]*Actor{}
	for _, a := range actors {
		root := find(a.ID)
		groups[root] = append(groups[root], a)
	}
	for _, members := range groups {
		if len(members) < 2 {
			continue // a lone actor is not yet a campaign
		}
		camp := &Campaign{
			ID:      evidence.NewID("camp"),
			Targets: map[string]int{},
		}
		for _, a := range members {
			camp.ActorIDs = append(camp.ActorIDs, a.ID)
			if camp.Start.IsZero() || a.FirstSeen.Before(camp.Start) {
				camp.Start = a.FirstSeen
			}
			if a.LastSeen.After(camp.End) {
				camp.End = a.LastSeen
			}
			for t, n := range actorTargets[a.ID] {
				camp.Targets[t] += n
			}
		}
		camp.Hypothesis = evidence.NewHypothesis(
			fmt.Sprintf("%d actors are one coordinated campaign", len(members)),
			c.CampaignConf, 48*time.Hour)
		camp.Hypothesis.Stage = evidence.StageRecon
		res.Campaigns[camp.ID] = camp
		res.Graph.AddNode(Node{ID: camp.ID, Kind: NodeCampaign,
			Label: fmt.Sprintf("campaign (%d actors)", len(members)), Time: camp.Start})
		for _, a := range members {
			_ = res.Graph.AddEdge(evidence.NewRelationship(
				evidence.RelPartOf, a.ID, camp.ID, c.CampaignConf))
		}
	}
}

func windowsOverlap(a, b *Actor) bool {
	return !a.FirstSeen.After(b.LastSeen) && !b.FirstSeen.After(a.LastSeen)
}

func sharedTargets(a, b map[string]int) int {
	n := 0
	for t := range a {
		if _, ok := b[t]; ok {
			n++
		}
	}
	return n
}
