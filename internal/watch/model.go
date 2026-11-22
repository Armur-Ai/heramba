package watch

import (
	"sort"
	"strconv"
	"time"

	"github.com/Armur-Ai/heramba/internal/event"
)

// State is the live aggregate the dashboard renders: it folds a stream of
// detection events into the handful of views an operator actually watches —
// who is hitting us (sources), how they cluster (campaigns / swarms), what
// deception they are walking into (decoy map), and the running tallies. It is
// pure data with no rendering, so it is cheap to test.
type State struct {
	Started         time.Time // when aggregation began (process time)
	First           time.Time // earliest event timestamp seen
	Total           int
	ByKind          map[event.Kind]int
	BySeverity      map[event.Severity]int
	HighestSeverity event.Severity
	Last            time.Time // latest event timestamp seen

	sources   map[string]*Source
	campaigns map[string]*Campaign
	decoys    map[string]*DecoyHit
	recent    []event.Event // newest at the end
	maxRecent int

	rate *timeSeries // per-second event-rate buckets for the header sparkline
}

// Source is everything we have learned about one client identity.
type Source struct {
	Addr        string
	Count       int
	Decoys      int
	Canaries    int
	Egress      int
	Agentic     int // number of agentic-traffic flags
	MaxScore    float64
	MaxSeverity event.Severity
	UA          string
	Campaign    string
	First       time.Time
	Last        time.Time
}

// Campaign clusters the sources that look like one coordinated effort. A
// campaign spanning two or more sources is a swarm — the signal that matters
// most on the dashboard, because a single autonomous attacker fanning out
// across identities is exactly what Heramba exists to catch.
type Campaign struct {
	ID          string
	Sources     map[string]struct{}
	Count       int
	Decoys      int
	MaxSeverity event.Severity
	First       time.Time
	Last        time.Time
}

// Swarm reports whether this campaign is a coordinated multi-source effort.
func (c *Campaign) Swarm() bool { return len(c.Sources) >= 2 }

// DecoyHit is a decoy/canary target and how often it has been touched.
type DecoyHit struct {
	Target string
	Kind   event.Kind
	Hits   int
	Last   time.Time
}

// NewState returns an empty aggregate that keeps the last maxRecent events for
// the live feed (<=0 means a sensible default).
func NewState(maxRecent int) *State {
	if maxRecent <= 0 {
		maxRecent = 200
	}
	return &State{
		Started:    time.Now(),
		ByKind:     map[event.Kind]int{},
		BySeverity: map[event.Severity]int{},
		sources:    map[string]*Source{},
		campaigns:  map[string]*Campaign{},
		decoys:     map[string]*DecoyHit{},
		maxRecent:  maxRecent,
		rate:       newTimeSeries(90, time.Second),
	}
}

// Add folds one event into every view.
func (s *State) Add(e event.Event) {
	s.Total++
	s.ByKind[e.Kind]++
	s.BySeverity[e.Severity]++
	if e.Severity > s.HighestSeverity {
		s.HighestSeverity = e.Severity
	}
	if e.Time.After(s.Last) {
		s.Last = e.Time
	}
	if !e.Time.IsZero() && (s.First.IsZero() || e.Time.Before(s.First)) {
		s.First = e.Time
	}

	if e.Source != "" {
		s.addSource(e)
		s.addCampaign(e)
	}
	if isDeception(e.Kind) && e.Target != "" {
		s.addDecoy(e)
	}

	s.recent = append(s.recent, e)
	if len(s.recent) > s.maxRecent {
		s.recent = s.recent[len(s.recent)-s.maxRecent:]
	}

	at := e.Time
	if at.IsZero() {
		at = time.Now()
	}
	s.rate.add(at, 1)
}

// Tick advances the event-rate window to `now` without recording an event, so
// the header sparkline keeps scrolling during quiet periods. The dashboard
// calls it on its one-second tick.
func (s *State) Tick(now time.Time) { s.rate.tick(now) }

// RateSeries returns the per-second event-rate buckets (oldest first) for the
// header sparkline. The caller must not mutate the slice.
func (s *State) RateSeries() []float64 { return s.rate.series() }

// RateNow is the number of events in the most recent one-second bucket.
func (s *State) RateNow() float64 { return s.rate.rate() }

// RatePeak is the busiest one-second bucket currently retained.
func (s *State) RatePeak() float64 { return s.rate.peak() }

func (s *State) addSource(e event.Event) {
	src := s.sources[e.Source]
	if src == nil {
		src = &Source{Addr: e.Source, First: e.Time}
		s.sources[e.Source] = src
	}
	src.Count++
	if e.Severity > src.MaxSeverity {
		src.MaxSeverity = e.Severity
	}
	if sc := scoreOf(e); sc > src.MaxScore {
		src.MaxScore = sc
	}
	if ua := e.Labels["ua"]; ua != "" {
		src.UA = ua
	}
	if c := campaignID(e); c != "" {
		src.Campaign = c
	}
	switch e.Kind {
	case event.KindDecoyHit:
		src.Decoys++
	case event.KindCanaryTriggered:
		src.Canaries++
	case event.KindEgressBlocked:
		src.Egress++
	case event.KindAgenticTraffic:
		src.Agentic++
	}
	if e.Time.After(src.Last) {
		src.Last = e.Time
	}
}

