package tripwire

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func newTestSigner(t *testing.T) *Signer {
	t.Helper()
	s, err := NewSigner([]byte("test-signing-key-0123456789"))
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	return s
}

func TestMintVerifyRoundTrip(t *testing.T) {
	s := newTestSigner(t)
	cases := []struct {
		kind  Kind
		label string
	}{
		{KindRobots, "/backup/"},
		{KindSitemap, "/admin/export.csv"},
		{KindHTMLComment, "db creds at /__trip"},
		{KindManual, ""},
		{KindAPIDoc, "unicode: café/路径/🔑"},
	}
	for _, tc := range cases {
		tok := s.Mint(tc.kind, tc.label)
		c, err := s.Verify(tok)
		if err != nil {
			t.Fatalf("Verify(%q) error: %v", tok, err)
		}
		if c.Kind != tc.kind || c.Label != tc.label {
			t.Fatalf("round-trip mismatch: got {%q,%q} want {%q,%q}", c.Kind, c.Label, tc.kind, tc.label)
		}
		if time.Since(c.Planted) > time.Minute || time.Since(c.Planted) < -time.Minute {
			t.Fatalf("planted time implausible: %v", c.Planted)
		}
	}
}

func TestTokenIsURLSafe(t *testing.T) {
	s := newTestSigner(t)
	tok := s.Mint(KindSitemap, "/a/b?c=d&e=f/g")
	// A trip token must drop into a path segment or query value untouched: no
	// slash (path boundary) and no raw characters that need escaping.
	if strings.ContainsAny(tok, "/ ?&#%") {
		t.Fatalf("token not URL-safe: %q", tok)
	}
}

func TestTamperDetected(t *testing.T) {
	s := newTestSigner(t)
	tok := s.Mint(KindRobots, "/secret/")
	parts := strings.Split(tok, ".")
	if len(parts) != 4 {
		t.Fatalf("expected 4 parts, got %d", len(parts))
	}

	// Flip the label payload to a different, validly-encoded value.
	forged := b64.EncodeToString([]byte("/public/"))
	tampered := parts[0] + "." + forged + "." + parts[2] + "." + parts[3]
	if _, err := s.Verify(tampered); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("tampered label should fail signature, got %v", err)
	}

	// Mangle the signature itself.
	badSig := parts[0] + "." + parts[1] + "." + parts[2] + "." + b64.EncodeToString([]byte("not-the-mac"))
	if _, err := s.Verify(badSig); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("mangled signature should fail, got %v", err)
	}
}

func TestWrongKeyRejected(t *testing.T) {
	a := newTestSigner(t)
	b, err := NewSigner([]byte("a-completely-different-key!!"))
	if err != nil {
		t.Fatal(err)
	}
	tok := a.Mint(KindJSConfig, "/config.js")
	if _, err := b.Verify(tok); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("token from key A must not verify under key B, got %v", err)
	}
}

func TestMalformedRejected(t *testing.T) {
	s := newTestSigner(t)
	for _, bad := range []string{
		"", "nodots", "a.b.c", "a.b.c.d.e",
		"!!.!!.123.!!",            // bad base64
		"YWJj.YWJj.notanumber.Zz", // bad timestamp
	} {
		if _, err := s.Verify(bad); err == nil {
			t.Fatalf("expected error for %q", bad)
		}
	}
}

func TestVerifyFreshExpiryWindow(t *testing.T) {
	s := newTestSigner(t)
	old := s.mintAt(KindRobots, "/old/", time.Now().Add(-2*time.Hour))

	if _, err := s.VerifyFresh(old, time.Hour); !errors.Is(err, ErrExpired) {
		t.Fatalf("2h-old token should be expired under a 1h window, got %v", err)
	}
	if _, err := s.VerifyFresh(old, 3*time.Hour); err != nil {
		t.Fatalf("2h-old token should pass a 3h window, got %v", err)
	}
	if _, err := s.VerifyFresh(old, 0); err != nil {
		t.Fatalf("maxAge 0 means no expiry, got %v", err)
	}
	// A still-expired token must not leak claims.
	if c, err := s.VerifyFresh(old, time.Hour); err == nil || c.Kind != "" {
		t.Fatalf("expired verify must return zero claims, got %+v err=%v", c, err)
	}
}

func TestEmptyKeyRejected(t *testing.T) {
	if _, err := NewSigner(nil); !errors.Is(err, ErrNoKey) {
		t.Fatalf("empty key should error, got %v", err)
	}
}

func TestKeyIsCopied(t *testing.T) {
	key := []byte("mutable-key-material-xxxxxxxx")
	s, err := NewSigner(key)
	if err != nil {
		t.Fatal(err)
	}
	tok := s.Mint(KindRobots, "/x/")
	// Mutating the caller's slice must not change the signer's key.
	for i := range key {
		key[i] = 0
	}
	if _, err := s.Verify(tok); err != nil {
		t.Fatalf("signer key must be insulated from caller mutation, got %v", err)
	}
}
