package campaign

import (
	"time"

	"github.com/Armur-Ai/heramba/internal/evidence"
)

// Actor is an *inferred* entity: the hypothesis that a set of sessions were
// driven by the same operator or tool. It is never asserted as fact. The Link
// hypothesis carries the evidence and confidence behind the grouping, and the
// grouping is reversible (see Resolver.Split).
type Actor struct {
	ID          evidence.ID          `json:"id"`
	SessionIDs  []evidence.ID        `json:"session_ids"`
	SourceKeys  map[string]struct{}  `json:"source_keys"`
	Fingerprint Fingerprint          `json:"fingerprint"`
	FirstSeen   time.Time            `json:"first_seen"`
	LastSeen    time.Time            `json:"last_seen"`
	Link        *evidence.Hypothesis `json:"link"` // "these sessions are one actor"
}

// SpansMultipleSources reports whether the actor hypothesis links sessions from
// more than one transport source key — the riskiest kind of inference, and the
// one most in need of corroborating evidence.
func (a *Actor) SpansMultipleSources() bool { return len(a.SourceKeys) > 1 }

// mergeSession folds a session into an actor, widening its fingerprint and
// source-key set and extending its time span.
func (a *Actor) mergeSession(s Session) {
	a.SessionIDs = append(a.SessionIDs, s.ID)
	if a.SourceKeys == nil {
		a.SourceKeys = map[string]struct{}{}
	}
	a.SourceKeys[s.SourceKey] = struct{}{}
	unionFingerprint(&a.Fingerprint, s.Fingerprint)
	if a.FirstSeen.IsZero() || s.Start.Before(a.FirstSeen) {
		a.FirstSeen = s.Start
	}
	if s.End.After(a.LastSeen) {
		a.LastSeen = s.End
	}
}

func unionFingerprint(dst *Fingerprint, src Fingerprint) {
	if dst.UserAgents == nil {
		*dst = newFingerprint()
	}
	for _, pair := range [][2]map[string]struct{}{
		{dst.UserAgents, src.UserAgents},
		{dst.Accepts, src.Accepts},
		{dst.Methods, src.Methods},
		{dst.PathStyles, src.PathStyles},
		{dst.ToolSignals, src.ToolSignals},
		{dst.IdentityTokens, src.IdentityTokens},
	} {
		for k := range pair[1] {
			pair[0][k] = struct{}{}
		}
	}
	if dst.CadenceBucket == "" {
		dst.CadenceBucket = src.CadenceBucket
	}
}
