package evidence

import "time"

// RelKind is the type of a typed, directed edge between two evidence records.
// Edges are how the campaign graph (CP2) walks from an event to the actor it
// implicates, from one session to the next, from a hypothesis to its support.
type RelKind string

const (
	RelDerivedFrom RelKind = "derived_from" // fact/hypothesis ← security event
	RelSupports    RelKind = "supports"     // evidence → hypothesis (raises confidence)
	RelContradicts RelKind = "contradicts"  // evidence → hypothesis (lowers confidence)
	RelSameSource  RelKind = "same_source"  // two events share a transport source key (Fact)
	RelSameActor   RelKind = "same_actor"   // inferred identity link (Hypothesis — never a Fact)
	RelPrecedes    RelKind = "precedes"     // temporal ordering within a session/campaign
	RelPartOf      RelKind = "part_of"      // event/session → campaign
	RelTargets     RelKind = "targets"      // event → asset/decoy
)

// Relationship is a typed, directed, timestamped edge with its own provenance
// and confidence. Edges that encode inference (RelSameActor) carry < 1.0
// confidence; edges that restate observation (RelSameSource) carry 1.0.
type Relationship struct {
	Schema     int        `json:"schema"`
	ID         ID         `json:"id"`
	Kind       RelKind    `json:"kind"`
	From       ID         `json:"from"`
	To         ID         `json:"to"`
	Confidence Confidence `json:"confidence"`
	CreatedAt  time.Time  `json:"created_at"`
	Note       string     `json:"note,omitempty"`
}

// NewRelationship builds a typed edge. Inference edges should pass a confidence
// below 1.0; observational edges pass 1.0.
func NewRelationship(kind RelKind, from, to ID, conf Confidence) Relationship {
	return Relationship{
		Schema:     SchemaVersion,
		ID:         NewID("rel"),
		Kind:       kind,
		From:       from,
		To:         to,
		Confidence: conf.Clamp(),
		CreatedAt:  time.Now().UTC(),
	}
}

// IsInference reports whether this edge encodes a probabilistic claim (as
// opposed to a direct restatement of observation).
func (r Relationship) IsInference() bool {
	switch r.Kind {
	case RelSameActor, RelSupports, RelContradicts, RelTargets:
		return true
	default:
		return false
	}
}
