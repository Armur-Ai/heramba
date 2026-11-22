package dashboard

import (
	"sort"
	"time"

	"github.com/Armur-Ai/heramba/internal/event"
)

// sevLabel returns an event's severity label, preferring the precomputed one.
func sevLabel(e event.Event) string {
	if e.SeverityS != "" {
		return e.SeverityS
	}
	return e.Severity.String()
}

// --- Timeline -------------------------------------------------------------

// TimelinePoint is one time bucket: a bucket-start timestamp and the event
// counts that fell in it, overall and by severity.
type TimelinePoint struct {
	T          int64          `json:"t"` // unix seconds, bucket start
	Total      int            `json:"total"`
	BySeverity map[string]int `json:"by_severity"`
}

// Timeline is a bucketed view of events over a trailing window, for the
// events-over-time chart. Points are fixed-width and contiguous (including
// empty buckets) so the x-axis is stable.
type Timeline struct {
	WindowSeconds int             `json:"window_seconds"`
	BucketSeconds int             `json:"bucket_seconds"`
	Points        []TimelinePoint `json:"points"`
}

// timelineBuckets is the target number of buckets across the window.
const timelineBuckets = 60

// parseWindow parses a ?window= duration ("15m", "1h", "24h"), clamped to a
// sane range, defaulting to 15m.
func parseWindow(q string) time.Duration {
	const def = 15 * time.Minute
	if q == "" {
		return def
	}
	d, err := time.ParseDuration(q)
	if err != nil || d <= 0 {
		return def
	}
	if d < time.Minute {
		d = time.Minute
	}
	if d > 7*24*time.Hour {
		d = 7 * 24 * time.Hour
	}
	return d
}

// buildTimeline buckets events over the trailing window. The window ends at
// "now", or at the latest event time when that is in the future of now, so a
// freshly synthesized stream is always visible.
func buildTimeline(evs []event.Event, window time.Duration) Timeline {
	end := time.Now().UTC()
	for _, e := range evs {
		if e.Time.After(end) {
			end = e.Time
		}
	}
	bucket := window / timelineBuckets
	if bucket < time.Second {
		bucket = time.Second
	}
	return buildTimelineAt(evs, window, bucket, end)
}

// buildTimelineAt is the deterministic core of buildTimeline: it buckets events
// into fixed-width buckets over [end-N*bucket, end], with N = window/bucket.
// Events at or after end are folded into the final bucket; events before the
// window start are ignored.
func buildTimelineAt(evs []event.Event, window, bucket time.Duration, end time.Time) Timeline {
	n := int(window / bucket)
	if n < 1 {
		n = 1
	}
	start := end.Add(-time.Duration(n) * bucket)

	points := make([]TimelinePoint, n)
	for i := range points {
		points[i] = TimelinePoint{
			T:          start.Add(time.Duration(i) * bucket).Unix(),
			BySeverity: map[string]int{},
		}
	}
	for _, e := range evs {
		if e.Time.Before(start) {
			continue
		}
		idx := int(e.Time.Sub(start) / bucket)
		if idx < 0 {
			continue
		}
		if idx >= n {
			idx = n - 1 // fold the newest events into the last bucket
		}
		points[idx].Total++
		points[idx].BySeverity[sevLabel(e)]++
	}
	return Timeline{
		WindowSeconds: int(window.Seconds()),
		BucketSeconds: int(bucket.Seconds()),
		Points:        points,
	}
}

// --- Sources --------------------------------------------------------------

// SourceAgg aggregates everything seen from one source (client IP / identity).
type SourceAgg struct {
	Source      string         `json:"source"`
	Count       int            `json:"count"`
	Kinds       map[string]int `json:"kinds"`
	MaxSeverity string         `json:"max_severity"`
	maxSevRank  event.Severity
	MaxConf     float64   `json:"max_confidence"`
	LastSeen    time.Time `json:"last_seen"`
}

// topSourcesLimit caps how many sources the table returns.
const topSourcesLimit = 50

// topSources aggregates events per source, worst (most active) first.
func topSources(evs []event.Event) []SourceAgg {
	byID := map[string]*SourceAgg{}
	for _, e := range evs {
		if e.Source == "" {
			continue
		}
		a := byID[e.Source]
		if a == nil {
			a = &SourceAgg{Source: e.Source, Kinds: map[string]int{}}
			byID[e.Source] = a
		}
		a.Count++
		a.Kinds[string(e.Kind)]++
		if e.Severity >= a.maxSevRank {
			a.maxSevRank = e.Severity
			a.MaxSeverity = sevLabel(e)
		}
		if e.Confidence > a.MaxConf {
			a.MaxConf = e.Confidence
		}
		if e.Time.After(a.LastSeen) {
			a.LastSeen = e.Time
		}
	}
	out := make([]SourceAgg, 0, len(byID))
	for _, a := range byID {
		out = append(out, *a)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Source < out[j].Source
	})
	if len(out) > topSourcesLimit {
		out = out[:topSourcesLimit]
	}
	if out == nil {
		out = []SourceAgg{}
	}
	return out
}

