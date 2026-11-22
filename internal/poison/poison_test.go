package poison

import (
	"errors"
	"strings"
	"testing"
)

func newTestMarker(t *testing.T) *Marker {
	t.Helper()
	m, err := NewMarker([]byte("watermark-secret-key-123456"))
	if err != nil {
		t.Fatalf("NewMarker: %v", err)
	}
	return m
}

func TestWatermarkRoundTrip(t *testing.T) {
	m := newTestMarker(t)
	for _, v := range []string{"", "tok_abc123", "a.b.c", "value with spaces", "unicode:路径"} {
		w := m.Watermark(v)
		if !m.IsOurs(w) {
			t.Fatalf("watermarked %q not recognized: %q", v, w)
		}
		got, ok := m.Strip(w)
		if !ok || got != v {
			t.Fatalf("strip(%q) = %q,%v want %q,true", w, got, ok, v)
		}
	}
}

func TestIsOursRejectsForeign(t *testing.T) {
	m := newTestMarker(t)
	for _, v := range []string{"", "plain", "tok_abc", "has.hrmbwm1butbadtag", "x.hrmbwm1"} {
		if m.IsOurs(v) {
			t.Fatalf("non-watermarked value wrongly recognized: %q", v)
		}
	}
}

func TestWrongKeyRejects(t *testing.T) {
	a := newTestMarker(t)
	b, _ := NewMarker([]byte("a-totally-different-key!!!"))
	w := a.Watermark("tok_secret")
	if b.IsOurs(w) {
		t.Fatal("watermark from key A must not verify under key B")
	}
}

func TestTamperRejected(t *testing.T) {
	m := newTestMarker(t)
	w := m.Watermark("tok_original")
	// Alter the body but keep the (now-stale) tag.
	tampered := strings.Replace(w, "tok_original", "tok_attacker", 1)
	if m.IsOurs(tampered) {
		t.Fatal("altering the body must invalidate the watermark")
	}
}

func TestWatermarkIdempotent(t *testing.T) {
	m := newTestMarker(t)
	w1 := m.Watermark("tok_x")
	w2 := m.Watermark(w1) // already watermarked
	if w1 != w2 {
		t.Fatalf("double-watermark changed value: %q != %q", w1, w2)
	}
}

func TestEmptyKey(t *testing.T) {
	if _, err := NewMarker(nil); !errors.Is(err, ErrNoKey) {
		t.Fatalf("empty key should error, got %v", err)
	}
}
