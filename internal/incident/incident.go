// Package incident is the R9 operator layer: it assembles an investigation
// timeline that links observations → facts → hypotheses → proposed actions →
// approved actions → outcomes into one causal chain, so an operator can explain
// every consequential action, see its evidence and uncertainty, confirm it is
// reversible, and export the whole thing as an incident report.
//
// It composes the existing typed records (evidence.SecurityEvent / Fact /
// Hypothesis and strategy.ActionExecution) rather than inventing a parallel
// model, and it keeps the observed/inferred distinction explicit at every step.
package incident

import (
	"sort"
	"time"

	"github.com/Armur-Ai/heramba/internal/evidence"
	"github.com/Armur-Ai/heramba/internal/strategy"
)

// Stage is a step in the investigation chain.
type Stage string

const (
	StageObservation Stage = "observation" // what was directly seen
	StageFact        Stage = "fact"        // a validated restatement, no inference
	StageHypothesis  Stage = "hypothesis"  // an inference, with confidence + uncertainty
	StageProposed    Stage = "proposed_action"
	StageApproved    Stage = "approved_action"
	StageOutcome     Stage = "outcome"
)

// Entry is one node in the timeline.
type Entry struct {
	Time         time.Time `json:"time"`
	Stage        Stage     `json:"stage"`
	ID           string    `json:"id"`
	Observed     bool      `json:"observed"` // true = observed fact; false = inference/decision
	Summary      string    `json:"summary"`
	Confidence   float64   `json:"confidence,omitempty"`
	Uncertainty  float64   `json:"uncertainty,omitempty"`
	Contradicts  int       `json:"contradicting_evidence,omitempty"`
	SourceHealth string    `json:"source_health,omitempty"`
	CausalNote   string    `json:"causal_note,omitempty"`
	Links        []string  `json:"links,omitempty"` // IDs of entries this derives from
}

// BlastRadius is an honest estimate of an action's reach.
type BlastRadius struct {
	ScopeKind      string `json:"scope_kind"` // source_key | session_id | decoy_id
	ScopeID        string `json:"scope_id"`
	Subjects       int    `json:"subjects"`         // bounded: always 1 (nothing global)
	CanAffectLegit bool   `json:"can_affect_legit"` // from impact
	Reversible     bool   `json:"reversible"`
	ReversibleWhy  string `json:"reversible_why"`
}

// ActionView is the operator's per-action summary: decision, blast radius, and
// reversibility, derived from the audited execution.
type ActionView struct {
	ID          string      `json:"id"`
	Type        string      `json:"type"`
	Status      string      `json:"status"`
	Verdict     string      `json:"verdict"`
	EvidenceIDs []string    `json:"evidence_ids"`
	Blast       BlastRadius `json:"blast_radius"`
}

// RetentionPolicy bounds how long records are kept.
type RetentionPolicy struct {
	MaxAge time.Duration `json:"max_age"`
}

// Incident is the assembled, exportable investigation.
type Incident struct {
	ID        string          `json:"id"`
	Title     string          `json:"title"`
	Window    [2]time.Time    `json:"window"`
	Entries   []Entry         `json:"entries"`
	Actions   []ActionView    `json:"actions"`
	Retention RetentionPolicy `json:"retention"`
	Caveats   []string        `json:"causal_caveats"`
}

// Builder accumulates records into an incident.
type Builder struct {
	inc Incident
}

// New starts an incident with a standing retention policy and causal caveats.
func New(id, title string, retention RetentionPolicy) *Builder {
	return &Builder{inc: Incident{
		ID: id, Title: title, Retention: retention,
		Caveats: []string{
			"inferred entries (hypotheses, actor/campaign links) are probabilistic, not proof; correlation is not causation",
			"a multi-source actor link is an identity inference to corroborate before acting",
			"source health reflects sensor liveness, not data truthfulness",
		},
	}}
}

// Observe adds an observation entry from a security event.
func (b *Builder) Observe(ev evidence.SecurityEvent, sourceHealth string) *Builder {
	b.inc.Entries = append(b.inc.Entries, Entry{
		Time: ev.Provenance.ObservedAt, Stage: StageObservation, ID: string(ev.ID), Observed: true,
		Summary:      string(ev.Kind) + " from " + ev.SourceKey + " on " + ev.Target,
		Confidence:   float64(ev.Confidence),
		SourceHealth: sourceHealth,
	})
	return b
}

// Fact adds a validated fact entry (no inference, confidence 1.0).
func (b *Builder) Fact(f evidence.Fact, derivedFrom ...string) *Builder {
	b.inc.Entries = append(b.inc.Entries, Entry{
		Time: f.CreatedAt, Stage: StageFact, ID: string(f.ID), Observed: true,
		Summary: f.Statement, Confidence: 1.0, Links: derivedFrom,
	})
	return b
}

// Hypothesis adds an inference entry carrying confidence, uncertainty, and the
// count of contradicting evidence — so the operator sees what argues against it.
func (b *Builder) Hypothesis(h *evidence.Hypothesis, derivedFrom ...string) *Builder {
	b.inc.Entries = append(b.inc.Entries, Entry{
		Time: h.CreatedAt, Stage: StageHypothesis, ID: string(h.ID), Observed: false,
		Summary:     h.Statement,
		Confidence:  float64(h.Confidence),
		Uncertainty: h.Uncertainty,
		Contradicts: len(h.Contradicting),
		CausalNote:  "inference (" + string(h.Status) + "); corroborate before acting",
	})
	return b
}