// --- Per-source drill -----------------------------------------------------

// sourceDrillLimit caps how many individual events a source drill returns.
const sourceDrillLimit = 200

// SourceDetail is the drill-down for a single source: its rolled-up stats plus
// the matching events, newest first. Events is never nil.
type SourceDetail struct {
	Source      string         `json:"source"`
	Count       int            `json:"count"`
	Kinds       map[string]int `json:"kinds"`
	Targets     map[string]int `json:"targets"`
	MaxSeverity string         `json:"max_severity"`
	MaxConf     float64        `json:"max_confidence"`
	FirstSeen   time.Time      `json:"first_seen"`
	LastSeen    time.Time      `json:"last_seen"`
	Events      []event.Event  `json:"events"`
}

// sourceDrill filters events down to one source and rolls up its activity. The
// returned Events are newest first and capped at sourceDrillLimit; an unknown
// source yields a zero-count detail (not nil), so the UI can render an empty
// state rather than erroring.
func sourceDrill(evs []event.Event, source string) SourceDetail {
	d := SourceDetail{
		Source:  source,
		Kinds:   map[string]int{},
		Targets: map[string]int{},
		Events:  []event.Event{},
	}
	var maxSevRank event.Severity
	for _, e := range evs {
		if e.Source != source {
			continue
		}
		d.Count++
		d.Kinds[string(e.Kind)]++
		if e.Target != "" {
			d.Targets[e.Target]++
		}
		if e.Severity >= maxSevRank {
			maxSevRank = e.Severity
			d.MaxSeverity = sevLabel(e)
		}
		if e.Confidence > d.MaxConf {
			d.MaxConf = e.Confidence
		}
		if d.FirstSeen.IsZero() || e.Time.Before(d.FirstSeen) {
			d.FirstSeen = e.Time
		}
		if e.Time.After(d.LastSeen) {
			d.LastSeen = e.Time
		}
		if len(d.Events) < sourceDrillLimit {
			d.Events = append(d.Events, e)
		}
	}
	// recentEvents already returns newest-first; preserve that ordering.
	return d
}

// --- Confirmed breaches (canary triggers) ---------------------------------

// breachLimit caps how many recent breach events are echoed back.
const breachLimit = 50

// Breaches summarises confirmed breaches — triggered canary tokens. A canary
// trigger is the highest-fidelity signal the edge emits: nobody legitimately
// replays a planted token, so each one is a confirmed intrusion. Recent is
// newest first and never nil.
type Breaches struct {
	Count    int            `json:"count"`
	Sources  int            `json:"sources"`
	Targets  map[string]int `json:"targets"`
	LastSeen time.Time      `json:"last_seen"`
	Recent   []event.Event  `json:"recent"`
}

// confirmedBreaches counts canary-trigger events and surfaces the most recent
// ones. Input is assumed newest-first (as recentEvents returns).
func confirmedBreaches(evs []event.Event) Breaches {
	b := Breaches{Targets: map[string]int{}, Recent: []event.Event{}}
	srcSet := map[string]struct{}{}
	for _, e := range evs {
		if e.Kind != event.KindCanaryTriggered {
			continue
		}
		b.Count++
		if e.Target != "" {
			b.Targets[e.Target]++
		}
		if e.Source != "" {
			srcSet[e.Source] = struct{}{}
		}
		if e.Time.After(b.LastSeen) {
			b.LastSeen = e.Time
		}
		if len(b.Recent) < breachLimit {
			b.Recent = append(b.Recent, e)
		}
	}
	b.Sources = len(srcSet)
	return b
}

// --- Campaigns / swarms ---------------------------------------------------

// Campaign is a coordinated cluster of activity — the same target (or an
// explicit campaign label) hit by one or more sources. Coordinated is set when
// three or more distinct sources participate, the signature of a swarm.
type Campaign struct {
	ID          string         `json:"id"`
	Target      string         `json:"target"`
	Sources     []string       `json:"sources"`
	SourceCount int            `json:"source_count"`
	Count       int            `json:"count"`
	Kinds       map[string]int `json:"kinds"`
	MaxSeverity string         `json:"max_severity"`
	maxSevRank  event.Severity
	FirstSeen   time.Time `json:"first_seen"`
	LastSeen    time.Time `json:"last_seen"`
	Coordinated bool      `json:"coordinated"`
}

