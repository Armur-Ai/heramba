package strategy

import (
	"time"

	"github.com/Armur-Ai/heramba/internal/evidence"
)

// Situation is the typed summary of what the intelligence plane believes about
// a source/session. It is the planner's ONLY input — deterministic fields, no
// free text that could carry an injection.
type Situation struct {
	SourceKey         string
	SessionID         string
	AgenticScore      float64 // 0..1 from the classifier
	DeceptionHit      bool    // touched a decoy/canary (zero-FP)
	HighSeverityCount int     // count of high/critical events
	Evidence          []evidence.EvidenceRef
}

// Planner turns a Situation into bounded Action proposals using deterministic
// rules. It never enforces — it proposes, and every proposal must clear policy
// in the Engine. The rules are intentionally transparent and conservative.
type Planner struct {
	// AgenticRateLimitAt: propose a rate limit when the agentic score is >= this.
	AgenticRateLimitAt float64
	// RestrictSessionAtSeverity: propose a (medium-impact, approval-gated)
	// session restriction when high-severity events reach this count.
	RestrictSessionAtSeverity int
	// Duration applied to time-bounded actions.
	Duration time.Duration
}

// DefaultPlanner returns calibrated defaults.
func DefaultPlanner() Planner {
	return Planner{
		AgenticRateLimitAt:        0.7,
		RestrictSessionAtSeverity: 3,
		Duration:                  15 * time.Minute,
	}
}

// Plan returns the ordered list of proposed actions for a situation. Order is
// least-to-most impactful so an operator reviewing them sees the safe ones
// first. The plan is deterministic given the same Situation.
func (p Planner) Plan(s Situation) []Action {
	var out []Action
	srcScope := Scope{SourceKey: s.SourceKey}

	// Always observe — zero impact, records intent.
	out = append(out, NewAction(ActionObserve, srcScope, ImpactNone, 0, "planner").
		WithEvidence(s.Evidence...).
		WithBenefit("record source for correlation"))

	if s.DeceptionHit {
		// A decoy/canary hit is a zero-FP attacker signal: light up more decoys
		// and escalate. Both are no-impact on legit users.
		out = append(out, NewAction(ActionActivateDecoy, srcScope, ImpactNone, p.Duration, "planner").
			WithEvidence(s.Evidence...).
			WithBenefit("expand lure surface for a confirmed attacker"))
		out = append(out, NewAction(ActionEscalate, srcScope, ImpactNone, 0, "planner").
			WithEvidence(s.Evidence...).
			WithBenefit("human investigation of a confirmed deception hit"))
	}

	if s.AgenticScore >= p.AgenticRateLimitAt {
		out = append(out, NewAction(ActionRateLimit, srcScope, ImpactLow, p.Duration, "planner").
			WithEvidence(s.Evidence...).
			WithBenefit("throttle a highly-automated source (bounded, reversible)"))
	}

	if s.HighSeverityCount >= p.RestrictSessionAtSeverity {
		// Medium impact → policy will require approval. We also recommend (not
		// perform) isolation, which is advisory only.
		if s.SessionID != "" {
			out = append(out, NewAction(ActionRestrictSession, Scope{SessionID: s.SessionID}, ImpactMedium, p.Duration, "planner").
				WithEvidence(s.Evidence...).
				WithBenefit("limit a session with repeated high-severity events (needs approval)"))
		}
		out = append(out, NewAction(ActionRecommendIsolate, srcScope, ImpactNone, 0, "planner").
			WithEvidence(s.Evidence...).
			WithBenefit("advise operators to consider isolating this source"))
	}
	return out
}