func (s *State) addCampaign(e event.Event) {
	id := campaignID(e)
	c := s.campaigns[id]
	if c == nil {
		c = &Campaign{ID: id, Sources: map[string]struct{}{}, First: e.Time}
		s.campaigns[id] = c
	}
	c.Sources[e.Source] = struct{}{}
	c.Count++
	if e.Severity > c.MaxSeverity {
		c.MaxSeverity = e.Severity
	}
	if isDeception(e.Kind) {
		c.Decoys++
	}
	if e.Time.After(c.Last) {
		c.Last = e.Time
	}
}

func (s *State) addDecoy(e event.Event) {
	d := s.decoys[e.Target]
	if d == nil {
		d = &DecoyHit{Target: e.Target, Kind: e.Kind}
		s.decoys[e.Target] = d
	}
	d.Hits++
	if e.Time.After(d.Last) {
		d.Last = e.Time
	}
}

// TopSources returns the n most urgent sources: highest severity first, then
// most active, then most recent.
func (s *State) TopSources(n int) []Source {
	out := make([]Source, 0, len(s.sources))
	for _, src := range s.sources {
		out = append(out, *src)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].MaxSeverity != out[j].MaxSeverity {
			return out[i].MaxSeverity > out[j].MaxSeverity
		}
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Last.After(out[j].Last)
	})
	return capSlice(out, n)
}

// Campaigns returns the most significant clusters: swarms first, then by volume.
func (s *State) Campaigns(n int) []Campaign {
	out := make([]Campaign, 0, len(s.campaigns))
	for _, c := range s.campaigns {
		out = append(out, *c)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Swarm() != out[j].Swarm() {
			return out[i].Swarm()
		}
		if len(out[i].Sources) != len(out[j].Sources) {
			return len(out[i].Sources) > len(out[j].Sources)
		}
		return out[i].Count > out[j].Count
	})
	return capSlice(out, n)
}

// TopDecoys returns the most-touched deception targets.
func (s *State) TopDecoys(n int) []DecoyHit {
	out := make([]DecoyHit, 0, len(s.decoys))
	for _, d := range s.decoys {
		out = append(out, *d)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Hits != out[j].Hits {
			return out[i].Hits > out[j].Hits
		}
		return out[i].Last.After(out[j].Last)
	})
	return capSlice(out, n)
}

// Recent returns up to n most-recent events, newest first.
func (s *State) Recent(n int) []event.Event {
	if n <= 0 || n > len(s.recent) {
		n = len(s.recent)
	}
	out := make([]event.Event, n)
	for i := 0; i < n; i++ {
		out[i] = s.recent[len(s.recent)-1-i]
	}
	return out
}

// SwarmCount is the number of active coordinated (multi-source) campaigns.
func (s *State) SwarmCount() int {
	n := 0
	for _, c := range s.campaigns {
		if c.Swarm() {
			n++
		}
	}
	return n
}

// Counts of the headline event kinds, for the summary bar.
func (s *State) DecoyHits() int    { return s.ByKind[event.KindDecoyHit] }
func (s *State) CanaryHits() int   { return s.ByKind[event.KindCanaryTriggered] }
func (s *State) EgressBlocks() int { return s.ByKind[event.KindEgressBlocked] }
func (s *State) AgenticFlags() int { return s.ByKind[event.KindAgenticTraffic] }

// SourceCount / CampaignCount are the unique-entity tallies.
func (s *State) SourceCount() int   { return len(s.sources) }
func (s *State) CampaignCount() int { return len(s.campaigns) }

func isDeception(k event.Kind) bool {
	return k == event.KindDecoyHit || k == event.KindCanaryTriggered
}

// campaignID clusters an event: an explicit campaign/actor label wins, else the
// source identity is its own single-member campaign.
func campaignID(e event.Event) string {
	if v := e.Labels["campaign"]; v != "" {
		return v
	}
	if v := e.Labels["actor"]; v != "" {
		return "actor:" + v
	}
	return "src:" + e.Source
}

// scoreOf reads an agentic score from labels, falling back to the event's own
// confidence (deception hits carry 1.0 by construction).
func scoreOf(e event.Event) float64 {
	for _, k := range []string{"agentic_score", "score"} {
		if v, ok := e.Labels[k]; ok {
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				return f
			}
		}
	}
	return e.Confidence
}

func capSlice[T any](s []T, n int) []T {
	if n > 0 && n < len(s) {
		return s[:n]
	}
	return s
}
