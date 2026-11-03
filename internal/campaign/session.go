package campaign

import (
	"sort"
	"time"

	"github.com/Armur-Ai/heramba/internal/evidence"
)

// Session is an *observational* grouping: a run of events from one transport
// source key with no idle gap longer than the sessionizer's threshold. It
// asserts nothing about identity — only "these requests arrived close together
// from the same source key".
type Session struct {
	ID           evidence.ID         `json:"id"`
	SourceKey    string              `json:"source_key"`
	Start        time.Time           `json:"start"`
	End          time.Time           `json:"end"`
	EventIDs     []evidence.ID       `json:"event_ids"`
	Targets      map[string]int      `json:"targets"` // target → count
	Fingerprint  Fingerprint         `json:"fingerprint"`
	DeceptionHit bool                `json:"deception_hit"` // touched a decoy/canary (zero-FP)
	MaxSeverity  evidence.Confidence `json:"-"`
}

// Duration is the wall-clock span of the session.
func (s Session) Duration() time.Duration { return s.End.Sub(s.Start) }

// CadenceRPS is the mean request rate across the session (requests/second).
func (s Session) CadenceRPS() float64 {
	d := s.Duration().Seconds()
	if d <= 0 {
		return float64(len(s.EventIDs))
	}
	return float64(len(s.EventIDs)) / d
}

// Sessionizer groups a source's events into sessions by idle gap.
type Sessionizer struct {
	// IdleGap ends a session when the next event is more than this after the
	// previous one.
	IdleGap time.Duration
}

// DefaultSessionizer uses a 30-minute idle gap, a common web-session heuristic.
func DefaultSessionizer() Sessionizer { return Sessionizer{IdleGap: 30 * time.Minute} }

// Sessionize splits one source key's events into chronological sessions. Events
// from mixed sources are first bucketed by SourceKey, so callers may pass the
// full stream. The result is ordered by session start time.
func (sz Sessionizer) Sessionize(events []evidence.SecurityEvent) []Session {
	gap := sz.IdleGap
	if gap <= 0 {
		gap = 30 * time.Minute
	}
	bySource := map[string][]evidence.SecurityEvent{}
	for _, e := range events {
		bySource[e.SourceKey] = append(bySource[e.SourceKey], e)
	}
	var sessions []Session
	for key, evs := range bySource {
		sort.SliceStable(evs, func(i, j int) bool {
			return evs[i].Provenance.ObservedAt.Before(evs[j].Provenance.ObservedAt)
		})
		var cur *Session
		flush := func() {
			if cur != nil {
				sessions = append(sessions, *cur)
				cur = nil
			}
		}
		for _, e := range evs {
			t := e.Provenance.ObservedAt
			if cur != nil && t.Sub(cur.End) > gap {
				flush()
			}
			if cur == nil {
				s := Session{
					ID:          evidence.NewID("sess"),
					SourceKey:   key,
					Start:       t,
					End:         t,
					Targets:     map[string]int{},
					Fingerprint: newFingerprint(),
				}
				cur = &s
			}
			cur.EventIDs = append(cur.EventIDs, e.ID)
			cur.End = t
			cur.Fingerprint.observe(e)
			if e.Target != "" {
				cur.Targets[e.Target]++
			}
			if e.Stage == evidence.StageDeceptionTouch {
				cur.DeceptionHit = true
			}
			if e.Confidence > cur.MaxSeverity {
				cur.MaxSeverity = e.Confidence
			}
		}
		flush()
	}
	// Classify cadence now that durations are known.
	for i := range sessions {
		sessions[i].Fingerprint.CadenceBucket = cadenceBucket(sessions[i].CadenceRPS())
	}
	sort.SliceStable(sessions, func(i, j int) bool {
		return sessions[i].Start.Before(sessions[j].Start)
	})
	return sessions
}

func cadenceBucket(rps float64) string {
	switch {
	case rps >= 20:
		return "burst"
	case rps >= 3:
		return "fast"
	case rps >= 0.2:
		return "steady"
	default:
		return "slow"
	}
}
