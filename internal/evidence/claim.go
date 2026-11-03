package evidence

import (
	"math"
	"time"
)

// EvidenceRef points at a stored record (a SecurityEvent, Fact, or another
// Hypothesis) that supports or contradicts a claim. Keeping references rather
// than copies means the provenance chain stays walkable and auditable.
type EvidenceRef struct {
	ID   ID     `json:"id"`
	Note string `json:"note,omitempty"` // why this is (counter)evidence; hostile text, never parsed
}

// Fact is a normalized, validated assertion derived from observations with no
// probabilistic inference. "source 10.0.0.4 requested decoy path /.env" is a
// Fact: restating what SecurityEvents directly witnessed. Confidence is 1.0 by
// construction; a Fact that needs a guess is not a Fact, it is a Hypothesis.
type Fact struct {
	Schema      int       `json:"schema"`
	ID          ID        `json:"id"`
	Statement   string    `json:"statement"` // human-readable normalized assertion
	Subject     string    `json:"subject,omitempty"`
	Predicate   string    `json:"predicate,omitempty"`
	Object      string    `json:"object,omitempty"`
	DerivedFrom []ID      `json:"derived_from"` // SecurityEvent IDs this restates
	CreatedAt   time.Time `json:"created_at"`
}

// NewFact builds a Fact restating the given SecurityEvents. It always carries
// at least one source; a Fact with no derivation is invalid (see Validate).
func NewFact(statement string, from ...ID) Fact {
	return Fact{
		Schema:      SchemaVersion,
		ID:          NewID("fact"),
		Statement:   statement,
		DerivedFrom: append([]ID(nil), from...),
		CreatedAt:   time.Now().UTC(),
	}
}

// Confidence of a Fact is 1.0 by definition.
func (f Fact) ConfidenceValue() Confidence { return 1.0 }

// HypothesisStatus tracks a hypothesis through its life.
type HypothesisStatus string

const (
	StatusOpen      HypothesisStatus = "open"      // under active evaluation
	StatusSupported HypothesisStatus = "supported" // evidence crossed the accept threshold
	StatusRejected  HypothesisStatus = "rejected"  // contradicted or decayed below floor
	StatusExpired   HypothesisStatus = "expired"   // passed review-by with no refresh
	StatusMerged    HypothesisStatus = "merged"    // folded into another (entity resolution)
)

// Hypothesis is an inferred claim. Unlike a Fact it carries *both* supporting
// and contradictory evidence, a confidence, an uncertainty, and an explicit
// review deadline — so nothing inferred is ever mistaken for proven. "These two
// sessions are the same actor" is a Hypothesis, never a Fact.
type Hypothesis struct {
	Schema        int              `json:"schema"`
	ID            ID               `json:"id"`
	Statement     string           `json:"statement"`
	Stage         Stage            `json:"stage,omitempty"`
	Confidence    Confidence       `json:"confidence"`
	Uncertainty   float64          `json:"uncertainty"` // spread/entropy of the estimate, [0,1]
	Supporting    []EvidenceRef    `json:"supporting"`
	Contradicting []EvidenceRef    `json:"contradicting"`
	Status        HypothesisStatus `json:"status"`
	CreatedAt     time.Time        `json:"created_at"`
	UpdatedAt     time.Time        `json:"updated_at"`
	ReviewBy      time.Time        `json:"review_by"` // after this, Expire() applies if untouched
}

// NewHypothesis opens a hypothesis with an initial confidence and a review
// deadline. Confidence is clamped; status starts Open.
func NewHypothesis(statement string, conf Confidence, reviewIn time.Duration) *Hypothesis {
	now := time.Now().UTC()
	return &Hypothesis{
		Schema:      SchemaVersion,
		ID:          NewID("hyp"),
		Statement:   statement,
		Confidence:  conf.Clamp(),
		Uncertainty: 1 - float64(conf.Clamp()), // default: uncertainty mirrors distance from certainty
		Status:      StatusOpen,
		CreatedAt:   now,
		UpdatedAt:   now,
		ReviewBy:    now.Add(reviewIn),
	}
}

// Support adds supporting evidence, nudges confidence up toward 1, and refreshes
// the update stamp. The nudge is bounded and never reaches certainty from
// inference alone.
func (h *Hypothesis) Support(ref EvidenceRef, weight Confidence) {
	h.Supporting = append(h.Supporting, ref)
	h.Confidence = (h.Confidence + (1-h.Confidence)*weight.Clamp()).Clamp()
	h.recompute()
}

// Contradict adds contradictory evidence and pulls confidence down toward 0.
func (h *Hypothesis) Contradict(ref EvidenceRef, weight Confidence) {
	h.Contradicting = append(h.Contradicting, ref)
	h.Confidence = (h.Confidence * (1 - weight.Clamp())).Clamp()
	h.recompute()
}

// Decay applies exponential confidence decay over elapsed time with the given
// per-hour half-life, modeling that stale inference should lose force. A
// hypothesis that decays below floor is Rejected.
func (h *Hypothesis) Decay(elapsed time.Duration, halfLife time.Duration, floor Confidence) {
	if halfLife <= 0 || elapsed <= 0 {
		return
	}
	factor := math.Exp2(-elapsed.Hours() / halfLife.Hours())
	h.Confidence = Confidence(float64(h.Confidence) * factor).Clamp()
	if h.Confidence < floor.Clamp() {
		h.Status = StatusRejected
	}
	h.recompute()
}

// Expire marks the hypothesis expired if its review deadline has passed and it
// is still open. Returns true if it changed state.
func (h *Hypothesis) Expire(now time.Time) bool {
	if h.Status == StatusOpen && !h.ReviewBy.IsZero() && now.After(h.ReviewBy) {
		h.Status = StatusExpired
		h.UpdatedAt = now
		return true
	}
	return false
}

func (h *Hypothesis) recompute() {
	h.UpdatedAt = time.Now().UTC()
	// Uncertainty peaks when confidence is near 0.5 (maximally undecided) and
	// shrinks toward the extremes — a simple, explainable proxy for spread.
	c := float64(h.Confidence)
	h.Uncertainty = 1 - 2*absf(c-0.5)
}

func absf(f float64) float64 { return math.Abs(f) }
