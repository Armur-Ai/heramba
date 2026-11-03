// Package swarm detects coordinated, distributed reconnaissance — the thing
// that distinguishes a swarm of autonomous agents from ordinary automation. It
// deliberately does NOT rely on user-agent or raw request rate alone (those are
// trivially spoofed and conflate legitimate high-volume clients with attackers).
// Instead it reasons about temporal synchrony, target overlap, division of
// labor, and deception contact across a cohort of sessions, and reports an
// explainable class with a confidence.
//
// Four classes are distinguished, matching the evaluation in the engineering
// mandate: a single sequential scanner, several independent scanners, a
// coordinated agent swarm, and legitimate high-volume automation.
package swarm

import (
	"fmt"
	"sort"
	"time"

	"github.com/Armur-Ai/heramba/internal/campaign"
)

// Class is the coordination verdict for a cohort of sessions.
type Class string

const (
	ClassSequential      Class = "sequential_scanner"   // one source, ordered enumeration
	ClassIndependent     Class = "independent_scanners" // several unrelated scanners
	ClassCoordinated     Class = "coordinated_swarm"    // synchronized, labor-divided agents
	ClassLegitAutomation Class = "legit_automation"     // high-volume but benign (CI, monitoring)
)

// Verdict is an explainable coordination decision.
type Verdict struct {
	Class      Class    `json:"class"`
	Score      float64  `json:"score"`      // coordination strength [0,1]
	Confidence float64  `json:"confidence"` // calibrated [0,1]
	Reasons    []string `json:"reasons"`
}

// Features are the explainable signals a verdict is built from.
type Features struct {
	Sources         int           // distinct transport source keys
	Sessions        int           // sessions in the cohort
	DistinctTargets int           // union of distinct targets across the cohort
	StartSpread     time.Duration // spread of session start times
	TargetJaccard   float64       // mean pairwise target-set overlap
	Complementarity float64       // union coverage vs mean individual coverage (division of labor)
	DeceptionHits   int           // sessions that touched a decoy/canary
	MeanCadenceRPS  float64
	CadenceSpread   float64 // coefficient of variation of per-session cadence
}

// Detector classifies cohorts. Thresholds are explicit and tunable.
type Detector struct {
	// SyncWindow: sessions whose starts fall within this window are "synchronized".
	SyncWindow time.Duration
	// OverlapThreshold: target Jaccard at/above this is "shared targeting".
	OverlapThreshold float64
	// ComplementThreshold: complementarity at/above this is "division of labor".
	ComplementThreshold float64
	// EnumerationThreshold: distinct targets at/above this looks like surface
	// enumeration rather than benign repeated calls to known endpoints.
	EnumerationThreshold int
}

// DefaultDetector returns calibrated defaults.
func DefaultDetector() Detector {
	return Detector{
		SyncWindow:           2 * time.Minute,
		OverlapThreshold:     0.3,
		ComplementThreshold:  1.6,
		EnumerationThreshold: 10,
	}
}

// Extract computes cohort features from sessions.
func (d Detector) Extract(sessions []campaign.Session) Features {
	f := Features{Sessions: len(sessions)}
	if len(sessions) == 0 {
		return f
	}
	srcSet := map[string]struct{}{}
	var minStart, maxStart time.Time
	var cadences []float64
	unionTargets := map[string]struct{}{}
	var totalDistinct int
	for i, s := range sessions {
		srcSet[s.SourceKey] = struct{}{}
		if i == 0 || s.Start.Before(minStart) {
			minStart = s.Start
		}
		if i == 0 || s.Start.After(maxStart) {
			maxStart = s.Start
		}
		cadences = append(cadences, s.CadenceRPS())
		if s.DeceptionHit {
			f.DeceptionHits++
		}
		for t := range s.Targets {
			unionTargets[t] = struct{}{}
		}
		totalDistinct += len(s.Targets)
	}
	f.Sources = len(srcSet)
	f.DistinctTargets = len(unionTargets)
	f.StartSpread = maxStart.Sub(minStart)
	f.TargetJaccard = meanPairwiseJaccard(sessions)
	// Complementarity: how much larger the union is than the average individual
	// coverage — high when sessions split the surface between them.
	meanIndividual := float64(totalDistinct) / float64(len(sessions))
	if meanIndividual > 0 {
		f.Complementarity = float64(len(unionTargets)) / meanIndividual
	}
	f.MeanCadenceRPS, f.CadenceSpread = meanAndCV(cadences)
	return f
}

