package strategy

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Armur-Ai/heramba/internal/evidence"
)

// Executor performs the bounded, real effect of an enforced action and returns
// a rollback token. It is the ONLY place an action has an external effect, and
// it is called only after policy allows, approval is granted (if required), the
// engine is in enforce mode, and the emergency kill-switch is off. In shadow
// mode the executor is never called.
type Executor interface {
	Enforce(a Action) (rollbackToken string, err error)
	Rollback(a Action, token string) error
}

// Clock lets tests control time for expiry.
type Clock func() time.Time

// Engine validates and (optionally) enforces actions under policy. It owns the
// audit log, approvals, expiry, rollback, and the emergency disable.
type Engine struct {
	mu        sync.Mutex
	policy    Policy
	mode      Mode
	exec      Executor
	now       Clock
	disabled  bool // emergency kill-switch: when true, nothing is enforced
	active    map[evidence.ID]*ActionExecution
	audit     []ActionExecution
	auditSink func(ActionExecution)
}

// NewEngine builds an engine. A nil executor is allowed (pure shadow/testing).
func NewEngine(policy Policy, mode Mode, exec Executor) *Engine {
	return &Engine{
		policy: policy,
		mode:   mode,
		exec:   exec,
		now:    func() time.Time { return time.Now().UTC() },
		active: map[evidence.ID]*ActionExecution{},
	}
}

// SetClock overrides the clock (tests).
func (e *Engine) SetClock(c Clock) { e.now = c }

// SetAuditSink registers a callback invoked for every audit record.
func (e *Engine) SetAuditSink(f func(ActionExecution)) { e.auditSink = f }

// EmergencyDisable flips the kill-switch: all enforcement stops and every active
// action is rolled back. Fail-safe for operators.
func (e *Engine) EmergencyDisable() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.disabled = true
	for _, ex := range e.active {
		e.rollbackLocked(ex, "emergency disable")
	}
}

// Enabled reports whether enforcement is currently possible.
func (e *Engine) Enabled() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return !e.disabled
}

// Submit runs an action through the full lifecycle: policy → (shadow? dry-run) →
// (approval?) → enforce → schedule expiry. It returns the resulting execution
// record. Submit never enforces in shadow mode and never enforces when
// disabled. An LLM-proposed action takes exactly this path — no shortcut.
func (e *Engine) Submit(a Action) ActionExecution {
	e.mu.Lock()
	defer e.mu.Unlock()

	dec := e.policy.Evaluate(a)
	ex := ActionExecution{Action: a, Decision: dec, Status: StatusProposed}

	switch dec.Verdict {
	case VerdictDeny:
		ex.Status = StatusRejected
		return e.record(ex)
	case VerdictNeedApproval:
		ex.Status = StatusPending
		ex.Notes = append(ex.Notes, "awaiting operator approval")
		e.active[a.ID] = &ex
		return e.record(ex)
	}

	// Allowed. In shadow mode or when disabled, we validate + audit but do not
	// enforce.
	if e.mode == ModeShadow || e.disabled {
		ex.Status = StatusDryRun
		why := "shadow mode: validated, not enforced"
		if e.disabled {
			why = "enforcement disabled: validated, not enforced"
		}
		ex.Notes = append(ex.Notes, why)
		return e.record(ex)
	}
	return e.enforceLocked(&ex)
}

// Approve enforces a previously-pending action (operator action). It re-checks
// the engine state; a disabled or shadow engine still will not enforce.
func (e *Engine) Approve(id evidence.ID, approver string) (ActionExecution, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	ex, ok := e.active[id]
	if !ok || ex.Status != StatusPending {
		return ActionExecution{}, errors.New("strategy: no pending action with that id")
	}
	ex.Notes = append(ex.Notes, "approved by "+approver)
	if e.mode == ModeShadow || e.disabled {
		ex.Status = StatusDryRun
		ex.Notes = append(ex.Notes, "approved but not enforced (shadow/disabled)")
		return e.record(*ex), nil
	}
	return e.enforceLocked(ex), nil
}

