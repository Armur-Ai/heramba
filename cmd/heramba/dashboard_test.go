package main

import (
	"testing"

	"github.com/Armur-Ai/heramba/internal/event"
)

func TestLiveSourceRecentNewestFirst(t *testing.T) {
	ls := newLiveSource(3, "observe")
	for _, s := range []string{"a", "b", "c", "d"} {
		ls.add(event.New(event.KindProbe, event.SeverityLow, s, "/x", "d"))
	}
	got := ls.Recent(10)
	if len(got) != 3 { // ring capped at 3
		t.Fatalf("ring should cap at 3, got %d", len(got))
	}
	if got[0].Source != "d" || got[2].Source != "b" {
		t.Errorf("expected newest-first d,c,b; got %s,%s,%s", got[0].Source, got[1].Source, got[2].Source)
	}
}

func TestLiveSourceMetricsAggregate(t *testing.T) {
	ls := newLiveSource(100, "protect")
	ls.add(event.Deception(event.KindDecoyHit, "1.1.1.1", "/.env", "d"))
	ls.add(event.Deception(event.KindDecoyHit, "1.1.1.1", "/.env", "d"))
	ls.add(event.New(event.KindProbe, event.SeverityLow, "2.2.2.2", "/x", "d"))

	m := ls.Metrics()
	if m.Mode != "protect" {
		t.Errorf("mode = %q, want protect", m.Mode)
	}
	if m.Total != 3 {
		t.Errorf("total = %d, want 3", m.Total)
	}
	if m.ByKind["decoy_hit"] != 2 {
		t.Errorf("decoy_hit count = %d, want 2", m.ByKind["decoy_hit"])
	}
	if m.Sources != 2 {
		t.Errorf("distinct sources = %d, want 2", m.Sources)
	}
}

func TestLiveSourceMetricsSnapshotIsCopied(t *testing.T) {
	// Mutating a returned map must not corrupt internal counters.
	ls := newLiveSource(10, "observe")
	ls.add(event.Deception(event.KindDecoyHit, "1.1.1.1", "/.env", "d"))
	m := ls.Metrics()
	m.ByKind["decoy_hit"] = 999
	if ls.Metrics().ByKind["decoy_hit"] != 1 {
		t.Error("Metrics() must return a copy, not the live map")
	}
}

func TestReadOnlyControllerRejects(t *testing.T) {
	var c readOnlyController
	if c.SetMode("protect") == nil || c.Clear() == nil || c.SetPaused(true) == nil || c.ApplyDecoyPreset("node") == nil {
		t.Error("read-only controller must reject every control action")
	}
}
