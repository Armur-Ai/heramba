package evidence

import (
	"errors"
	"fmt"
)

// Validation rejects malformed evidence *safely*: it returns a typed error and
// never panics, so a hostile or corrupt record cannot crash ingestion. The
// store refuses to persist anything that fails validation.

// ErrInvalid is the sentinel wrapped by every validation failure.
var ErrInvalid = errors.New("invalid evidence")

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, args...))
}

// Validate checks a SecurityEvent for the invariants the intelligence plane
// depends on.
func (e SecurityEvent) Validate() error {
	if e.Schema != SchemaVersion {
		return invalid("security_event schema %d != %d", e.Schema, SchemaVersion)
	}
	if e.ID == "" {
		return invalid("security_event missing id")
	}
	if e.Kind == "" {
		return invalid("security_event missing kind")
	}
	if !e.Confidence.Valid() {
		return invalid("security_event confidence %v out of [0,1]", e.Confidence)
	}
	if !e.Provenance.Valid() {
		return invalid("security_event missing provenance (sensor + observed_at required)")
	}
	return nil
}

// Validate checks a Fact: it must restate at least one SecurityEvent, since a
// Fact with no derivation is by definition an unsupported claim.
func (f Fact) Validate() error {
	if f.Schema != SchemaVersion {
		return invalid("fact schema %d != %d", f.Schema, SchemaVersion)
	}
	if f.ID == "" {
		return invalid("fact missing id")
	}
	if f.Statement == "" {
		return invalid("fact missing statement")
	}
	if len(f.DerivedFrom) == 0 {
		return invalid("fact %q derives from no observations", f.Statement)
	}
	return nil
}

// Validate checks a Hypothesis.
func (h *Hypothesis) Validate() error {
	if h == nil {
		return invalid("nil hypothesis")
	}
	if h.Schema != SchemaVersion {
		return invalid("hypothesis schema %d != %d", h.Schema, SchemaVersion)
	}
	if h.ID == "" {
		return invalid("hypothesis missing id")
	}
	if h.Statement == "" {
		return invalid("hypothesis missing statement")
	}
	if !h.Confidence.Valid() {
		return invalid("hypothesis confidence %v out of [0,1]", h.Confidence)
	}
	if h.Uncertainty < 0 || h.Uncertainty > 1 {
		return invalid("hypothesis uncertainty %v out of [0,1]", h.Uncertainty)
	}
	switch h.Status {
	case StatusOpen, StatusSupported, StatusRejected, StatusExpired, StatusMerged:
	default:
		return invalid("hypothesis unknown status %q", h.Status)
	}
	return nil
}

// Validate checks a Relationship.
func (r Relationship) Validate() error {
	if r.Schema != SchemaVersion {
		return invalid("relationship schema %d != %d", r.Schema, SchemaVersion)
	}
	if r.ID == "" {
		return invalid("relationship missing id")
	}
	if r.Kind == "" {
		return invalid("relationship missing kind")
	}
	if r.From == "" || r.To == "" {
		return invalid("relationship missing endpoint (from=%q to=%q)", r.From, r.To)
	}
	if r.From == r.To {
		return invalid("relationship is a self-loop on %q", r.From)
	}
	if !r.Confidence.Valid() {
		return invalid("relationship confidence %v out of [0,1]", r.Confidence)
	}
	return nil
}
