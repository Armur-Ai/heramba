package evidence

import (
	"time"

	"github.com/Armur-Ai/heramba/internal/event"
)

// Stage is an attack-stage label (loosely kill-chain / ATT&CK-aligned). It is a
// *hypothesis dimension*, never asserted as fact from a single event.
type Stage string

const (
	StageUnknown        Stage = "unknown"
	StageRecon          Stage = "recon"           // surface enumeration, fingerprinting
	StageAccess         Stage = "access"          // auth probing, credential use
	StageDiscovery      Stage = "discovery"       // post-access mapping
	StageExfiltration   Stage = "exfiltration"    // data egress / OOB callback
	StageImpact         Stage = "impact"          // destructive / objective completion
	StageDeceptionTouch Stage = "deception_touch" // touched a decoy / canary (zero-FP)
)

// SecurityEvent is the normalized, versioned currency of the intelligence
// plane. It wraps the raw detail of a data-plane detection with the metadata
// correlation needs: a stable ID, provenance, a source *key* (not an identity),
// an optional target, a confidence, a tentative stage, and free-form
// attributes. It is the atomic Observation unit that higher layers link.
type SecurityEvent struct {
	Schema     int               `json:"schema"`
	ID         ID                `json:"id"`
	Kind       event.Kind        `json:"kind"`
	Severity   event.Severity    `json:"severity"`
	Stage      Stage             `json:"stage"`
	SourceKey  string            `json:"source_key"`       // transport key (IP, etc.) — NOT an identity
	Target     string            `json:"target,omitempty"` // decoy path, token, endpoint
	Confidence Confidence        `json:"confidence"`       // 1.0 for deception hits
	Detail     string            `json:"detail,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"` // normalized labels (method, ua, path…)
	Provenance Provenance        `json:"provenance"`
}

// NewSecurityEvent builds a normalized event with a fresh ID and the current
// schema version. ObservedAt/RecordedAt default to now when the caller leaves
// provenance times zero.
func NewSecurityEvent(kind event.Kind, sev event.Severity, sourceKey string, prov Provenance) SecurityEvent {
	now := time.Now().UTC()
	if prov.ObservedAt.IsZero() {
		prov.ObservedAt = now
	}
	if prov.RecordedAt.IsZero() {
		prov.RecordedAt = now
	}
	return SecurityEvent{
		Schema:     SchemaVersion,
		ID:         NewID("se"),
		Kind:       kind,
		Severity:   sev,
		Stage:      StageUnknown,
		SourceKey:  sourceKey,
		Confidence: 0,
		Attributes: map[string]string{},
		Provenance: prov,
	}
}

// WithAttr sets a normalized attribute and returns the event (chainable).
func (e SecurityEvent) WithAttr(k, v string) SecurityEvent {
	if e.Attributes == nil {
		e.Attributes = map[string]string{}
	}
	e.Attributes[k] = v
	return e
}

// WithStage sets the tentative attack stage and returns the event.
func (e SecurityEvent) WithStage(s Stage) SecurityEvent {
	e.Stage = s
	return e
}

// FromDetection lifts a flat data-plane event.Event into a normalized
// SecurityEvent, carrying its confidence and labels and attaching provenance.
// The sensor name identifies which collector observed it.
func FromDetection(d event.Event, sensor, rawRef string) SecurityEvent {
	prov := Provenance{
		Sensor:     sensor,
		RawRef:     rawRef,
		ObservedAt: d.Time,
		RecordedAt: time.Now().UTC(),
	}
	if prov.ObservedAt.IsZero() {
		prov.ObservedAt = prov.RecordedAt
	}
	se := SecurityEvent{
		Schema:     SchemaVersion,
		ID:         NewID("se"),
		Kind:       d.Kind,
		Severity:   d.Severity,
		Stage:      stageForKind(d.Kind),
		SourceKey:  d.Source,
		Target:     d.Target,
		Confidence: Confidence(d.Confidence).Clamp(),
		Detail:     d.Detail,
		Attributes: map[string]string{},
		Provenance: prov,
	}
	for k, v := range d.Labels {
		se.Attributes[k] = v
	}
	return se
}

// stageForKind maps a detection kind to its most likely attack stage. This is a
// default hint only; campaign correlation may revise it with more context.
func stageForKind(k event.Kind) Stage {
	switch k {
	case event.KindDecoyHit, event.KindCanaryTriggered, event.KindCounterDeployed:
		return StageDeceptionTouch
	case event.KindAgenticTraffic, event.KindProbe, event.KindRateLimited:
		return StageRecon
	case event.KindEgressBlocked:
		return StageExfiltration
	default:
		return StageUnknown
	}
}
