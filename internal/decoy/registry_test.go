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

func TestDecoyPrefixZones(t *testing.T) {
	r := NewRegistry()
	// Any path under a decoy zone is a hit, even if not an exact decoy.
	for _, p := range []string{"/.git/HEAD", "/.git/refs/heads/main", "/actuator/health", "/.aws/config", "/wp-admin/options.php"} {
		if _, ok := r.IsDecoy(p); !ok {
			t.Errorf("%s should match a decoy zone", p)
		}
	}
	// Real routes that merely share a leading string must NOT match.
	for _, p := range []string{"/api/users", "/home", "/actuators", "/wp-admin-ui"} {
		if _, ok := r.IsDecoy(p); ok {
			t.Errorf("%s must not be a decoy", p)
		}
	}
	// Longest-prefix note wins when zones overlap.
	r.AddPrefix("/actuator/env/", "actuator env secret walk")
	e, ok := r.IsDecoy("/actuator/env/db.password")
	if !ok || e.Note != "actuator env secret walk" {
		t.Errorf("longest prefix should win, got %+v ok=%v", e, ok)
	}
}

func TestPrefixInspectEmitsEvent(t *testing.T) {
	r := NewRegistry()
	ev, ok := r.InspectPath("GET", "/.git/HEAD", "8.8.8.8")
	if !ok || ev.Source != "8.8.8.8" {
		t.Fatalf("decoy-zone path should emit an event: %+v ok=%v", ev, ok)
	}
}
