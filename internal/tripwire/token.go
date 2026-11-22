// Package tripwire plants signed "trip" URLs and bait surfaces that only an
// automated crawler or a snooping attacker would ever request — a legitimate
// user never sees them. Every planted URL carries an opaque, self-describing
// token sealed with a keyed HMAC, so a hit is fully attributable (which bait,
// when planted) and forgery is detectable, with no database to consult.
//
// This is pure defense: the surfaces are served FROM our own honeypot. Nothing
// here contacts, scans, or sends traffic to the client — a tripwire only tells
// us, with zero false positives, that something touched bait no human would.
package tripwire

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"time"
)

// Kind classifies which bait surface a trip token was planted on. It travels
// inside the signed token, so a hit is attributable to its exact source.
type Kind string

const (
	KindRobots      Kind = "robots"       // a Disallow entry in robots.txt
	KindSitemap     Kind = "sitemap"      // a URL in a fake sitemap.xml
	KindHTMLComment Kind = "html_comment" // a link hidden in an HTML comment
	KindLinkHeader  Kind = "link_header"  // a Link: response header
	KindJSConfig    Kind = "js_config"    // a path inside a decoy JS config blob
	KindAPIDoc      Kind = "api_doc"      // a fake API-documentation endpoint
	KindManual      Kind = "manual"       // operator-planted, out-of-band
)

// Claims is what a verified trip token carries: the bait it was planted on, a
// free-form label for triage, and when it was planted.
type Claims struct {
	Kind    Kind
	Label   string
	Planted time.Time
}

// Age reports how long ago the token was planted, relative to now.
func (c Claims) Age() time.Duration { return time.Since(c.Planted) }

var (
	// ErrMalformed means the token is not three-plus-signature dot-separated
	// base64url parts, or a field failed to decode.
	ErrMalformed = errors.New("tripwire: malformed token")
	// ErrBadSignature means the HMAC did not authenticate — a forged or
	// tampered token, or one signed with a different key.
	ErrBadSignature = errors.New("tripwire: signature mismatch")
	// ErrExpired means the token verified but is older than the allowed window.
	ErrExpired = errors.New("tripwire: token expired")
	// ErrNoKey means a Signer was built without a signing key.
	ErrNoKey = errors.New("tripwire: empty signing key")
)

var b64 = base64.RawURLEncoding

// Signer mints and verifies trip tokens under a single secret key. It holds no
// per-token state: a token is self-describing and authenticated by the key
// alone, so Signers are cheap, safe for concurrent use, and need no storage.
type Signer struct {
	key []byte
}

// NewSigner returns a Signer over the given secret key. The key must be
// non-empty; keep it secret and stable so tokens minted across restarts stay
// verifiable.
func NewSigner(key []byte) (*Signer, error) {
	if len(key) == 0 {
		return nil, ErrNoKey
	}
	k := make([]byte, len(key))
	copy(k, key)
	return &Signer{key: k}, nil
}

// NewRandomSigner returns a Signer over a fresh 256-bit random key. Tokens it
// mints survive only for the lifetime of this Signer, which suits ephemeral
// per-process bait.
func NewRandomSigner() (*Signer, error) {
	var k [32]byte
	if _, err := rand.Read(k[:]); err != nil {
		return nil, err
	}
	return &Signer{key: k[:]}, nil
}

// Mint seals {kind, label, now} into an opaque, URL-safe token. The token
// contains no "/" or "." reserved for path/extension boundaries beyond its own
// three separators, so it drops cleanly into a path segment or a query value.
func (s *Signer) Mint(kind Kind, label string) string {
	return s.mintAt(kind, label, time.Now().UTC())
}

// mintAt is Mint with an explicit plant time, for deterministic tests.
func (s *Signer) mintAt(kind Kind, label string, t time.Time) string {
	payload := b64.EncodeToString([]byte(kind)) + "." +
		b64.EncodeToString([]byte(label)) + "." +
		strconv.FormatInt(t.UnixNano(), 10)
	return payload + "." + b64.EncodeToString(s.mac(payload))
}

// Verify authenticates a token and returns its claims. It checks the signature
// only; use VerifyFresh to also enforce an expiry window.
func (s *Signer) Verify(token string) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 4 {
		return Claims{}, ErrMalformed
	}
	payload := parts[0] + "." + parts[1] + "." + parts[2]
	sig, err := b64.DecodeString(parts[3])
	if err != nil {
		return Claims{}, ErrMalformed
	}
	if !hmac.Equal(sig, s.mac(payload)) {
		return Claims{}, ErrBadSignature
	}
	kb, err := b64.DecodeString(parts[0])
	if err != nil {
		return Claims{}, ErrMalformed
	}
	lb, err := b64.DecodeString(parts[1])
	if err != nil {
		return Claims{}, ErrMalformed
	}
	ns, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return Claims{}, ErrMalformed
	}
	return Claims{Kind: Kind(kb), Label: string(lb), Planted: time.Unix(0, ns).UTC()}, nil
}

// VerifyFresh authenticates a token and additionally requires it to have been
// planted no longer than maxAge ago. A maxAge <= 0 means no expiry (same as
// Verify). A token with a future plant time is treated as valid (clock skew).
func (s *Signer) VerifyFresh(token string, maxAge time.Duration) (Claims, error) {
	c, err := s.Verify(token)
	if err != nil {
		return Claims{}, err
	}
	if maxAge > 0 && time.Since(c.Planted) > maxAge {
		return Claims{}, ErrExpired
	}
	return c, nil
}

// mac computes the HMAC-SHA256 of the payload under the signer's key.
func (s *Signer) mac(payload string) []byte {
	m := hmac.New(sha256.New, s.key)
	m.Write([]byte(payload))
	return m.Sum(nil)
}
