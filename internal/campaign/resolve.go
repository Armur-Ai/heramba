package campaign

import (
	"fmt"
	"sort"
	"time"

	"github.com/Armur-Ai/heramba/internal/evidence"
)

// Resolver performs probabilistic entity resolution: it assigns sessions to
// inferred Actors, recording every link as an evidence.Hypothesis with
// supporting (and later contradicting) evidence. It supports false-merge
// correction via Split. It never concludes identity from a source key alone.
type Resolver struct {
	// SameSourceConf is the confidence that two sessions sharing a transport
	// source key are the same actor. High, but < 1.0 — shared NAT, CGNAT, and
	// proxies mean a source key is not an identity.
	SameSourceConf evidence.Confidence
	// LinkThreshold is the minimum fingerprint similarity (cross-source) to
	// hypothesize that two sessions are the same actor.
	LinkThreshold float64
	// CrossSourceScale discounts a cross-source link relative to its raw
	// fingerprint similarity (behavioral match across IPs is weaker evidence).
	CrossSourceScale float64
	// ReviewWindow is how long an actor-link hypothesis stays before review.
	ReviewWindow time.Duration

	actors []*Actor
}

// DefaultResolver returns calibrated defaults suitable for edge traffic.
func DefaultResolver() *Resolver {
	return &Resolver{
		SameSourceConf:   0.85,
		LinkThreshold:    0.6,
		CrossSourceScale: 0.7,
		ReviewWindow:     24 * time.Hour,
	}
}

// Actors returns the current inferred actors.
func (r *Resolver) Actors() []*Actor { return r.actors }

// Assign places a session with the best-matching existing actor, or opens a new
// actor if none clears the bar. It returns the actor and the confidence of the
// link decision. The decision is explainable: same-source is a strong link;
// cross-source requires fingerprint similarity above LinkThreshold.
func (r *Resolver) Assign(s Session) (*Actor, evidence.Confidence) {
	best, bestConf, bestWhy := r.bestMatch(s)
	if best != nil {
		best.mergeSession(s)
		ref := evidence.EvidenceRef{ID: s.ID, Note: bestWhy}
		best.Link.Support(ref, bestConf)
		return best, bestConf
	}
	// New actor — the link hypothesis starts with just this session.
	a := &Actor{
		ID:          evidence.NewID("actor"),
		SourceKeys:  map[string]struct{}{},
		Fingerprint: newFingerprint(),
	}
	a.Link = evidence.NewHypothesis(
		fmt.Sprintf("sessions grouped as actor %s", a.ID),
		r.SameSourceConf, r.ReviewWindow)
	a.Link.Stage = evidence.StageRecon
	a.mergeSession(s)
	a.Link.Supporting = append(a.Link.Supporting, evidence.EvidenceRef{
		ID:   s.ID,
		Note: "seed session",
	})
	r.actors = append(r.actors, a)
	return a, r.SameSourceConf
}

// bestMatch finds the actor most likely to own a session and the confidence +
// human reason for the link.
func (r *Resolver) bestMatch(s Session) (*Actor, evidence.Confidence, string) {
	var best *Actor
	var bestConf evidence.Confidence
	var bestWhy string
	for _, a := range r.actors {
		conf, why, ok := r.linkScore(a, s)
		if ok && conf > bestConf {
			best, bestConf, bestWhy = a, conf, why
		}
	}
	return best, bestConf, bestWhy
}

// linkScore scores a candidate (actor, session) link.
//
// Same transport source key is a strong link (still < 1.0: NAT/CGNAT/proxy
// mean a source key is not an identity). A *cross-source* link is only made
// when the session shares a DISTINCTIVE identity token with the actor (a canary,
// an auth subject, a session id) — never on generic behavioral similarity,
// which would wrongly collapse every user of a common tool into one actor.
// Behavioral similarity across sources is coordination evidence and surfaces at
// the campaign layer instead.
func (r *Resolver) linkScore(a *Actor, s Session) (evidence.Confidence, string, bool) {
	if _, shared := a.SourceKeys[s.SourceKey]; shared {
		return r.SameSourceConf,
			fmt.Sprintf("same source key %s", s.SourceKey), true
	}
	if shared := intersect(a.Fingerprint.IdentityTokens, s.Fingerprint.IdentityTokens); len(shared) > 0 {
		// Distinctive shared identity → confident cross-source link, scaled.
		conf := evidence.Confidence(r.CrossSourceScale).Clamp()
		return conf,
			fmt.Sprintf("shared distinctive identity token(s) %v across source keys", shared), true
	}
	return 0, "", false
}

func intersect(a, b map[string]struct{}) []string {
	var out []string
	for k := range a {
		if _, ok := b[k]; ok {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// Split performs false-merge correction: it pulls a session out of an actor
// into a fresh actor, recording contradicting evidence on the original link and
// a seed on the new one. Returns the new actor, or an error if the session is
// not in the actor.
func (r *Resolver) Split(a *Actor, sessionID evidence.ID, reason string) (*Actor, error) {
	idx := -1
	for i, id := range a.SessionIDs {
		if id == sessionID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return nil, fmt.Errorf("session %s not in actor %s", sessionID, a.ID)
	}
	a.SessionIDs = append(a.SessionIDs[:idx], a.SessionIDs[idx+1:]...)
	a.Link.Contradict(evidence.EvidenceRef{ID: sessionID, Note: reason}, 0.5)

	split := &Actor{
		ID:          evidence.NewID("actor"),
		SourceKeys:  map[string]struct{}{},
		Fingerprint: newFingerprint(),
	}
	split.Link = evidence.NewHypothesis(
		fmt.Sprintf("session %s split from actor %s: %s", sessionID, a.ID, reason),
		r.SameSourceConf, r.ReviewWindow)
	split.Link.Supporting = append(split.Link.Supporting, evidence.EvidenceRef{
		ID:   sessionID,
		Note: "split seed: " + reason,
	})
	r.actors = append(r.actors, split)
	return split, nil
}

// Merge folds actor b into actor a (used when later evidence unifies two
// previously-separate actors). It returns a with b's sessions absorbed; b is
// marked merged. Both link hypotheses are reconciled.
func (r *Resolver) Merge(a, b *Actor, reason string, conf evidence.Confidence) {
	for k := range b.SourceKeys {
		a.SourceKeys[k] = struct{}{}
	}
	a.SessionIDs = append(a.SessionIDs, b.SessionIDs...)
	unionFingerprint(&a.Fingerprint, b.Fingerprint)
	if b.FirstSeen.Before(a.FirstSeen) || a.FirstSeen.IsZero() {
		a.FirstSeen = b.FirstSeen
	}
	if b.LastSeen.After(a.LastSeen) {
		a.LastSeen = b.LastSeen
	}
	a.Link.Support(evidence.EvidenceRef{ID: b.ID, Note: "merged: " + reason}, conf)
	b.Link.Status = evidence.StatusMerged
	// Drop b from the active actor list.
	for i, x := range r.actors {
		if x == b {
			r.actors = append(r.actors[:i], r.actors[i+1:]...)
			break
		}
	}
}
