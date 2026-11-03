// Package strategy is Heramba's adaptive-defense plane: it proposes, validates,
// and (optionally) enforces bounded defensive actions in response to campaign
// intelligence — always under deterministic policy, never under model control.
//
// The safety spine, taken verbatim from the engineering mandate:
//
//   - Every action is a typed, bounded schema with explicit scope, expiry,
//     rollback, and audit. There are no free-form actions.
//   - Policy is code and deterministic. It reads only typed fields; no
//     attacker-origin text and no LLM proposal can change a decision. An LLM may
//     *propose* an Action, but a proposal is just data that must clear the same
//     policy gate — it can never execute enforcement itself.
//   - Shadow/observe is the safe default: actions are planned and audited but
//     not enforced. High-impact actions require human approval. Everything
//     expires and can be rolled back. An emergency disable stops all enforcement.
package strategy

import (
	"time"

	"github.com/Armur-Ai/heramba/internal/evidence"
)

// ActionType enumerates the bounded set of defensive actions. This list is
// closed: the planner and policy only ever reason about these.
type ActionType string

const (
	ActionObserve          ActionType = "observe"           // record only; no change
	ActionActivateDecoy    ActionType = "activate_decoy"    // bring a decoy/lure online for a source
	ActionAlterSynthetic   ActionType = "alter_synthetic"   // change a *synthetic* response (never a real one)
	ActionRateLimit        ActionType = "rate_limit"        // temporary per-source request-rate cap
	ActionRestrictSession  ActionType = "restrict_session"  // scoped restriction on a session
	ActionRecommendIsolate ActionType = "recommend_isolate" // advisory: recommend isolating a source (no auto-block)
	ActionEscalate         ActionType = "escalate"          // raise for human investigation
)

// Impact estimates an action's effect on legitimate users. The planner must set
// it honestly; policy uses it to decide whether approval is required.
type Impact string

const (
	ImpactNone   Impact = "none"   // cannot affect a legitimate user (e.g. observe, decoy)
	ImpactLow    Impact = "low"    // bounded, reversible, unlikely to touch legit users
	ImpactMedium Impact = "medium" // could degrade a legit user's experience
	ImpactHigh   Impact = "high"   // could deny a legit user; always needs approval
)

// Action is a typed, bounded defensive-action proposal. It carries its own
// justification (evidence + expected benefit), its blast radius (scope +
// impact), and its safety envelope (expiry + rollback). Policy decisions read
// ONLY these typed fields.
type Action struct {
	ID   evidence.ID `json:"id"`
	Type ActionType  `json:"type"`
	// Scope bounds the action to a specific subject (a source key, a session id).
	// An action with an empty scope is rejected by policy — nothing is global.
	Scope    Scope         `json:"scope"`
	Impact   Impact        `json:"impact"`
	Duration time.Duration `json:"duration"` // how long the action stays in force
	// Evidence references justifying the action. Free-text notes on refs are
	// NEVER read by policy — they are audit context only.
	Evidence        []evidence.EvidenceRef `json:"evidence"`
	ExpectedBenefit string                 `json:"expected_benefit"`
	// ProposedBy records the origin ("planner", "operator", "llm:<model>").
	// An LLM origin does not grant any execution authority — it still clears the
	// same policy gate.
	ProposedBy string    `json:"proposed_by"`
	CreatedAt  time.Time `json:"created_at"`
}

// Scope bounds an action to a subject. Exactly one subject must be set.
type Scope struct {
	SourceKey string `json:"source_key,omitempty"`
	SessionID string `json:"session_id,omitempty"`
	DecoyID   string `json:"decoy_id,omitempty"`
}

// Subject returns the single subject the scope targets, or "" if none/ambiguous.
func (s Scope) Subject() (kind, id string) {
	switch {
	case s.SourceKey != "" && s.SessionID == "" && s.DecoyID == "":
		return "source_key", s.SourceKey
	case s.SessionID != "" && s.SourceKey == "" && s.DecoyID == "":
		return "session_id", s.SessionID
	case s.DecoyID != "" && s.SourceKey == "" && s.SessionID == "":
		return "decoy_id", s.DecoyID
	default:
		return "", ""
	}
}

// NewAction builds an action with an ID and creation stamp.
func NewAction(t ActionType, scope Scope, impact Impact, dur time.Duration, proposedBy string) Action {
	return Action{
		ID:         evidence.NewID("act"),
		Type:       t,
		Scope:      scope,
		Impact:     impact,
		Duration:   dur,
		ProposedBy: proposedBy,
		CreatedAt:  time.Now().UTC(),
	}
}

// WithEvidence attaches justifying evidence and returns the action.
func (a Action) WithEvidence(refs ...evidence.EvidenceRef) Action {
	a.Evidence = append(a.Evidence, refs...)
	return a
}

// WithBenefit sets the expected benefit and returns the action.
func (a Action) WithBenefit(s string) Action { a.ExpectedBenefit = s; return a }

// ExecutionStatus tracks an action through the engine.
type ExecutionStatus string

const (
	StatusProposed   ExecutionStatus = "proposed"
	StatusRejected   ExecutionStatus = "rejected" // failed policy
	StatusDryRun     ExecutionStatus = "dry_run"  // validated, not enforced (shadow or pre-approval)
	StatusPending    ExecutionStatus = "pending"  // awaiting approval
	StatusEnforced   ExecutionStatus = "enforced" // active
	StatusExpired    ExecutionStatus = "expired"  // duration elapsed, auto-rolled-back
	StatusRolledBack ExecutionStatus = "rolled_back"
)

// ActionExecution is the audit record of an action's lifecycle.
type ActionExecution struct {
	Action        Action          `json:"action"`
	Status        ExecutionStatus `json:"status"`
	Decision      Decision        `json:"decision"`
	EnforcedAt    time.Time       `json:"enforced_at,omitempty"`
	ExpiresAt     time.Time       `json:"expires_at,omitempty"`
	RolledBackAt  time.Time       `json:"rolled_back_at,omitempty"`
	RollbackToken string          `json:"rollback_token,omitempty"`
	Notes         []string        `json:"notes,omitempty"`
}