// Classify returns an explainable verdict for a cohort of sessions.
func (d Detector) Classify(sessions []campaign.Session) Verdict {
	f := d.Extract(sessions)
	var reasons []string
	add := func(format string, a ...any) { reasons = append(reasons, fmt.Sprintf(format, a...)) }

	synchronized := f.Sources > 1 && f.StartSpread <= d.SyncWindow
	sharedTargets := f.TargetJaccard >= d.OverlapThreshold
	dividedLabor := f.Complementarity >= d.ComplementThreshold && f.Sources > 1

	// Single source → either a sequential scanner or benign automation. The
	// discriminator is enumeration breadth and decoy contact, NOT request rate:
	// legitimate automation hammers a few known endpoints; a scanner walks a
	// wide, novel surface and tends to trip decoys.
	enumerating := f.DistinctTargets >= d.EnumerationThreshold
	if f.Sources <= 1 {
		if f.DeceptionHits == 0 && !enumerating {
			add("single source, narrow surface (%d targets), no decoy contact", f.DistinctTargets)
			return Verdict{Class: ClassLegitAutomation, Score: 0.1, Confidence: 0.6, Reasons: reasons}
		}
		if f.DeceptionHits > 0 {
			add("single source touched a decoy/canary (zero-FP)")
		}
		add("single source enumerating %d distinct targets", f.DistinctTargets)
		return Verdict{Class: ClassSequential, Score: 0.4, Confidence: 0.65, Reasons: reasons}
	}

	// Multi-source: coordination hinges on synchrony + (shared OR divided targets).
	score := 0.0
	if synchronized {
		score += 0.4
		add("multi-source starts within %s (synchronized)", f.StartSpread.Round(time.Second))
	}
	if sharedTargets {
		score += 0.3
		add("shared targeting (mean target Jaccard %.2f)", f.TargetJaccard)
	}
	if dividedLabor {
		score += 0.3
		add("division of labor (complementarity %.2f)", f.Complementarity)
	}
	if f.DeceptionHits > 0 {
		score += 0.1
		add("%d session(s) touched a decoy/canary (zero-FP)", f.DeceptionHits)
	}
	if score > 1 {
		score = 1
	}

	// Coordination requires synchrony AND a signal benign clients do not emit:
	// division of labor, OR shared targeting that is *also* broad enumeration or
	// trips a decoy. Synchronized clients merely sharing a few known endpoints
	// (a deploy, a monitoring fleet, users hitting the same dashboard) are NOT a
	// swarm — gating the shared-only path on enumeration/decoy breadth is what
	// keeps the false-positive rate down.
	sharedIsCoordinating := sharedTargets && (enumerating || f.DeceptionHits > 0)
	if synchronized && (dividedLabor || sharedIsCoordinating) {
		return Verdict{Class: ClassCoordinated, Score: score,
			Confidence: clamp01(0.5 + score/2), Reasons: reasons}
	}
	if synchronized && sharedTargets && !enumerating && f.DeceptionHits == 0 {
		add("synchronized and shared, but a narrow known surface with no decoy contact — reads as benign automation, not a swarm")
	}
	// Multiple sources, not synchronized/divided. Distinguish independent
	// scanners (each enumerating a novel surface, or tripping decoys) from
	// benign multi-source automation (narrow, known endpoints) by enumeration
	// breadth and decoy contact — never by rate alone.
	if enumerating || f.DeceptionHits > 0 {
		add("multiple sources enumerating a wide surface (%d distinct targets) without synchrony", f.DistinctTargets)
		return Verdict{Class: ClassIndependent, Score: score, Confidence: 0.55, Reasons: reasons}
	}
	add("multi-source, narrow known surface, no decoy contact — benign automation")
	return Verdict{Class: ClassLegitAutomation, Score: score, Confidence: 0.55, Reasons: reasons}
}

func meanPairwiseJaccard(sessions []campaign.Session) float64 {
	if len(sessions) < 2 {
		if len(sessions) == 1 && len(sessions[0].Targets) > 0 {
			return 1
		}
		return 0
	}
	var sum float64
	var n int
	for i := 0; i < len(sessions); i++ {
		for j := i + 1; j < len(sessions); j++ {
			sum += jaccardTargets(sessions[i].Targets, sessions[j].Targets)
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return sum / float64(n)
}

func jaccardTargets(a, b map[string]int) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 0
	}
	inter := 0
	for k := range a {
		if _, ok := b[k]; ok {
			inter++
		}
	}
	union := len(a) + len(b) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

func meanAndCV(xs []float64) (mean, cv float64) {
	if len(xs) == 0 {
		return 0, 0
	}
	var sum float64
	for _, x := range xs {
		sum += x
	}
	mean = sum / float64(len(xs))
	if mean == 0 {
		return 0, 0
	}
	var varr float64
	for _, x := range xs {
		varr += (x - mean) * (x - mean)
	}
	varr /= float64(len(xs))
	cv = sqrt(varr) / mean
	return mean, cv
}

func sqrt(x float64) float64 {
	if x <= 0 {
		return 0
	}
	// Newton's method, a few iterations — avoids importing math for one site.
	g := x
	for i := 0; i < 20; i++ {
		g = 0.5 * (g + x/g)
	}
	return g
}

func clamp01(x float64) float64 {
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}

// SortReasons gives verdicts a stable reason order for display.
func (v Verdict) SortReasons() { sort.Strings(v.Reasons) }
