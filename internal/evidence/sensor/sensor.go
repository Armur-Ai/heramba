// Package sensor adds optional multi-sensor telemetry to Heramba's evidence
// plane: pluggable collectors normalize app, identity, and network signals into
// the same evidence.SecurityEvent vocabulary so they can be correlated into a
// single incident timeline.
//
// The design follows the mandate's reliability requirements: every collector
// reports health, a down sensor degrades gracefully (others keep flowing),
// duplicate reports of the same underlying event are collapsed by an
// idempotency key, timestamps are reconciled to UTC, and ingestion is bounded
// (backpressure) so a noisy sensor cannot exhaust memory.
package sensor

import (
	"sort"
	"sync"
	"time"

	"github.com/Armur-Ai/heramba/internal/event"
	"github.com/Armur-Ai/heramba/internal/evidence"
)

// Collector is a pluggable telemetry source. Collect returns a batch of
// normalized events (already carrying provenance with the collector's name).
// It must not block indefinitely; a failing collector returns an error and is
// marked unhealthy rather than stalling the pipeline.
type Collector interface {
	Name() string
	Collect() ([]evidence.SecurityEvent, error)
}

// Health is a collector's last-known status.
type Health struct {
	Name       string
	Healthy    bool
	LastOK     time.Time
	LastErr    string
	Collected  int
	Duplicates int
}

// Pipeline ingests from multiple collectors into an evidence.Store, collapsing
// duplicates, reconciling timestamps, tracking health, and enforcing a per-pass
// backpressure cap.
type Pipeline struct {
	mu         sync.Mutex
	store      *evidence.Store
	collectors []Collector
	health     map[string]*Health
	seen       map[string]struct{} // idempotency keys already ingested
	// MaxPerPass bounds how many events a single Ingest pass will accept from
	// any one collector (backpressure); extras are dropped with a count.
	MaxPerPass int
	dropped    int
}

// NewPipeline builds a pipeline over a store.
func NewPipeline(store *evidence.Store) *Pipeline {
	return &Pipeline{
		store:      store,
		health:     map[string]*Health{},
		seen:       map[string]struct{}{},
		MaxPerPass: 10000,
	}
}

// Add registers a collector.
func (p *Pipeline) Add(c Collector) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.collectors = append(p.collectors, c)
	p.health[c.Name()] = &Health{Name: c.Name(), Healthy: true}
}

// idempotencyKey collapses duplicate reports of the same underlying event. Two
// sensors reporting the same fact (same sensor-supplied raw ref, or the same
// kind+source+target+observation-second) are counted once.
func idempotencyKey(e evidence.SecurityEvent) string {
	if e.Provenance.RawRef != "" {
		return e.Provenance.Sensor + "|" + e.Provenance.RawRef
	}
	return string(e.Kind) + "|" + e.SourceKey + "|" + e.Target + "|" +
		e.Provenance.ObservedAt.UTC().Truncate(time.Second).Format(time.RFC3339)
}

// Ingest runs one collection pass over all collectors. A collector error marks
// it unhealthy but does not abort the pass (missing-data safety). Returns the
// number of new events ingested across all sensors.
func (p *Pipeline) Ingest() int {
	p.mu.Lock()
	collectors := append([]Collector(nil), p.collectors...)
	p.mu.Unlock()

	total := 0
	for _, c := range collectors {
		events, err := c.Collect()
		p.mu.Lock()
		h := p.health[c.Name()]
		if err != nil {
			h.Healthy = false
			h.LastErr = err.Error()
			p.mu.Unlock()
			continue
		}
		h.Healthy = true
		h.LastOK = time.Now().UTC()
		accepted := 0
		for _, e := range events {
			if accepted >= p.MaxPerPass {
				p.dropped += len(events) - accepted
				break
			}
			// reconcile timestamp to UTC
			e.Provenance.ObservedAt = e.Provenance.ObservedAt.UTC()
			key := idempotencyKey(e)
			if _, dup := p.seen[key]; dup {
				h.Duplicates++
				continue
			}
			if err := p.store.PutEvent(e); err != nil {
				// invalid event from a sensor: skip, note, keep going
				h.LastErr = "rejected event: " + err.Error()
				continue
			}
			p.seen[key] = struct{}{}
			h.Collected++
			accepted++
			total++
		}
		p.mu.Unlock()
	}
	return total
}

// Health returns a snapshot of every collector's health, name-sorted.
func (p *Pipeline) Health() []Health {
	p.mu.Lock()
	defer p.mu.Unlock()
	var out []Health
	for _, h := range p.health {
		out = append(out, *h)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Dropped reports how many events backpressure has shed.
func (p *Pipeline) Dropped() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.dropped
}

// TimelineEntry is one event in a correlated multi-sensor incident timeline.
type TimelineEntry struct {
	Time   time.Time
	Sensor string
	Kind   event.Kind
	Source string
	Target string
	Detail string
}

// IncidentTimeline returns the time-ordered events for a transport source key
// across ALL sensors — the single evidence-backed timeline the mandate asks
// for, fusing API, identity, and network signals.
func (p *Pipeline) IncidentTimeline(sourceKey string) []TimelineEntry {
	events := p.store.EventsBySource(sourceKey)
	var out []TimelineEntry
	for _, e := range events {
		out = append(out, TimelineEntry{
			Time:   e.Provenance.ObservedAt,
			Sensor: e.Provenance.Sensor,
			Kind:   e.Kind,
			Source: e.SourceKey,
			Target: e.Target,
			Detail: e.Detail,
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Time.Before(out[j].Time) })
	return out
}
