package alert

import (
	"strings"

	"github.com/Armur-Ai/heramba/internal/event"
)

// DefaultMaxFieldLen is the default cap applied by RedactingSink to free-text
// fields (Detail, Target) and to each label value, so an attacker-influenced
// field can't blow up a webhook/SIEM payload or an alert channel.
const DefaultMaxFieldLen = 4096

// truncateRunes returns s capped at max runes, appending an ellipsis marker when
// it had to cut. It is rune-safe (never splits a multi-byte rune) and a no-op for
// max <= 0 or strings already within the cap.
func truncateRunes(s string, max int) string {
	if max <= 0 || s == "" {
		return s
	}
	if len(s) <= max { // fast path: byte length bounds rune count
		return s
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	const marker = "…[truncated]"
	keep := max
	if keep > len(marker) {
		keep = max - len([]rune(marker))
		if keep < 0 {
			keep = 0
		}
	}
	return string(r[:keep]) + marker
}

// sensitiveLabel reports whether a label key looks like it carries a secret, so
// its value is masked rather than forwarded to a sink. Detection is heuristic and
// case-insensitive.
func sensitiveLabel(key string) bool {
	k := strings.ToLower(key)
	for _, needle := range []string{"token", "secret", "password", "passwd", "apikey", "api_key", "authorization", "credential", "private_key"} {
		if strings.Contains(k, needle) {
			return true
		}
	}
	return false
}

// RedactEvent returns a copy of e with free-text fields truncated to max runes
// and any secret-looking label values masked. The original event (and its Labels
// map) is never mutated, so it is safe to call before fanning out to sinks. A
// max <= 0 disables truncation but still masks secrets.
func RedactEvent(e event.Event, max int) event.Event {
	e.Detail = truncateRunes(e.Detail, max)
	e.Target = truncateRunes(e.Target, max)
	if len(e.Labels) == 0 {
		return e
	}
	labels := make(map[string]string, len(e.Labels))
	for k, v := range e.Labels {
		if sensitiveLabel(k) {
			labels[k] = "[redacted]"
			continue
		}
		labels[k] = truncateRunes(v, max)
	}
	e.Labels = labels
	return e
}

// RedactingSink wraps any Sink and redacts each event (truncating oversized
// free-text fields and masking secret-looking labels) before it reaches the
// inner sink. It is an optional, additive wrapper — the inner sink's constructor
// is unchanged — and is placed in front of a terminal sink so nothing oversized
// or sensitive is ever sent or logged.
type RedactingSink struct {
	inner Sink
	max   int
}

// NewRedacting wraps inner with field redaction. max <= 0 uses DefaultMaxFieldLen.
func NewRedacting(inner Sink, max int) *RedactingSink {
	if inner == nil {
		inner = Discard
	}
	if max <= 0 {
		max = DefaultMaxFieldLen
	}
	return &RedactingSink{inner: inner, max: max}
}

// Emit redacts the event and forwards it.
func (s *RedactingSink) Emit(e event.Event) error {
	return s.inner.Emit(RedactEvent(e, s.max))
}

// Flush forwards to the inner sink's Flush (if any).
func (s *RedactingSink) Flush() error { return FlushAll(s.inner) }
