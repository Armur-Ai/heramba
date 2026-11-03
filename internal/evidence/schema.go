// Package evidence is Heramba's intelligence-plane foundation: a normalized,
// versioned evidence model that distinguishes what was *observed* from what is
// *inferred*.
//
// The data plane (internal/edge, internal/classify, …) emits flat
// detections (internal/event.Event). The intelligence plane needs more: a
// provenance chain, an explicit split between directly-witnessed facts and
// probabilistic hypotheses, confidence *and* uncertainty, and typed links
// between pieces of evidence that a campaign graph can walk.
//
// Three load-bearing rules, taken straight from the engineering mandate:
//
//   - An Observation is something we literally saw. A Fact is a normalized,
//     validated restatement of observations with no inference. A Hypothesis is
//     an inferred claim that carries both supporting and contradictory
//     evidence, a confidence, and an uncertainty — never presented as proven.
//   - Provenance is mandatory. Every record names the sensor that produced it
//     and an opaque reference to the raw artifact, so a human can audit back to
//     source.
//   - A source IP is an Observation, never an identity. Equating the two is a
//     Hypothesis (entity linking, CP2) with its own evidence and uncertainty.
package evidence

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync/atomic"
	"time"
)

// SchemaVersion is the evidence wire-format version. Bump it only on an
// incompatible change; the store records it on every persisted record so old
// logs can be read (or explicitly rejected) by a newer binary.
const SchemaVersion = 1

// ID is a time-sortable, globally-unique evidence identifier. The leading
// component is a big-endian unix-nanosecond timestamp so lexical order is
// chronological order; the tail is a per-process counter plus random bytes to
// stay unique within the same nanosecond and across processes.
type ID string

var idCounter atomic.Uint64

// NewID mints a time-sortable ID. It never returns an empty ID.
func NewID(prefix string) ID {
	now := uint64(time.Now().UTC().UnixNano())
	seq := idCounter.Add(1)
	var rnd [4]byte
	_, _ = rand.Read(rnd[:]) // best-effort entropy; collision guard, not a secret
	if prefix == "" {
		prefix = "ev"
	}
	return ID(fmt.Sprintf("%s_%016x%06x%s", prefix, now, seq&0xffffff, hex.EncodeToString(rnd[:])))
}

// Confidence is a calibrated belief in [0,1]. For a Fact it is 1.0 by
// construction (it restates an observation). For a Hypothesis it is the
// probability the claim is true given current evidence.
type Confidence float64

// Clamp returns the confidence bounded to [0,1].
func (c Confidence) Clamp() Confidence {
	switch {
	case c < 0:
		return 0
	case c > 1:
		return 1
	default:
		return c
	}
}

// Valid reports whether the confidence is a usable number in [0,1].
func (c Confidence) Valid() bool { return c >= 0 && c <= 1 }

// Provenance records where a piece of evidence came from, so any record can be
// audited back to its raw source. It is mandatory on every stored record.
type Provenance struct {
	// Sensor is the collector that produced the record: "edge", "classifier",
	// "decoy", "egress", an external adapter name, etc.
	Sensor string `json:"sensor"`
	// RawRef is an opaque reference to the raw artifact (request id, log line,
	// pcap offset). It lets a human find the ground truth; it is never parsed
	// by the engine and is treated as hostile text.
	RawRef string `json:"raw_ref,omitempty"`
	// ObservedAt is when the event happened in the world (best known).
	ObservedAt time.Time `json:"observed_at"`
	// RecordedAt is when Heramba ingested it.
	RecordedAt time.Time `json:"recorded_at"`
}

// Valid reports whether the provenance carries the minimum auditability: a
// sensor name and a non-zero observation time.
func (p Provenance) Valid() bool {
	return p.Sensor != "" && !p.ObservedAt.IsZero()
}
