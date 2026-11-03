package main

import (
	"sync"

	"github.com/Armur-Ai/heramba/internal/event"
	"github.com/Armur-Ai/heramba/internal/evidence"
	"github.com/Armur-Ai/heramba/internal/strategy"
)

// shadowAdvisor runs the strategy engine inline from `serve`, in SHADOW mode:
// it watches the live detection stream, and at shutdown proposes bounded
// containment actions for the sources it saw and runs them through the real
// policy — validating and auditing them, but NEVER enforcing (shadow is the
// safe default). This surfaces "what Heramba would do" to the operator without
// taking any inline action, closing the gap between detection and the
// (opt-in, approval-gated) response engine.
type shadowAdvisor struct {
	mu       sync.Mutex
	bySource map[string]*srcAgg
}

type srcAgg struct {
	agentic bool
	decoy   bool
	highSev int
	evID    evidence.ID
}

func newShadowAdvisor() *shadowAdvisor {
	return &shadowAdvisor{bySource: map[string]*srcAgg{}}
}

// Emit implements alert.Sink; it only accumulates, never blocks or errors, so it
// is safe on the hot path.
func (a *shadowAdvisor) Emit(e event.Event) error {
	if e.Source == "" {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	agg := a.bySource[e.Source]
	if agg == nil {
		agg = &srcAgg{evID: evidence.NewID("se")}
		a.bySource[e.Source] = agg
	}
	switch e.Kind {
	case event.KindDecoyHit, event.KindCanaryTriggered:
		agg.decoy = true
	case event.KindAgenticTraffic:
		agg.agentic = true
	}
	if e.Severity >= event.SeverityHigh {
		agg.highSev++
	}
	return nil
}

// Propose builds a Situation per source, plans bounded actions, and runs them
// through a shadow engine. It returns the audited (dry-run) executions.
func (a *shadowAdvisor) Propose() []strategy.ActionExecution {
	a.mu.Lock()
	defer a.mu.Unlock()
	planner := strategy.DefaultPlanner()
	eng := strategy.NewEngine(strategy.DefaultPolicy(), strategy.ModeShadow, strategy.NewRecordingExecutor())
	for src, agg := range a.bySource {
		score := 0.0
		if agg.agentic {
			score = 0.85 // an agentic-suspected event crossed the classifier threshold
		}
		sit := strategy.Situation{
			SourceKey:         src,
			AgenticScore:      score,
			DeceptionHit:      agg.decoy,
			HighSeverityCount: agg.highSev,
			Evidence:          []evidence.EvidenceRef{{ID: agg.evID, Note: "live detection"}},
		}
		for _, act := range planner.Plan(sit) {
			eng.Submit(act)
		}
	}
	return eng.Audit()
}