// coordinatedThreshold is the distinct-source count at which a cluster is
// flagged as a coordinated swarm.
const coordinatedThreshold = 3

// detectCampaigns groups events into campaigns. Events carrying a "campaign"
// label are grouped by it; the rest are grouped by target. Only multi-source
// clusters (>= 2 distinct sources) are returned — single-source noise is
// filtered out — sorted by distinct-source count then volume.
func detectCampaigns(evs []event.Event) []Campaign {
	type group struct {
		c       *Campaign
		srcSet  map[string]struct{}
		srcList []string
	}
	groups := map[string]*group{}
	for _, e := range evs {
		key := ""
		if e.Labels != nil {
			key = e.Labels["campaign"]
		}
		labeled := key != ""
		if key == "" {
			key = "target:" + e.Target
		}
		g := groups[key]
		if g == nil {
			id := key
			if labeled {
				id = e.Labels["campaign"]
			}
			g = &group{
				c:      &Campaign{ID: id, Target: e.Target, Kinds: map[string]int{}, FirstSeen: e.Time, LastSeen: e.Time},
				srcSet: map[string]struct{}{},
			}
			groups[key] = g
		}
		c := g.c
		c.Count++
		c.Kinds[string(e.Kind)]++
		if e.Severity >= c.maxSevRank {
			c.maxSevRank = e.Severity
			c.MaxSeverity = sevLabel(e)
		}
		if e.Time.Before(c.FirstSeen) {
			c.FirstSeen = e.Time
		}
		if e.Time.After(c.LastSeen) {
			c.LastSeen = e.Time
		}
		if e.Source != "" {
			if _, ok := g.srcSet[e.Source]; !ok {
				g.srcSet[e.Source] = struct{}{}
				g.srcList = append(g.srcList, e.Source)
			}
		}
	}
	out := make([]Campaign, 0, len(groups))
	for _, g := range groups {
		g.c.SourceCount = len(g.srcList)
		if g.c.SourceCount < 2 {
			continue
		}
		sort.Strings(g.srcList)
		g.c.Sources = g.srcList
		g.c.Coordinated = g.c.SourceCount >= coordinatedThreshold
		out = append(out, *g.c)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].SourceCount != out[j].SourceCount {
			return out[i].SourceCount > out[j].SourceCount
		}
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].ID < out[j].ID
	})
	if out == nil {
		out = []Campaign{}
	}
	return out
}

// --- Decoys ---------------------------------------------------------------

// DecoyAgg aggregates hits against one decoy/canary target — the hit-map rows.
type DecoyAgg struct {
	Target      string         `json:"target"`
	Count       int            `json:"count"`
	SourceCount int            `json:"source_count"`
	Kinds       map[string]int `json:"kinds"`
	MaxSeverity string         `json:"max_severity"`
	maxSevRank  event.Severity
	LastSeen    time.Time `json:"last_seen"`
}

// topDecoysLimit caps how many decoy rows are returned.
const topDecoysLimit = 50

// topDecoys aggregates events per target (decoy path / canary token), most hit
// first. Events with no target are ignored.
func topDecoys(evs []event.Event) []DecoyAgg {
	type agg struct {
		d      *DecoyAgg
		srcSet map[string]struct{}
	}
	byTarget := map[string]*agg{}
	for _, e := range evs {
		if e.Target == "" {
			continue
		}
		a := byTarget[e.Target]
		if a == nil {
			a = &agg{d: &DecoyAgg{Target: e.Target, Kinds: map[string]int{}}, srcSet: map[string]struct{}{}}
			byTarget[e.Target] = a
		}
		d := a.d
		d.Count++
		d.Kinds[string(e.Kind)]++
		if e.Severity >= d.maxSevRank {
			d.maxSevRank = e.Severity
			d.MaxSeverity = sevLabel(e)
		}
		if e.Time.After(d.LastSeen) {
			d.LastSeen = e.Time
		}
		if e.Source != "" {
			a.srcSet[e.Source] = struct{}{}
		}
	}
	out := make([]DecoyAgg, 0, len(byTarget))
	for _, a := range byTarget {
		a.d.SourceCount = len(a.srcSet)
		out = append(out, *a.d)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Target < out[j].Target
	})
	if len(out) > topDecoysLimit {
		out = out[:topDecoysLimit]
	}
	if out == nil {
		out = []DecoyAgg{}
	}
	return out
}
