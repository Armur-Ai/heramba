// Package coevolve is the R4 adaptation-race & co-evolution lab. It is a
// strictly local, seeded, event-sourced simulation: a bounded attacker shifts
// strategy, a bounded defender observes → detects → decides → applies → reaches
// effect, and the lab measures the adaptation latency (decomposed into detection
// and response components) and the defender's net utility across repeated
// shifts.
//
// Everything is a harmless surrogate. There are no live targets, no real
// exploitation, no propagation, no credential theft, nothing destructive — only
// logical ticks and labeled events. The event log is append-only and replay is
// deterministic given the seed (asserted in tests); nondeterminism caveats are
// documented where they would apply to a non-seeded run.
package coevolve

import (
	"fmt"
	"math/rand"
)

// Tick is logical simulation time. One tick maps to TickSeconds wall-seconds
// only for reporting; the simulation itself is unitless and deterministic.
type Tick int

// TickSeconds maps a logical tick onto seconds for latency reporting.
const TickSeconds = 1.0

// Phase names the stage of the defender's response pipeline an event marks.
type Phase string

const (
	PhaseChange    Phase = "strategy_change"    // t_change: attacker shifts
	PhaseObserved  Phase = "observed"           // t_observed
	PhaseDetected  Phase = "detected"           // t_detected
	PhaseDecision  Phase = "decision"           // t_decision
	PhaseApplied   Phase = "applied"            // t_applied
	PhaseEffective Phase = "effective"          // t_effective
	PhaseObjective Phase = "attacker_objective" // t_objective (attacker won the round)
	PhaseRollback  Phase = "rollback"           // policy reverted (oscillation)
)

// Event is one entry in the append-only, replayable log.
type Event struct {
	Tick   Tick   `json:"tick"`
	Round  int    `json:"round"`
	Phase  Phase  `json:"phase"`
	Detail string `json:"detail"`
}

// Archetype is a bounded attacker model. Detectability drives how long detection
// takes after a shift; a less detectable adversary (deception-aware, adversarial)
// takes longer to catch. ObjectiveTicks is how long the attacker needs to reach
// its objective while uncountered.
type Archetype struct {
	Name           string  `json:"name"`
	Detectability  float64 `json:"detectability"` // (0,1]; higher = faster detection
	ObjectiveTicks Tick    `json:"objective_ticks"`
	Feints         bool    `json:"feints"` // shifts then reverts, to induce policy oscillation
}

// Archetypes returns the bounded attacker families the lab races against.
func Archetypes() []Archetype {
	return []Archetype{
		{Name: "scripted", Detectability: 0.9, ObjectiveTicks: 25},
		{Name: "independent", Detectability: 0.7, ObjectiveTicks: 22},
		{Name: "coordinated", Detectability: 0.6, ObjectiveTicks: 18},
		{Name: "deception_aware", Detectability: 0.35, ObjectiveTicks: 16},
		{Name: "adversarial_varied", Detectability: 0.3, ObjectiveTicks: 16, Feints: true},
	}
}

// DefenderMode selects the adaptation behavior (for ablations).
type DefenderMode int

const (
	// Adaptive is the real defender: observe→detect→decide→apply→effective with
	// realistic per-phase delays.
	Adaptive DefenderMode = iota
	// NoAdaptation never updates policy; the attacker wins every shift (lower
	// bound / negative control).
	NoAdaptation
	// Instant adapts with zero latency (upper bound; physically unattainable).
	Instant
)

// Params are the defender's response-pipeline delays (in ticks) and the utility
// weights. The response components (decision/apply/effective) are fixed costs of
// rolling out a policy change; the detection component depends on the archetype.
type Params struct {
	ObserveDelay    Tick    // t_observed - t_change
	DetectBase      Tick    // detection delay scales as DetectBase/detectability
	DecisionDelay   Tick    // t_decision - t_detected
	ApplyDelay      Tick    // t_applied - t_decision
	EffectiveDelay  Tick    // t_effective - t_applied
	WObjective      float64 // utility weight of preventing an objective
	FPCostPerUpdate float64 // benign-disruption cost charged per policy update
	ComputeCost     float64 // compute cost charged per policy update
	FeintPenalty    float64 // extra utility cost when a feint causes an oscillation
}

// DefaultParams returns calibrated defaults. The response components sum to a
// fixed rollout cost; only detection varies with the adversary.
func DefaultParams() Params {
	return Params{
		ObserveDelay:    1,
		DetectBase:      3,
		DecisionDelay:   2,
		ApplyDelay:      1,
		EffectiveDelay:  1,
		WObjective:      1.0,
		FPCostPerUpdate: 0.05,
		ComputeCost:     0.02,
		FeintPenalty:    0.3,
	}
}

