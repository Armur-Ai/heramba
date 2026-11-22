// Package poison generates believable-but-false intelligence to feed a
// *confirmed* attacker, so whatever they exfiltrate from our honeypot is
// worthless and self-incriminating. Every poisoned value carries an embedded,
// keyed watermark: if it ever shows up again — a later request, a paste site,
// another victim's logs — we know it came from us, and we know it is fake.
//
// Hard safety rule: generated "secrets" are inert, non-functional examples.
// They never use a real vendor credential format that could be mistaken for a
// live key, and they always carry the watermark. Nothing here attacks anyone;
// it only produces fake data served FROM our honeypot.
package poison

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"errors"
	"strings"
)

// sentinel is a fixed, human-recognizable marker so even a casual scan of logs
// reveals a value as Heramba-planted. Authentication still rests on the HMAC
// tag that follows it (a sentinel alone could be copied).
const sentinel = "hrmbwm1"

// ErrNoKey is returned when a Marker is built without a key.
var ErrNoKey = errors.New("poison: empty watermark key")

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

// Marker embeds and verifies watermarks under a secret key. Safe for concurrent
// use; holds no per-value state.
type Marker struct {
	key []byte
}

// NewMarker returns a Marker over the given secret key (must be non-empty).
// Keep the key stable so watermarks planted earlier stay recognizable.
func NewMarker(key []byte) (*Marker, error) {
	if len(key) == 0 {
		return nil, ErrNoKey
	}
	k := make([]byte, len(key))
	copy(k, key)
	return &Marker{key: k}, nil
}

// NewRandomMarker returns a Marker over a fresh 256-bit random key.
func NewRandomMarker() (*Marker, error) {
	var k [32]byte
	if _, err := rand.Read(k[:]); err != nil {
		return nil, err
	}
	return &Marker{key: k[:]}, nil
}

// tag is the base32 of the first 8 bytes of HMAC-SHA256(key, value): a compact,
// unforgeable marker (13 chars, lowercased).
func (m *Marker) tag(value string) string {
	h := hmac.New(sha256.New, m.key)
	h.Write([]byte(value))
	sum := h.Sum(nil)
	return strings.ToLower(b32.EncodeToString(sum[:8]))
}

// Watermark returns value with our sentinel + authenticated tag appended, so the
// result reads as one opaque string but is recognizable and attributable to us.
// Watermarking an already-watermarked value is a no-op.
func (m *Marker) Watermark(value string) string {
	if m.IsOurs(value) {
		return value
	}
	return value + "." + sentinel + m.tag(value)
}

// IsOurs reports whether value carries a valid Heramba watermark under our key.
// It authenticates the tag (not just the sentinel), so a copied-but-altered
// value is rejected.
func (m *Marker) IsOurs(value string) bool {
	_, ok := m.Strip(value)
	return ok
}

// Strip recovers the original value from a watermarked one and reports whether a
// valid watermark was present. On failure it returns the input unchanged.
func (m *Marker) Strip(value string) (string, bool) {
	marker := "." + sentinel
	i := strings.LastIndex(value, marker)
	if i < 0 {
		return value, false
	}
	orig := value[:i]
	got := value[i+len(marker):]
	if hmac.Equal([]byte(got), []byte(m.tag(orig))) {
		return orig, true
	}
	return value, false
}
