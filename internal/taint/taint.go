// Package taint is Heramba's data-plane safety layer (R5). Its premise is that
// everything an attacker can influence — URLs, headers, logs, telemetry,
// retrieved pages, decoy content, tool outputs — is hostile, and must be unable
// to change a policy decision or gain an execution privilege.
//
// The mechanism is a typed taint label that travels with every value and
// propagates through derivation. A decision may read only values it can prove
// are Trusted; attacker-controlled content is structurally inaccessible to the
// trusted path (you cannot forget to check — there is no trusted accessor that
// returns it). This makes prompt injection a type error, not a judgment call.
//
// Companion guards cover the rest of the directive's battery: an explicit,
// fail-closed tool allowlist; a replay guard; tenant isolation; and a log
// sanitizer. No offensive payloads live here — tests use inert fixtures.
package taint

import (
	"errors"
	"regexp"
	"strings"
	"sync"
)

// Label is the trust classification of a value.
type Label int

const (
	// Trusted values originate inside the defender's own typed contracts.
	Trusted Label = iota
	// AttackerControlled values originate from anything an attacker can influence.
	AttackerControlled
	// Derived values were computed from at least one attacker-controlled input.
	// Derived is treated as untrusted for every decision purpose.
	Derived
)

func (l Label) String() string {
	switch l {
	case Trusted:
		return "trusted"
	case AttackerControlled:
		return "attacker_controlled"
	default:
		return "derived"
	}
}

// Provenance names where a value came from so a human can audit back to ground
// truth, and carries the tenant for isolation.
type Provenance struct {
	Sensor string `json:"sensor"`
	RawRef string `json:"raw_ref"`
	Tenant string `json:"tenant"`
}

// Tainted wraps a value together with its trust label and provenance. The inner
// value is never exposed as trusted unless the label is Trusted — there is no
// accessor that hands out attacker-controlled content on the trusted path.
type Tainted[T any] struct {
	value T
	label Label
	prov  Provenance
}

// AttackerValue tags a value as attacker-controlled (the default assumption for
// any external input).
func AttackerValue[T any](v T, p Provenance) Tainted[T] {
	return Tainted[T]{value: v, label: AttackerControlled, prov: p}
}

// TrustedValue tags a value as trusted. Reserved for data the defender itself
// produced under its own typed contracts — never for parsed external input.
func TrustedValue[T any](v T, p Provenance) Tainted[T] {
	return Tainted[T]{value: v, label: Trusted, prov: p}
}

// Label returns the trust classification.
func (t Tainted[T]) Label() Label { return t.label }

// Provenance returns the audit provenance.
func (t Tainted[T]) Provenance() Provenance { return t.prov }

// IsTrusted reports whether the value is safe for a decision.
func (t Tainted[T]) IsTrusted() bool { return t.label == Trusted }

// Trusted returns the value only if it is trusted; otherwise it returns the zero
// value and false. This is the ONLY accessor a decision path may use.
func (t Tainted[T]) Trusted() (T, bool) {
	if t.label == Trusted {
		return t.value, true
	}
	var zero T
	return zero, false
}

// Expose returns the underlying value regardless of taint. It is for logging and
// display ONLY and the caller must sanitize before emitting; using it on a
// decision path defeats the whole mechanism. Named loudly on purpose.
func (t Tainted[T]) Expose() T { return t.value }

// DeriveString computes a derived string from a tainted value; the result is
// Derived (untrusted) whenever the source was not trusted — taint propagates, it
// never launders away through a transformation.
func DeriveString[T any](src Tainted[T], f func(T) string) Tainted[string] {
	out := Tainted[string]{value: f(src.value), prov: src.prov}
	if src.label == Trusted {
		out.label = Trusted
	} else {
		out.label = Derived
	}
	return out
}

// Errors.
var (
	ErrToolNotAllowed   = errors.New("taint: tool not on the allowlist (fail closed)")
	ErrUntrustedDecision = errors.New("taint: decision input is not trusted")
	ErrCrossTenant      = errors.New("taint: cross-tenant data use")
	ErrReplay           = errors.New("taint: replayed nonce")
)

// ToolAllowlist is an explicit, fail-closed allowlist of tool names. Anything
// not explicitly allowed is denied.
type ToolAllowlist struct {
	allowed map[string]struct{}
}

// NewToolAllowlist builds an allowlist from the given tool names.
func NewToolAllowlist(tools ...string) ToolAllowlist {
	m := make(map[string]struct{}, len(tools))
	for _, t := range tools {
		m[t] = struct{}{}
	}
	return ToolAllowlist{allowed: m}
}

// Authorize returns nil only if the tool is explicitly allowed.
func (a ToolAllowlist) Authorize(tool string) error {
	if _, ok := a.allowed[tool]; ok {
		return nil
	}
	return ErrToolNotAllowed
}

// RequireTrusted is a decision guard: it returns the value only if trusted, else
// a wrapped error. Decision code calls this instead of touching Expose.
func RequireTrusted[T any](t Tainted[T]) (T, error) {
	if v, ok := t.Trusted(); ok {
		return v, nil
	}
	var zero T
	return zero, ErrUntrustedDecision
}

// SameTenant enforces tenant isolation: two provenances must share a non-empty
// tenant for one to inform a decision about the other.
func SameTenant(a, b Provenance) error {
	if a.Tenant == "" || b.Tenant == "" || a.Tenant != b.Tenant {
		return ErrCrossTenant
	}
	return nil
}

// ReplayGuard rejects a nonce it has already seen (defends evidence/telemetry
// replay). Concurrency-safe.
type ReplayGuard struct {
	mu   sync.Mutex
	seen map[string]struct{}
}

// NewReplayGuard creates an empty guard.
func NewReplayGuard() *ReplayGuard { return &ReplayGuard{seen: map[string]struct{}{}} }

// Admit returns nil the first time a nonce is seen and ErrReplay thereafter.
func (g *ReplayGuard) Admit(nonce string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, ok := g.seen[nonce]; ok {
		return ErrReplay
	}
	g.seen[nonce] = struct{}{}
	return nil
}

var controlChars = regexp.MustCompile(`[\x00-\x08\x0b\x0c\x0e-\x1f\x7f]`)

// SanitizeForLog neutralizes log-injection: it strips control characters, ANSI
// escapes, and CR/LF so attacker-controlled text cannot forge log lines, move
// the cursor, or inject terminal escapes. It is the only safe way to render an
// Expose()'d value.
func SanitizeForLog(s string) string {
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\x1b", "")
	s = controlChars.ReplaceAllString(s, "")
	const max = 512
	if len(s) > max {
		s = s[:max] + "…"
	}
	return s
}