// Action folds an audited action execution into the timeline: a proposed entry
// always, an approved entry when approval was required/granted, and an outcome
// entry for the final status. It also records the per-action blast radius and
// reversibility.
func (b *Builder) Action(ax strategy.ActionExecution, derivedFrom ...string) *Builder {
	id := string(ax.Action.ID)
	kind, sid := ax.Action.Scope.Subject()

	b.inc.Entries = append(b.inc.Entries, Entry{
		Time: ax.Action.CreatedAt, Stage: StageProposed, ID: id, Observed: false,
		Summary:    "propose " + string(ax.Action.Type) + " on " + kind + "=" + sid,
		CausalNote: "verdict: " + string(ax.Decision.Verdict),
		Links:      derivedFrom,
	})
	if ax.Decision.Verdict == strategy.VerdictNeedApproval || !ax.EnforcedAt.IsZero() {
		approved := !ax.EnforcedAt.IsZero()
		note := "awaiting human approval (medium+ impact)"
		at := ax.Action.CreatedAt
		if approved {
			note = "approved and enforced"
			at = ax.EnforcedAt
		}
		b.inc.Entries = append(b.inc.Entries, Entry{
			Time: at, Stage: StageApproved, ID: id, Observed: false,
			Summary: "approval gate for " + string(ax.Action.Type), CausalNote: note, Links: []string{id},
		})
	}
	outTime := ax.EnforcedAt
	if !ax.RolledBackAt.IsZero() {
		outTime = ax.RolledBackAt
	}
	if outTime.IsZero() {
		outTime = ax.Action.CreatedAt
	}
	b.inc.Entries = append(b.inc.Entries, Entry{
		Time: outTime, Stage: StageOutcome, ID: id, Observed: false,
		Summary: "status: " + string(ax.Status), Links: []string{id},
	})

	rev, why := reversibility(ax)
	b.inc.Actions = append(b.inc.Actions, ActionView{
		ID: id, Type: string(ax.Action.Type), Status: string(ax.Status),
		Verdict:     string(ax.Decision.Verdict),
		EvidenceIDs: evidenceIDs(ax.Action.Evidence),
		Blast: BlastRadius{
			ScopeKind: kind, ScopeID: sid, Subjects: 1,
			CanAffectLegit: canAffectLegit(ax.Action.Impact),
			Reversible:     rev, ReversibleWhy: why,
		},
	})
	return b
}

// Build finalizes the incident: sorts entries chronologically and sets the
// window. Prunes nothing (use Prune for retention).
func (b *Builder) Build() Incident {
	sort.SliceStable(b.inc.Entries, func(i, j int) bool { return b.inc.Entries[i].Time.Before(b.inc.Entries[j].Time) })
	if len(b.inc.Entries) > 0 {
		b.inc.Window = [2]time.Time{b.inc.Entries[0].Time, b.inc.Entries[len(b.inc.Entries)-1].Time}
	}
	return b.inc
}

// Explain returns the causal chain for one action: every entry that bears its ID
// plus the evidence/hypothesis entries it links from, in time order.
func (inc Incident) Explain(actionID string) []Entry {
	want := map[string]bool{actionID: true}
	// First pass: entries for the action; collect their Links as evidence we need.
	for _, e := range inc.Entries {
		if e.ID == actionID {
			for _, l := range e.Links {
				want[l] = true
			}
		}
	}
	var out []Entry
	for _, e := range inc.Entries {
		if want[e.ID] {
			out = append(out, e)
		}
	}
	return out
}

// AllConsequentialReversible reports whether every consequential (enforced)
// action is reversible, which is the R9 gate for "undo it".
func (inc Incident) AllConsequentialReversible() bool {
	for _, a := range inc.Actions {
		if a.Status == string(strategy.StatusEnforced) && !a.Blast.Reversible {
			return false
		}
	}
	return true
}

// AllConsequentialHaveEvidence reports whether every state-changing action cites
// evidence — the R9 gate for "audit its evidence".
func (inc Incident) AllConsequentialHaveEvidence() bool {
	for _, a := range inc.Actions {
		if changesState(a.Type) && len(a.EvidenceIDs) == 0 {
			return false
		}
	}
	return true
}

// Prune drops entries older than the retention window relative to now.
func (inc Incident) Prune(now time.Time) Incident {
	if inc.Retention.MaxAge <= 0 {
		return inc
	}
	cutoff := now.Add(-inc.Retention.MaxAge)
	var kept []Entry
	for _, e := range inc.Entries {
		if !e.Time.Before(cutoff) {
			kept = append(kept, e)
		}
	}
	inc.Entries = kept
	return inc
}

func reversibility(ax strategy.ActionExecution) (bool, string) {
	switch ax.Status {
	case strategy.StatusEnforced:
		if ax.RollbackToken != "" {
			return true, "enforced with a rollback token; Rollback/expiry reverts it"
		}
		return false, "enforced without a rollback token"
	case strategy.StatusRolledBack, strategy.StatusExpired:
		return true, "already reverted"
	case strategy.StatusDryRun, strategy.StatusProposed, strategy.StatusPending, strategy.StatusRejected:
		return true, "no external effect to reverse (shadow/pending/rejected)"
	default:
		return true, "no enforced effect"
	}
}

func canAffectLegit(impact strategy.Impact) bool {
	return impact == strategy.ImpactMedium || impact == strategy.ImpactHigh
}

func changesState(t string) bool {
	switch strategy.ActionType(t) {
	case strategy.ActionObserve, strategy.ActionEscalate, strategy.ActionRecommendIsolate:
		return false
	default:
		return true
	}
}

func evidenceIDs(refs []evidence.EvidenceRef) []string {
	var out []string
	for _, r := range refs {
		out = append(out, string(r.ID))
	}
	return out
}
