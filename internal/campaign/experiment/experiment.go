// Package experiment implements bounded active learning: when the evidence is
// ambiguous, Heramba can run a *safe decoy experiment* to discriminate between
// competing explanations — but only within an explicit budget, only against the
// synthetic surface, and never by manipulating a legitimate business response.
//
// The math is deliberately simple and auditable: competing explanations carry
// probabilities, an experiment has a likelihood model P(outcome | explanation),
// and expected information gain is the expected reduction in Shannon entropy.
// The registry picks the highest-information experiment it can still afford,
// then updates posteriors by Bayes' rule from the observed outcome.
package experiment

import (
	"errors"
	"math"
	"sort"
)

// HypothesisSet is a normalized probability distribution over competing
// explanations for an observation (e.g. {"agent":0.5,"human":0.5}).
type HypothesisSet struct {
	p map[string]float64
}

// NewHypothesisSet builds a set from (unnormalized) priors; it normalizes.
func NewHypothesisSet(priors map[string]float64) (*HypothesisSet, error) {
	if len(priors) < 2 {
		return nil, errors.New("experiment: need at least two competing explanations")
	}
	h := &HypothesisSet{p: map[string]float64{}}
	var sum float64
	for k, v := range priors {
		if v < 0 {
			return nil, errors.New("experiment: negative prior for " + k)
		}
		h.p[k] = v
		sum += v
	}
	if sum == 0 {
		return nil, errors.New("experiment: priors sum to zero")
	}
	for k := range h.p {
		h.p[k] /= sum
	}
	return h, nil
}

// Prob returns the current probability of an explanation.
func (h *HypothesisSet) Prob(id string) float64 { return h.p[id] }

// Entropy returns the Shannon entropy (bits) of the current distribution. It is
// the quantity experiments aim to reduce.
func (h *HypothesisSet) Entropy() float64 {
	var e float64
	for _, p := range h.p {
		if p > 0 {
			e -= p * math.Log2(p)
		}
	}
	return e
}

// MostLikely returns the leading explanation and its probability.
func (h *HypothesisSet) MostLikely() (string, float64) {
	best, bestP := "", -1.0
	for _, id := range h.sortedIDs() {
		if h.p[id] > bestP {
			best, bestP = id, h.p[id]
		}
	}
	return best, bestP
}

func (h *HypothesisSet) sortedIDs() []string {
	ids := make([]string, 0, len(h.p))
	for id := range h.p {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func (h *HypothesisSet) clone() *HypothesisSet {
	c := &HypothesisSet{p: map[string]float64{}}
	for k, v := range h.p {
		c.p[k] = v
	}
	return c
}

// update applies Bayes' rule given per-explanation likelihoods of the observed
// outcome and renormalizes.
func (h *HypothesisSet) update(likelihood map[string]float64) {
	var sum float64
	next := map[string]float64{}
	for id, prior := range h.p {
		l := likelihood[id]
		next[id] = prior * l
		sum += next[id]
	}
	if sum == 0 {
		return // outcome impossible under all explanations; leave unchanged
	}
	for id := range next {
		h.p[id] = next[id] / sum
	}
}

// Experiment is a safe decoy probe. Likelihood[explanation][outcome] is
// P(outcome | explanation). Outcomes must be the same key set across
// explanations. An experiment never touches a real response — the caller runs
// it against the synthetic surface only.
type Experiment struct {
	ID         string
	Desc       string
	Likelihood map[string]map[string]float64 // explanation → outcome → probability
}

// outcomes returns the union of outcome labels.
func (e *Experiment) outcomes() []string {
	seen := map[string]struct{}{}
	for _, m := range e.Likelihood {
		for o := range m {
			seen[o] = struct{}{}
		}
	}
	var out []string
	for o := range seen {
		out = append(out, o)
	}
	sort.Strings(out)
	return out
}

// ExpectedInfoGain returns the expected entropy reduction (bits) from running
// the experiment against the current hypothesis set. Higher is better.
func (e *Experiment) ExpectedInfoGain(h *HypothesisSet) float64 {
	before := h.Entropy()
	var expectedAfter float64
	for _, o := range e.outcomes() {
		// P(outcome) = Σ P(expl) P(outcome|expl)
		var po float64
		for id, prior := range h.p {
			po += prior * e.Likelihood[id][o]
		}
		if po <= 0 {
			continue
		}
		// posterior entropy if this outcome occurs
		post := h.clone()
		lk := map[string]float64{}
		for id := range h.p {
			lk[id] = e.Likelihood[id][o]
		}
		post.update(lk)
		expectedAfter += po * post.Entropy()
	}
	gain := before - expectedAfter
	if gain < 0 {
		gain = 0
	}
	return gain
}

// Budget bounds how many experiments may run (per target/window). It is a hard
// safety limit — active learning never runs unbounded.
type Budget struct {
	Max  int
	used int
}

// NewBudget returns a budget with the given cap.
func NewBudget(max int) *Budget { return &Budget{Max: max} }

// Remaining reports how many experiments may still run.
func (b *Budget) Remaining() int { return b.Max - b.used }

// Registry runs the active-learning loop over a hypothesis set within a budget.
type Registry struct {
	H       *HypothesisSet
	Budget  *Budget
	history []string
}

// NewRegistry builds a registry.
func NewRegistry(h *HypothesisSet, b *Budget) *Registry {
	return &Registry{H: h, Budget: b}
}

// MinGain is the smallest expected information gain (bits) worth spending a
// budget unit on. Experiments below this are not run.
const MinGain = 0.01

// Pick selects the affordable experiment with the highest expected information
// gain above MinGain. It returns (nil, false) if the budget is exhausted or no
// experiment is informative enough.
func (r *Registry) Pick(candidates []Experiment) (*Experiment, bool) {
	if r.Budget.Remaining() <= 0 {
		return nil, false
	}
	var best *Experiment
	var bestGain float64
	for i := range candidates {
		g := candidates[i].ExpectedInfoGain(r.H)
		if g > bestGain {
			bestGain, best = g, &candidates[i]
		}
	}
	if best == nil || bestGain < MinGain {
		return nil, false
	}
	return best, true
}

// Run records an experiment's observed outcome, updates posteriors by Bayes,
// and spends one budget unit. It errors if the budget is exhausted.
func (r *Registry) Run(e *Experiment, outcome string) error {
	if r.Budget.Remaining() <= 0 {
		return errors.New("experiment: budget exhausted")
	}
	lk := map[string]float64{}
	for id := range r.H.p {
		lk[id] = e.Likelihood[id][outcome]
	}
	r.H.update(lk)
	r.Budget.used++
	r.history = append(r.history, e.ID+"="+outcome)
	return nil
}

// History returns the ordered experiment/outcome trail (audit).
func (r *Registry) History() []string { return append([]string(nil), r.history...) }
