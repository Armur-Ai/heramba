package decoy

import (
	"testing"

	"github.com/Armur-Ai/heramba/internal/event"
)

func TestDefaultDecoysMatch(t *testing.T) {
	r := NewRegistry()
	if _, ok := r.IsDecoy("/.env"); !ok {
		t.Fatal("/.env should be a default decoy")
	}
	if _, ok := r.IsDecoy("/api/users"); ok {
		t.Fatal("a real path must not be a decoy")
	}
}

func TestExtraAndAdd(t *testing.T) {
	r := NewRegistry("/secret-backup")
	if _, ok := r.IsDecoy("/secret-backup"); !ok {
		t.Fatal("extra decoy not registered")
	}
	r.Add("/db.sql", "db dump lure")
	if e, ok := r.IsDecoy("/db.sql"); !ok || e.Note != "db dump lure" {
		t.Fatalf("Add failed: %+v %v", e, ok)
	}
}

func TestInspectPathEmitsZeroFPEvent(t *testing.T) {
	r := NewRegistry()
	ev, ok := r.InspectPath("GET", "/.env", "7.7.7.7")
	if !ok {
		t.Fatal("decoy path should produce an event")
	}
	if ev.Kind != event.KindDecoyHit || ev.Confidence != 1.0 || ev.Source != "7.7.7.7" {
		t.Fatalf("bad event: %+v", ev)
	}
	if _, ok := r.InspectPath("GET", "/home", "7.7.7.7"); ok {
		t.Fatal("non-decoy must not produce an event")
	}
}
