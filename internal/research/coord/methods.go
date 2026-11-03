package coord

import (
	"math"
	"time"

	"github.com/Armur-Ai/heramba/internal/campaign/swarm"
)

// OperatingThreshold is the single decision boundary used for the confusion
// matrix of every method, so methods are compared at one operating point. PR-AUC
// is reported separately as the threshold-free comparison.
const OperatingThreshold = 0.5

// Method scores a cohort's coordination strength in [0,1] from the shipped
// swarm feature set. Each method is restricted to a subset of signals so the
// experiment attributes performance to specific features (H1 ablations).
type Method struct {
	Name  string
	Score func(f swarm.Features, syncWindow time.Duration) float64
}

// sharedNorm maps mean pairwise target Jaccard onto [0,1] against the shipped
// overlap threshold.
func sharedNorm(f swarm.Features) float64 {
	return clamp01(f.TargetJaccard / swarm.DefaultDetector().OverlapThreshold)
}

// dividedNorm maps complementarity (division of labor) onto [0,1]: ~0 when
// sessions all target the same set, ~1 when they split a surface n ways.
func dividedNorm(f swarm.Features) float64 {
	if f.Sources <= 1 {
		return 0
	}
	return clamp01((f.Complementarity - 1) / (2.0 - 1))
}

func synchronized(f swarm.Features, syncWindow time.Duration) bool {
	return f.Sources > 1 && f.StartSpread <= syncWindow
}

// RequestOnly: a rate-only heuristic. Deliberately weak — with request counts
// and windows matched, per-session cadence tracks source count, not
// coordination, so this is expected to be a poor coordination detector. Kept to
// quantify how little a volume heuristic buys.
func RequestOnly() Method {
	return Method{Name: "request_only", Score: func(f swarm.Features, _ time.Duration) float64 {
		return sigmoid((f.MeanCadenceRPS - 0.3) / 0.2)
	}}
}

// StaticCorrelation: shared-targeting only (mean pairwise Jaccard), no timing,
// no division-of-labor, no memory. Flags any cohort that reuses the same
// wordlist — including independent scanners and legit automation — and misses
// labor-divided swarms (low Jaccard). Illustrates why co-targeting alone is
// insufficient.
func StaticCorrelation() Method {
	return Method{Name: "static_correlation", Score: func(f swarm.Features, _ time.Duration) float64 {
		return sharedNorm(f)
	}}
}

// GraphCorrelation: co-targeting structure — shared OR divided targeting — but
// still no temporal synchrony gate. Catches division of labor that static
// correlation misses, at the cost of flagging coincidental co-targeting.
func GraphCorrelation() Method {
	return Method{Name: "graph_correlation", Score: func(f swarm.Features, _ time.Duration) float64 {
		return math.Max(sharedNorm(f), dividedNorm(f))
	}}
}

// FullHeramba: the shipped signal set — synchrony gate + shared/divided
// targeting + enumeration + decoy contact. Synchrony is the gate that
// suppresses the false positives the correlation-only methods suffer.
func FullHeramba() Method {
	return Method{Name: "full_heramba", Score: func(f swarm.Features, syncWindow time.Duration) float64 {
		return fullScore(f, syncWindow, fullSignals)
	}}
}

// signalSet toggles which signals FullHeramba may use (for ablations).
type signalSet struct {
	sync, shared, divided, decoy bool
}

var fullSignals = signalSet{sync: true, shared: true, divided: true, decoy: true}

func fullScore(f swarm.Features, syncWindow time.Duration, sig signalSet) float64 {
	if f.Sources <= 1 {
		return 0.1 // a single source is not a swarm
	}
	var corr float64
	if sig.shared {
		corr = math.Max(corr, sharedNorm(f))
	}
	if sig.divided {
		corr = math.Max(corr, dividedNorm(f))
	}
	score := 0.5 * corr
	if sig.sync {
		if synchronized(f, syncWindow) {
			score += 0.5
		} else {
			score *= 0.4 // the synchrony gate: un-synchronized co-targeting is discounted
		}
	}
	if sig.decoy && f.DeceptionHits > 0 {
		score = math.Min(1, score+0.05)
	}
	return clamp01(score)
}

// Ablations returns FullHeramba with one signal removed each, to attribute
// performance to specific signals (directive R2: no campaign memory, no
// cross-source correlation, no graph motifs, no active probes).
func Ablations() []Method {
	mk := func(name string, sig signalSet) Method {
		return Method{Name: name, Score: func(f swarm.Features, sw time.Duration) float64 {
			return fullScore(f, sw, sig)
		}}
	}
	return []Method{
		mk("ablate_no_campaign_memory", signalSet{sync: false, shared: true, divided: true, decoy: true}),
		mk("ablate_no_cross_source", signalSet{sync: false, shared: true, divided: false, decoy: true}),
		mk("ablate_no_graph_motifs", signalSet{sync: true, shared: true, divided: false, decoy: true}),
		mk("ablate_no_active_probes", signalSet{sync: true, shared: true, divided: true, decoy: false}),
	}
}

// AllMethods returns the four compared methods in directive order.
func AllMethods() []Method {
	return []Method{RequestOnly(), StaticCorrelation(), GraphCorrelation(), FullHeramba()}
}

func sigmoid(x float64) float64 { return 1 / (1 + math.Exp(-x)) }

func clamp01(x float64) float64 {
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}
