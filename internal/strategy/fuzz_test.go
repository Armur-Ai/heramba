package strategy

import (
	"testing"
	"time"

	"github.com/Armur-Ai/heramba/internal/evidence"
)

// FuzzPolicyEvaluate ensures policy evaluation never panics and never ALLOWS an
// action with an unbounded (zero or ambiguous) scope — the core injection-proof
// safety invariant must hold for arbitrary field values.
func FuzzPolicyEvaluate(f *testing.F) {
	f.Add("rate_limit", "src", "", "", "low", int64(time.Minute), 1)
	f.Add("observe", "", "", "", "none", int64(0), 0)
	f.Add("delete_everything", "a", "b", "c", "high", int64(1<<62), 3)
	f.Add("", "", "", "", "", int64(-5), 0)
	p := DefaultPolicy()
	f.Fuzz(func(t *testing.T, typ, src, sess, decoy, impact string, durNanos int64, nEvidence int) {
		a := Action{
			Type:     ActionType(typ),
			Scope:    Scope{SourceKey: src, SessionID: sess, DecoyID: decoy},
			Impact:   Impact(impact),
			Duration: time.Duration(durNanos),
		}
		if nEvidence > 0 && nEvidence < 100 {
			for i := 0; i < nEvidence; i++ {
				a.Evidence = append(a.Evidence, evidence.EvidenceRef{})
			}
		}
		d := p.Evaluate(a) // must not panic

		// Invariant 1: an unbounded scope can NEVER be allowed or approved.
		if kind, id := a.Scope.Subject(); kind == "" || id == "" {
			if d.Verdict != VerdictDeny {
				t.Fatalf("unbounded scope must be denied, got %q", d.Verdict)
			}
		}
		// Invariant 2: a type outside the closed allow-set can never be allowed.
		if !p.AllowedTypes[a.Type] && d.Verdict != VerdictDeny {
			t.Fatalf("disallowed type %q must be denied, got %q", a.Type, d.Verdict)
		}
		// Invariant 3: an allowed state-changing action must have bounded duration.
		if d.Verdict == VerdictAllow && changesState(a.Type) {
			if a.Duration <= 0 || a.Duration > p.MaxDuration {
				t.Fatalf("allowed action has out-of-bounds duration %s", a.Duration)
			}
		}
	})
}