// enforceLocked performs the actual enforcement via the executor and schedules
// expiry. Caller holds the lock.
func (e *Engine) enforceLocked(ex *ActionExecution) ActionExecution {
	if e.exec == nil {
		ex.Status = StatusDryRun
		ex.Notes = append(ex.Notes, "no executor wired: validated, not enforced")
		return e.record(*ex)
	}
	token, err := e.exec.Enforce(ex.Action)
	if err != nil {
		ex.Status = StatusRejected
		ex.Notes = append(ex.Notes, "executor error (fail-safe, no enforcement): "+err.Error())
		return e.record(*ex)
	}
	now := e.now()
	ex.Status = StatusEnforced
	ex.EnforcedAt = now
	ex.ExpiresAt = now.Add(ex.Action.Duration)
	ex.RollbackToken = token
	e.active[ex.Action.ID] = ex
	return e.record(*ex)
}

// Tick expires actions whose duration has elapsed, rolling them back. Operators
// (or a background loop) call it periodically. It returns expired executions.
func (e *Engine) Tick() []ActionExecution {
	e.mu.Lock()
	defer e.mu.Unlock()
	now := e.now()
	var expired []ActionExecution
	for id, ex := range e.active {
		if ex.Status == StatusEnforced && !ex.ExpiresAt.IsZero() && now.After(ex.ExpiresAt) {
			e.rollbackLocked(ex, "duration expired")
			ex.Status = StatusExpired
			expired = append(expired, *ex)
			delete(e.active, id)
		}
	}
	return expired
}

// Rollback manually reverses an enforced action.
func (e *Engine) Rollback(id evidence.ID, reason string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	ex, ok := e.active[id]
	if !ok {
		return errors.New("strategy: no active action with that id")
	}
	e.rollbackLocked(ex, reason)
	delete(e.active, id)
	return nil
}

func (e *Engine) rollbackLocked(ex *ActionExecution, reason string) {
	if ex.Status != StatusEnforced {
		return
	}
	if e.exec != nil && ex.RollbackToken != "" {
		if err := e.exec.Rollback(ex.Action, ex.RollbackToken); err != nil {
			ex.Notes = append(ex.Notes, "rollback error: "+err.Error())
		}
	}
	ex.Status = StatusRolledBack
	ex.RolledBackAt = e.now()
	ex.Notes = append(ex.Notes, "rolled back: "+reason)
	e.record(*ex)
}

// record appends to the audit log and fans out to the sink. Caller holds lock.
func (e *Engine) record(ex ActionExecution) ActionExecution {
	e.audit = append(e.audit, ex)
	if e.auditSink != nil {
		e.auditSink(ex)
	}
	return ex
}

// Audit returns a copy of the audit log.
func (e *Engine) Audit() []ActionExecution {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]ActionExecution(nil), e.audit...)
}

// newRollbackToken is used by executors that want a random token.
func newRollbackToken() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// recordingExecutor is a test/simple executor that just tracks enforced scopes.
type recordingExecutor struct {
	mu       sync.Mutex
	enforced map[string]Action
}

// NewRecordingExecutor returns an Executor that records enforcement in memory —
// useful for shadow-to-enforce testing and for a honeypot-only deployment where
// the "effect" is purely internal state.
func NewRecordingExecutor() *recordingExecutor {
	return &recordingExecutor{enforced: map[string]Action{}}
}

func (r *recordingExecutor) Enforce(a Action) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	tok := newRollbackToken()
	r.enforced[tok] = a
	return tok, nil
}

func (r *recordingExecutor) Rollback(a Action, token string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.enforced[token]; !ok {
		return fmt.Errorf("unknown rollback token %q", token)
	}
	delete(r.enforced, token)
	return nil
}

// ActiveCount reports how many actions the recording executor currently holds.
func (r *recordingExecutor) ActiveCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.enforced)
}
