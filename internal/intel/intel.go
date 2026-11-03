// Package intel is the bridge between Heramba's data plane and its intelligence
// plane. It implements the alert.Sink interface, so it can be fanned into the
// live edge alongside the text/JSONL sinks: every detection the edge emits is
// lifted into the normalized evidence store (internal/evidence) and becomes
// available for campaign reconstruction (internal/campaign) — without changing
// the hot path or the operator's existing alert output.
//
// This is the integration seam that makes the capabilities real end-to-end:
// data-plane detections flow into evidence, evidence into sessions/actors/
// campaigns, and from there into the strategy planner and the swarm detector.
package intel

import (
	"sync"

	"github.com/Armur-Ai/heramba/internal/campaign"
	"github.com/Armur-Ai/heramba/internal/campaign/swarm"
	"github.com/Armur-Ai/heramba/internal/event"
	"github.com/Armur-Ai/heramba/internal/evidence"
)

// Ingestor lifts data-plane events into the evidence store and reconstructs
// campaigns on demand. It is safe for concurrent Emit calls from the edge.
type Ingestor struct {
	mu     sync.Mutex
	store  *evidence.Store
	sensor string
	seen   int
	lifted int
}

// NewIngestor builds an ingestor over a store, tagging provenance with sensor.
func NewIngestor(store *evidence.Store, sensor string) *Ingestor {
	if store == nil {
		store = evidence.NewStore()
	}
	if sensor == "" {
		sensor = "edge"
	}
	return &Ingestor{store: store, sensor: sensor}
}

// Emit implements alert.Sink. It lifts the detection into a SecurityEvent and
// stores it. A lift that fails validation is dropped silently (the detection
// still reaches the other sinks); intel must never break the alert path.
func (i *Ingestor) Emit(e event.Event) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.seen++
	se := evidence.FromDetection(e, i.sensor, "")
	if se.Validate() != nil {
		return nil
	}
	if err := i.store.PutEvent(se); err != nil {
		return nil
	}
	i.lifted++
	return nil
}

// Store exposes the underlying evidence store.
func (i *Ingestor) Store() *evidence.Store { return i.store }

// Stats reports how many detections were seen and how many were lifted.
func (i *Ingestor) Stats() (seen, lifted int) {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.seen, i.lifted
}

// Reconstruct runs campaign correlation over everything ingested so far.
func (i *Ingestor) Reconstruct() *campaign.Result {
	return campaign.DefaultCorrelator().Reconstruct(i.store)
}

// Summary is a compact intelligence rollup for an operator at shutdown.
type Summary struct {
	Events    int
	Sessions  int
	Actors    int
	Campaigns int
	// SwarmClasses counts how many reconstructed actors' sessions, grouped per
	// actor, classify into each coordination class.
	SwarmClasses map[swarm.Class]int
}

// Summarize reconstructs and classifies the current intelligence state.
func (i *Ingestor) Summarize() Summary {
	res := i.Reconstruct()
	s := Summary{
		Sessions:     len(res.Sessions),
		Actors:       len(res.Actors),
		Campaigns:    len(res.Campaigns),
		SwarmClasses: map[swarm.Class]int{},
	}
	ev, _, _, _ := i.store.Counts()
	s.Events = ev
	det := swarm.DefaultDetector()
	for _, a := range res.Actors {
		var sessions []campaign.Session
		for _, sid := range a.SessionIDs {
			if sess, ok := res.Sessions[sid]; ok {
				sessions = append(sessions, sess)
			}
		}
		if len(sessions) == 0 {
			continue
		}
		v := det.Classify(sessions)
		s.SwarmClasses[v.Class]++
	}
	return s
}
