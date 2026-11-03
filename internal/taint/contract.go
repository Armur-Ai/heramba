package taint

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

// This file defines the typed observation/action contract. External input is
// parsed into a struct with a CLOSED set of enum fields and free text that is
// quarantined as attacker-controlled. The decision path reads only the typed
// fields; the free text is never consulted, so no instruction an attacker writes
// into it can influence an action.

// ObservationKind is a closed enum. An unknown kind is rejected (schema
// confusion defense), never coerced.
type ObservationKind string

const (
	KindDecoyHit      ObservationKind = "decoy_hit"
	KindAgenticScore  ObservationKind = "agentic_score"
	KindBlockedEgress ObservationKind = "blocked_egress"
	KindRateExceeded  ObservationKind = "rate_exceeded"
)

var validKinds = map[ObservationKind]bool{
	KindDecoyHit: true, KindAgenticScore: true, KindBlockedEgress: true, KindRateExceeded: true,
}

// rawObservation is the exact wire shape; strict decoding rejects anything else.
type rawObservation struct {
	Kind      string  `json:"kind"`
	SourceKey string  `json:"source_key"`
	Severity  float64 `json:"severity"`
	Nonce     string  `json:"nonce"`
	Note      string  `json:"note"` // free text — attacker-controllable, quarantined
}

// Observation is the typed, trust-labeled contract the decision path consumes.
// The decision-relevant fields are trusted scalars validated against closed
// sets; Note is retained only as a tainted value for audit/logging.
type Observation struct {
	Kind      ObservationKind  // validated enum (trusted)
	SourceKey Tainted[string]  // a transport key: never an identity, kept tainted
	Severity  Tainted[float64] // numeric, validated into [0,1] -> trusted
	Nonce     string           // for replay defense
	Note      Tainted[string]  // quarantined free text, never read by a decision
	Prov      Provenance
}

// Decode errors.
var (
	ErrSchemaConfusion = errors.New("taint: unexpected field or type in observation (schema confusion)")
	ErrUnknownKind     = errors.New("taint: unknown observation kind")
	ErrSeverityRange   = errors.New("taint: severity out of range")
)

// DecodeObservation parses untrusted JSON into the typed contract. It rejects
// unknown fields and type mismatches (schema confusion), rejects unknown kinds,
// range-checks severity, and quarantines free text as attacker-controlled. A
// malformed input returns an error and never a partially-trusted object.
func DecodeObservation(raw []byte, prov Provenance) (Observation, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields() // schema-confusion defense: extra fields are rejected
	var r rawObservation
	if err := dec.Decode(&r); err != nil {
		return Observation{}, fmt.Errorf("%w: %v", ErrSchemaConfusion, err)
	}
	// Reject trailing data (smuggling a second document).
	if dec.More() {
		return Observation{}, fmt.Errorf("%w: trailing data", ErrSchemaConfusion)
	}
	kind := ObservationKind(r.Kind)
	if !validKinds[kind] {
		return Observation{}, fmt.Errorf("%w: %q", ErrUnknownKind, r.Kind)
	}
	if r.Severity < 0 || r.Severity > 1 {
		return Observation{}, fmt.Errorf("%w: %v", ErrSeverityRange, r.Severity)
	}
	return Observation{
		Kind:      kind,
		SourceKey: AttackerValue(r.SourceKey, prov), // a source key is attacker-chosen
		Severity:  TrustedValue(r.Severity, prov),   // validated numeric
		Nonce:     r.Nonce,
		Note:      AttackerValue(r.Note, prov),
		Prov:      prov,
	}, nil
}

// ContainmentDecision is the typed output of the authorizer. RequiresApproval
// gates high-impact actions behind a human.
type ContainmentDecision struct {
	Act              bool
	Tool             string
	RequiresApproval bool
	Reason           string
}

// AuthorizeContainment decides whether to contain, reading ONLY typed, trusted
// fields and the tool allowlist. It never consults Note or SourceKey content, so
// attacker text cannot steer it. High severity requires human approval
// (fail-safe), and the chosen tool must be on the allowlist (fail-closed).
func AuthorizeContainment(obs Observation, allow ToolAllowlist, tool string) (ContainmentDecision, error) {
	if err := allow.Authorize(tool); err != nil {
		return ContainmentDecision{Reason: "tool not allowed"}, err
	}
	sev, ok := obs.Severity.Trusted()
	if !ok {
		return ContainmentDecision{Reason: "severity not trusted"}, ErrUntrustedDecision
	}
	switch obs.Kind {
	case KindDecoyHit:
		// Deception contact is zero-false-positive: a bounded action is justified.
		return ContainmentDecision{Act: true, Tool: tool, RequiresApproval: sev >= 0.8, Reason: "decoy hit"}, nil
	case KindRateExceeded, KindBlockedEgress, KindAgenticScore:
		return ContainmentDecision{Act: sev >= 0.5, Tool: tool, RequiresApproval: sev >= 0.8, Reason: string(obs.Kind)}, nil
	default:
		return ContainmentDecision{Reason: "no rule"}, nil
	}
}
