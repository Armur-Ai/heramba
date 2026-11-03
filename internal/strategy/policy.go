package strategy

import (
	"fmt"
	"time"
)

// Mode is the enforcement posture. Shadow is the safe default: actions are
// validated and audited but never enforced.
type Mode string

const (
	ModeShadow  Mode = "shadow"  // validate + audit, never enforce (default)
	ModeEnforce Mode = "enforce" // enforce actions policy permits
)

// Verdict is a policy outcome.
type Verdict string

const (
	VerdictAllow        Verdict = "allow"
	VerdictDeny         Verdict = "deny"
	VerdictNeedApproval Verdict = "need_approval"
)

// Decision is the deterministic result of evaluating an action against policy.
// It is explainable: Reasons lists every rule that fired.
type Decision struct {
	Verdict Verdict  `json:"verdict"`
	Reasons []string `json:"reasons"`
}

func (d Decision) allowed() bool { return d.Verdict == VerdictAllow }

// Policy is deterministic policy-as-code. It reads ONLY the typed fields of an
// Action — never evidence notes, never free text, never a proposer's identity
// beyond the closed origin set. This is what makes prompt injection structurally
// impossible: there is no field an attacker can populate that policy will trust.
type Policy struct {
	// AllowedTypes is the closed set of action types permitted at all.
	AllowedTypes map[ActionType]bool
	// MaxDuration caps how long any action may stay in force.
	MaxDuration time.Duration
	// ApprovalRequiredAtOrAbove: any action with impact >= this needs approval.
	ApprovalRequiredAtOrAbove Impact
	// RequireEvidence: actions that change anything must cite >=1 evidence ref.
	RequireEvidence bool
}

// DefaultPolicy is a conservative, production-safe policy: only non- and
// low-impact actions run without approval, nothing lasts over an hour, and any
// medium+ impact action needs a human.
func DefaultPolicy() Policy {
	return Policy{
		AllowedTypes: map[ActionType]bool{
			ActionObserve:          true,
			ActionActivateDecoy:    true,
			ActionAlterSynthetic:   true,
			ActionRateLimit:        true,
			ActionRestrictSession:  true,
			ActionRecommendIsolate: true,
			ActionEscalate:         true,
		},
		MaxDuration:               time.Hour,
		ApprovalRequiredAtOrAbove: ImpactMedium,
		RequireEvidence:           true,
	}
}

var impactRank = map[Impact]int{ImpactNone: 0, ImpactLow: 1, ImpactMedium: 2, ImpactHigh: 3}

// changesState reports whether an action type has any external effect (and thus
// must cite evidence and respect duration). Observe/escalate/recommend do not
// change enforcement state.
func changesState(t ActionType) bool {
	switch t {
	case ActionObserve, ActionEscalate, ActionRecommendIsolate:
		return false
	default:
		return true
	}
}

// Evaluate returns a deterministic decision for an action. It is a pure
// function of the action's typed fields and the policy — same inputs, same
// decision, every time.
func (p Policy) Evaluate(a Action) Decision {
	var reasons []string
	deny := func(f string, args ...any) Decision {
		reasons = append(reasons, fmt.Sprintf(f, args...))
		return Decision{Verdict: VerdictDeny, Reasons: reasons}
	}

	// 1. Type must be in the closed allow-set.
	if !p.AllowedTypes[a.Type] {
		return deny("action type %q is not permitted by policy", a.Type)
	}
	// 2. Scope must name exactly one subject — nothing is global.
	if kind, id := a.Scope.Subject(); kind == "" || id == "" {
		return deny("action has no single bounded scope subject")
	}
	// 3. State-changing actions must cite evidence and have bounded duration.
	if changesState(a.Type) {
		if p.RequireEvidence && len(a.Evidence) == 0 {
			return deny("state-changing action %q cites no evidence", a.Type)
		}
		if a.Duration <= 0 {
			return deny("state-changing action %q has no positive duration", a.Type)
		}
		if p.MaxDuration > 0 && a.Duration > p.MaxDuration {
			return deny("duration %s exceeds policy max %s", a.Duration, p.MaxDuration)
		}
	}
	// 4. Impact gate: medium+ impact requires approval.
	if impactRank[a.Impact] >= impactRank[p.ApprovalRequiredAtOrAbove] && impactRank[a.Impact] > 0 {
		reasons = append(reasons, fmt.Sprintf("impact %q is at/above approval threshold %q", a.Impact, p.ApprovalRequiredAtOrAbove))
		return Decision{Verdict: VerdictNeedApproval, Reasons: reasons}
	}

	reasons = append(reasons, fmt.Sprintf("type %q within policy, scope bounded, impact %q below approval threshold", a.Type, a.Impact))
	return Decision{Verdict: VerdictAllow, Reasons: reasons}
}
