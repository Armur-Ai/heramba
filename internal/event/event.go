// Package event defines Heramba's detection event model. Events are the single
// currency the engine emits: a decoy hit, a triggered canary, an agentic-traffic
// score crossing threshold, a blocked egress callback.
//
// A core design rule: deception signals are high-confidence by construction.
// Nobody legitimately touches a decoy endpoint or a canary token, so a decoy
// hit carries Confidence 1.0 — "zero false positive" is a property of what is
// observed, not of a model's self-assurance.
package event

import "time"

// Severity ranks an event's urgency.
type Severity int

const (
	SeverityInfo Severity = iota
	SeverityLow
	SeverityMedium
	SeverityHigh
	SeverityCritical
)

func (s Severity) String() string {
	switch s {
	case SeverityInfo:
		return "info"
	case SeverityLow:
		return "low"
	case SeverityMedium:
		return "medium"
	case SeverityHigh:
		return "high"
	case SeverityCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// Kind classifies what was detected.
type Kind string

const (
	KindDecoyHit        Kind = "decoy_hit"        // a request touched a decoy endpoint
	KindCanaryTriggered Kind = "canary_triggered" // a planted canary token was used
	KindAgenticTraffic  Kind = "agentic_traffic"  // behavioral classifier crossed threshold
	KindEgressBlocked   Kind = "egress_blocked"   // an out-of-policy callback was denied
	KindRateLimited     Kind = "rate_limited"     // a source exceeded its request-rate budget
	KindCounterDeployed Kind = "counter_deployed" // a counter-agent prompt decoy was injected
	KindProbe           Kind = "probe"            // suspicious probing of real surface
)

// Event is one detection. It is JSON-serializable for alert sinks and audit.
type Event struct {
	Time       time.Time         `json:"time"`
	Kind       Kind              `json:"kind"`
	Severity   Severity          `json:"severity"`
	SeverityS  string            `json:"severity_label"`
	Source     string            `json:"source,omitempty"` // client IP / identity
	Target     string            `json:"target,omitempty"` // decoy path / token / endpoint touched
	Confidence float64           `json:"confidence"`       // 0..1; deception hits = 1.0
	Detail     string            `json:"detail,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`
}

// New builds an event, stamping the time and the severity label.
func New(kind Kind, sev Severity, source, target, detail string) Event {
	return Event{
		Time:      time.Now().UTC(),
		Kind:      kind,
		Severity:  sev,
		SeverityS: sev.String(),
		Source:    source,
		Target:    target,
		Detail:    detail,
		// Confidence defaults to 0; deception constructors set 1.0 explicitly.
	}
}

// Deception builds a high-confidence (1.0) event for a decoy/canary hit.
func Deception(kind Kind, source, target, detail string) Event {
	e := New(kind, SeverityHigh, source, target, detail)
	e.Confidence = 1.0
	return e
}

// WithLabel attaches a label and returns the event (chainable).
func (e Event) WithLabel(k, v string) Event {
	if e.Labels == nil {
		e.Labels = map[string]string{}
	}
	e.Labels[k] = v
	return e
}