// RoundResult is the measured outcome of one strategy shift.
type RoundResult struct {
	Round             int     `json:"round"`
	TChange           Tick    `json:"t_change"`
	TDetected         Tick    `json:"t_detected"`
	TEffective        Tick    `json:"t_effective"`
	TObjective        Tick    `json:"t_objective"` // -1 if attacker did not complete
	Countered         bool    `json:"countered"`   // effect reached before objective
	Oscillated        bool    `json:"oscillated"`
	AdaptationLatency Tick    `json:"adaptation_latency"`  // t_effective - t_change
	DetectionComp     Tick    `json:"detection_component"` // t_detected - t_change
	ResponseComp      Tick    `json:"response_component"`  // t_effective - t_detected
	Utility           float64 `json:"utility"`
}

// Sim is one archetype vs the defender across several rounds. It owns the
// append-only event log.
type Sim struct {
	Archetype Archetype
	Mode      DefenderMode
	Params    Params
	Log       []Event
}

func (s *Sim) emit(t Tick, round int, ph Phase, detail string) {
	s.Log = append(s.Log, Event{Tick: t, Round: round, Phase: ph, Detail: detail})
}

// Run races the archetype against the defender for `rounds` strategy shifts,
// spacing shifts far enough apart that each resolves before the next. Returns
// per-round results; the event log is on the Sim. Deterministic given seed.
func (s *Sim) Run(rounds int, seed int64) []RoundResult {
	rng := rand.New(rand.NewSource(seed))
	p := s.Params
	var out []RoundResult
	var clock Tick = 5 // warmup
	const spacing Tick = 40

	for r := 0; r < rounds; r++ {
		tChange := clock
		s.emit(tChange, r, PhaseChange, fmt.Sprintf("%s shifts strategy", s.Archetype.Name))

		// Detection component depends on the defender mode and archetype.
		var tDetected, tEffective Tick
		switch s.Mode {
		case NoAdaptation:
			tDetected = -1
			tEffective = -1
		case Instant:
			tDetected = tChange
			tEffective = tChange
		default: // Adaptive
			obs := tChange + p.ObserveDelay
			// jitter in detection reflects observational noise; bounded and seeded.
			jitter := Tick(rng.Intn(3))
			det := obs + detectDelay(p.DetectBase, s.Archetype.Detectability) + jitter
			tDetected = det
			tEffective = det + p.DecisionDelay + p.ApplyDelay + p.EffectiveDelay
		}

		// Attacker reaches its objective ObjectiveTicks after the shift unless the
		// defender's countermeasure is effective first.
		tObjectiveIfUncountered := tChange + s.Archetype.ObjectiveTicks
		countered := tEffective >= 0 && tEffective <= tObjectiveIfUncountered

		res := RoundResult{Round: r, TChange: tChange, TDetected: tDetected, TEffective: tEffective, TObjective: -1}
		if tDetected >= 0 {
			s.emit(tChange+p.ObserveDelay, r, PhaseObserved, "activity observed")
			s.emit(tDetected, r, PhaseDetected, "new pattern detected")
			s.emit(tDetected+p.DecisionDelay, r, PhaseDecision, "bounded counter proposed")
			s.emit(tDetected+p.DecisionDelay+p.ApplyDelay, r, PhaseApplied, "policy update applied (shadow→enforce)")
			s.emit(tEffective, r, PhaseEffective, "countermeasure effective")
			res.AdaptationLatency = tEffective - tChange
			res.DetectionComp = tDetected - tChange
			res.ResponseComp = tEffective - tDetected
		} else {
			res.AdaptationLatency = -1
		}

		if countered {
			res.Countered = true
		} else {
			res.TObjective = tObjectiveIfUncountered
			s.emit(tObjectiveIfUncountered, r, PhaseObjective, "attacker reached objective before countermeasure")
		}

		// Feints: an adversarial-varied attacker sometimes shifts then reverts,
		// causing the defender to update and then roll back — oscillation.
		if s.Archetype.Feints && s.Mode == Adaptive && rng.Float64() < 0.5 {
			res.Oscillated = true
			s.emit(tEffective+2, r, PhaseRollback, "feint reverted; policy rolled back (oscillation)")
		}

		res.Utility = roundUtility(res, p, s.Mode)
		out = append(out, res)
		clock += spacing
	}
	return out
}

// detectDelay scales inversely with detectability (less detectable → slower).
func detectDelay(base Tick, detectability float64) Tick {
	if detectability <= 0 {
		detectability = 0.1
	}
	return Tick(float64(base) / detectability)
}

// roundUtility = objective-prevention value - (FP + compute) update costs -
// feint oscillation penalty. NoAdaptation pays no update cost but counters
// nothing; Instant counters everything but still pays update costs.
func roundUtility(r RoundResult, p Params, mode DefenderMode) float64 {
	u := 0.0
	if r.Countered {
		u += p.WObjective
	}
	if mode != NoAdaptation {
		u -= p.FPCostPerUpdate + p.ComputeCost
	}
	if r.Oscillated {
		u -= p.FeintPenalty
	}
	return u
}
