package alert

import (
	"strings"
	"testing"

	"github.com/Armur-Ai/heramba/internal/event"
)

func TestTruncateRunes(t *testing.T) {
	if got := truncateRunes("short", 100); got != "short" {
		t.Fatalf("under-cap string should be unchanged, got %q", got)
	}
	if got := truncateRunes("", 10); got != "" {
		t.Fatalf("empty should stay empty, got %q", got)
	}
	long := strings.Repeat("a", 5000)
	got := truncateRunes(long, 100)
	if len([]rune(got)) > 100 {
		t.Fatalf("truncated length %d exceeds cap 100", len([]rune(got)))
	}
	if !strings.HasSuffix(got, "[truncated]") {
		t.Fatalf("expected truncation marker, got %q", got[len(got)-20:])
	}
	// Rune-safe: never splits a multibyte rune.
	multi := strings.Repeat("é", 200)
	tr := truncateRunes(multi, 50)
	if !strings.HasPrefix(tr, "é") || strings.Contains(tr, "�") {
		t.Fatalf("truncation split a multibyte rune: %q", tr)
	}
}

func TestRedactEventTruncatesAndMasks(t *testing.T) {
	e := event.New(event.KindProbe, event.SeverityHigh, "1.2.3.4", strings.Repeat("t", 9000), strings.Repeat("d", 9000))
	e = e.WithLabel("campaign", "spring").
		WithLabel("api_token", "super-secret-value").
		WithLabel("notes", strings.Repeat("x", 9000))

	r := RedactEvent(e, 4096)
	if len([]rune(r.Detail)) > 4096 {
		t.Fatalf("Detail not truncated: %d", len([]rune(r.Detail)))
	}
	if len([]rune(r.Target)) > 4096 {
		t.Fatalf("Target not truncated: %d", len([]rune(r.Target)))
	}
	if r.Labels["api_token"] != "[redacted]" {
		t.Fatalf("secret label not masked: %q", r.Labels["api_token"])
	}
	if r.Labels["campaign"] != "spring" {
		t.Fatalf("non-secret label changed: %q", r.Labels["campaign"])
	}
	if len([]rune(r.Labels["notes"])) > 4096 {
		t.Fatalf("oversized label value not truncated: %d", len([]rune(r.Labels["notes"])))
	}
	// Original event must be untouched (no in-place mutation of caller's map).
	if e.Labels["api_token"] != "super-secret-value" || len([]rune(e.Detail)) != 9000 {
		t.Fatal("RedactEvent mutated the original event")
	}
}

func TestRedactingSinkForwardsRedacted(t *testing.T) {
	var seen event.Event
	capture := sinkFunc(func(e event.Event) error { seen = e; return nil })
	s := NewRedacting(capture, 16)
	e := event.New(event.KindProbe, event.SeverityHigh, "x", "y", strings.Repeat("z", 100)).
		WithLabel("secret_key", "abc")
	if err := s.Emit(e); err != nil {
		t.Fatalf("emit: %v", err)
	}
	if len([]rune(seen.Detail)) > 16 {
		t.Fatalf("sink received un-truncated detail: %d", len([]rune(seen.Detail)))
	}
	if seen.Labels["secret_key"] != "[redacted]" {
		t.Fatalf("sink received unmasked secret: %q", seen.Labels["secret_key"])
	}
}

func TestSensitiveLabel(t *testing.T) {
	for _, k := range []string{"api_token", "Authorization", "DB_PASSWORD", "client_secret", "private_key"} {
		if !sensitiveLabel(k) {
			t.Errorf("%q should be treated as sensitive", k)
		}
	}
	for _, k := range []string{"campaign", "source", "kind"} {
		if sensitiveLabel(k) {
			t.Errorf("%q should not be treated as sensitive", k)
		}
	}
}
