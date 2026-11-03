package campaign

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Armur-Ai/heramba/internal/evidence"
)

// TimelineEntry is one step in an actor's reconstructed timeline.
type TimelineEntry struct {
	Time   time.Time
	Kind   string
	Detail string
	// Inferred is true when this entry rests on a hypothesis (e.g. a
	// cross-source link) rather than a direct observation.
	Inferred bool
}

// ActorTimeline reconstructs an actor's activity as a chronological list of
// events, pulling the concrete events from the store. Each entry is marked as
// observed or inferred so an analyst never mistakes a linked session for a
// proven one.
func (res *Result) ActorTimeline(store *evidence.Store, actorID evidence.ID) []TimelineEntry {
	a, ok := res.Actors[actorID]
	if !ok {
		return nil
	}
	var out []TimelineEntry
	multiSource := a.SpansMultipleSources()
	for _, sid := range a.SessionIDs {
		s, ok := res.Sessions[sid]
		if !ok {
			continue
		}
		for _, eid := range s.EventIDs {
			e, ok := store.Event(eid)
			if !ok {
				continue
			}
			out = append(out, TimelineEntry{
				Time:   e.Provenance.ObservedAt,
				Kind:   string(e.Kind),
				Detail: fmt.Sprintf("%s %s", strings.ToUpper(e.Attributes["method"]), firstNonEmpty(e.Target, e.Attributes["path"], e.Detail)),
				// Events within a session are observed; the *grouping* of a
				// multi-source actor is the inferred part.
				Inferred: multiSource,
			})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Time.Before(out[j].Time) })
	return out
}

// Explain produces a human-readable, provenance-honest summary of an actor:
// what was observed (sessions, source keys, targets, deception hits) versus
// what is inferred (the actor grouping, its confidence and uncertainty).
func (res *Result) Explain(actorID evidence.ID) string {
	a, ok := res.Actors[actorID]
	if !ok {
		return "unknown actor"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Actor %s\n", a.ID)
	fmt.Fprintf(&b, "  active: %s → %s (%s)\n",
		a.FirstSeen.Format(time.RFC3339), a.LastSeen.Format(time.RFC3339),
		a.LastSeen.Sub(a.FirstSeen).Round(time.Second))

	// OBSERVED
	b.WriteString("  observed (facts):\n")
	fmt.Fprintf(&b, "    - %d session(s) across %d source key(s): %s\n",
		len(a.SessionIDs), len(a.SourceKeys), strings.Join(sortedKeys(a.SourceKeys), ", "))
	targets := map[string]int{}
	deception := false
	for _, sid := range a.SessionIDs {
		s := res.Sessions[sid]
		for t, n := range s.Targets {
			targets[t] += n
		}
		if s.DeceptionHit {
			deception = true
		}
	}
	if len(targets) > 0 {
		fmt.Fprintf(&b, "    - targets touched: %s\n", strings.Join(topTargets(targets, 6), ", "))
	}
	if deception {
		b.WriteString("    - touched a decoy/canary → zero-false-positive attacker signal\n")
	}

	// INFERRED
	b.WriteString("  inferred (hypothesis):\n")
	if a.Link != nil {
		fmt.Fprintf(&b, "    - grouping confidence %.2f, uncertainty %.2f, status %s\n",
			float64(a.Link.Confidence), a.Link.Uncertainty, a.Link.Status)
		if a.SpansMultipleSources() {
			b.WriteString("    - ⚠ links multiple source keys — identity inference, corroborate before acting\n")
		}
		fmt.Fprintf(&b, "    - supporting evidence: %d, contradicting: %d\n",
			len(a.Link.Supporting), len(a.Link.Contradicting))
	}
	return b.String()
}

func firstNonEmpty(xs ...string) string {
	for _, x := range xs {
		if strings.TrimSpace(x) != "" {
			return x
		}
	}
	return ""
}

func topTargets(m map[string]int, n int) []string {
	type kv struct {
		k string
		v int
	}
	var all []kv
	for k, v := range m {
		all = append(all, kv{k, v})
	}
	sort.SliceStable(all, func(i, j int) bool {
		if all[i].v != all[j].v {
			return all[i].v > all[j].v
		}
		return all[i].k < all[j].k
	})
	var out []string
	for i, x := range all {
		if i >= n {
			break
		}
		out = append(out, fmt.Sprintf("%s×%d", x.k, x.v))
	}
	return out
}
