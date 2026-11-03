// Package decoy implements Heramba's deception surface: canary tokens and decoy
// endpoints. Its defining property is zero-false-positive detection — a
// legitimate user never mints, holds, or uses these, so any use is a
// high-confidence signal of an attacker (often an autonomous agent enumerating
// surface).
package decoy

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// canaryPrefix tags Heramba canary tokens so they are recognizable in logs and
// so a used token is unambiguously ours.
const canaryPrefix = "hrmb_"

// Canary is a planted token. Using it anywhere (as a credential, API key, path
// segment, etc.) is, by construction, illegitimate.
type Canary struct {
	Token   string    `json:"token"`
	Label   string    `json:"label"` // where it was planted, for triage
	Planted time.Time `json:"planted"`
}

// CanaryStore mints and tracks canary tokens. Safe for concurrent use.
type CanaryStore struct {
	mu      sync.RWMutex
	byToken map[string]Canary
}

// NewCanaryStore returns an empty store.
func NewCanaryStore() *CanaryStore {
	return &CanaryStore{byToken: map[string]Canary{}}
}

// Mint creates a new unguessable canary token labeled with where it will be
// planted. The token is cryptographically random (128 bits).
func (s *CanaryStore) Mint(label string) (Canary, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return Canary{}, err
	}
	c := Canary{Token: canaryPrefix + hex.EncodeToString(b[:]), Label: label, Planted: time.Now().UTC()}
	s.mu.Lock()
	s.byToken[c.Token] = c
	s.mu.Unlock()
	return c, nil
}

// Check reports whether a presented value is a known canary token (i.e. the
// token has been used/observed). The lookup is exact-match on the full token.
func (s *CanaryStore) Check(token string) (Canary, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.byToken[token]
	return c, ok
}

// Looks reports whether a value even resembles a Heramba canary (has the
// prefix), useful for cheap pre-filtering before an exact Check.
func Looks(value string) bool {
	return len(value) == len(canaryPrefix)+32 && value[:len(canaryPrefix)] == canaryPrefix
}

// All returns a snapshot of planted canaries.
func (s *CanaryStore) All() []Canary {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Canary, 0, len(s.byToken))
	for _, c := range s.byToken {
		out = append(out, c)
	}
	return out
}
