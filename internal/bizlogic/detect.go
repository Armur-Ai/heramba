package bizlogic

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// roleRank orders roles for the RequiredRole check.
var roleRank = map[string]int{"": 0, "viewer": 1, "member": 2, "admin": 3}

// Request is one observed API call in a session, already attributed to an actor
// and (where known) the object being touched.
type Request struct {
	Time    time.Time
	Method  string
	Path    string
	ActorID string
	Role    string
	// ObjectOwned reports whether the actor is authorized for the object named
	// by the operation's OwnershipParam. The caller supplies this (e.g. from the
	// deception world's CanAccessProject), keeping bizlogic decoupled from any
	// particular ownership store.
	ObjectOwned bool
}

// FindingKind classifies a business-logic violation.
type FindingKind string

const (
	FindingBOLA      FindingKind = "object_authz_violation" // IDOR/BOLA: touched an object not owned
	FindingPrivilege FindingKind = "privilege_violation"    // called an op above the actor's role
	FindingSequence  FindingKind = "sequence_violation"     // workflow step out of order
	FindingUnknownOp FindingKind = "unknown_operation"      // not in the spec (probing)
)

// Finding is an explainable, evidence-linked business-logic detection.
type Finding struct {
	Kind        FindingKind `json:"kind"`
	OpID        string      `json:"op_id"`
	ActorID     string      `json:"actor_id"`
	Method      string      `json:"method"`
	Path        string      `json:"path"`
	Time        time.Time   `json:"time"`
	Explanation string      `json:"explanation"`
	// Sequence is the ordered op-id trail that led here (evidence of the
	// multi-request nature of the abuse).
	Sequence []string `json:"sequence"`
}

// Detector consumes a session's request sequence and reports business-logic
// violations. It is sequence-aware: it tracks each actor's workflow state and
// op-trail so multi-request abuse is visible, not just single bad requests.
type Detector struct {
	Spec *Spec
	// per-actor state
	state map[string]*actorState
}

type actorState struct {
	wfState map[string]string // workflow id → current state
	trail   []string          // ordered op ids invoked
}

// NewDetector builds a detector over a spec.
func NewDetector(spec *Spec) *Detector {
	return &Detector{Spec: spec, state: map[string]*actorState{}}
}

func (d *Detector) actor(id string) *actorState {
	st, ok := d.state[id]
	if !ok {
		st = &actorState{wfState: map[string]string{}}
		d.state[id] = st
	}
	return st
}

// Observe feeds one request and returns any findings it triggers (usually zero
// or one). It updates the actor's workflow state and op-trail.
func (d *Detector) Observe(r Request) []Finding {
	op, _, ok := d.Spec.Match(r.Method, r.Path)
	st := d.actor(r.ActorID)
	if !ok {
		// Not in the spec — surface-probing of an undocumented path.
		return []Finding{{
			Kind: FindingUnknownOp, ActorID: r.ActorID, Method: r.Method, Path: r.Path,
			Time:        r.Time,
			Explanation: fmt.Sprintf("%s %s matches no known operation (undocumented-surface probe)", r.Method, r.Path),
			Sequence:    append([]string(nil), st.trail...),
		}}
	}
	st.trail = append(st.trail, op.ID)
	var findings []Finding

	// 1. privilege (role) check
	if op.RequiredRole != "" && roleRank[strings.ToLower(r.Role)] < roleRank[strings.ToLower(op.RequiredRole)] {
		findings = append(findings, Finding{
			Kind: FindingPrivilege, OpID: op.ID, ActorID: r.ActorID, Method: r.Method, Path: r.Path,
			Time:        r.Time,
			Explanation: fmt.Sprintf("actor role %q is below required role %q for %s", r.Role, op.RequiredRole, op.ID),
			Sequence:    append([]string(nil), st.trail...),
		})
	}

	// 2. object-level authorization (BOLA/IDOR) check
	if op.OwnershipParam != "" && !r.ObjectOwned {
		findings = append(findings, Finding{
			Kind: FindingBOLA, OpID: op.ID, ActorID: r.ActorID, Method: r.Method, Path: r.Path,
			Time:        r.Time,
			Explanation: fmt.Sprintf("actor accessed %s %q without authorization to the referenced %s", op.ID, r.Path, firstNonEmpty(op.ResourceType, "object")),
			Sequence:    append([]string(nil), st.trail...),
		})
	}

	// 3. workflow sequence check
	for _, wf := range d.Spec.Workflows {
		if !opInWorkflow(wf, op.ID) {
			continue
		}
		cur, seen := st.wfState[wf.ID]
		if !seen {
			cur = wf.Start
		}
		if next, ok := allowedTransition(wf, cur, op.ID); ok {
			st.wfState[wf.ID] = next
		} else {
			findings = append(findings, Finding{
				Kind: FindingSequence, OpID: op.ID, ActorID: r.ActorID, Method: r.Method, Path: r.Path,
				Time:        r.Time,
				Explanation: fmt.Sprintf("operation %s is not a valid step from workflow %q state %q", op.ID, wf.ID, cur),
				Sequence:    append([]string(nil), st.trail...),
			})
		}
	}
	return findings
}

func opInWorkflow(wf Workflow, opID string) bool {
	for _, trs := range wf.Transitions {
		for _, t := range trs {
			if t.OpID == opID {
				return true
			}
		}
	}
	return false
}

func allowedTransition(wf Workflow, state, opID string) (string, bool) {
	for _, t := range wf.Transitions[state] {
		if t.OpID == opID {
			return t.To, true
		}
	}
	return "", false
}

// ActorTrail returns the ordered op-id trail observed for an actor.
func (d *Detector) ActorTrail(actorID string) []string {
	st, ok := d.state[actorID]
	if !ok {
		return nil
	}
	return append([]string(nil), st.trail...)
}

func firstNonEmpty(xs ...string) string {
	for _, x := range xs {
		if strings.TrimSpace(x) != "" {
			return x
		}
	}
	return ""
}

// SortFindings orders findings by time then kind for stable reporting.
func SortFindings(fs []Finding) {
	sort.SliceStable(fs, func(i, j int) bool {
		if !fs[i].Time.Equal(fs[j].Time) {
			return fs[i].Time.Before(fs[j].Time)
		}
		return fs[i].Kind < fs[j].Kind
	})
}
