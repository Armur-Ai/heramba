package watch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Armur-Ai/heramba/internal/event"
)

func sampleState() *State {
	s := NewState(50)
	for _, ip := range []string{"1.1.1.1", "2.2.2.2"} {
		s.Add(event.Deception(event.KindDecoyHit, ip, "/.env", "d").WithLabel("campaign", "sweep"))
	}
	s.Add(event.Deception(event.KindCanaryTriggered, "1.1.1.1", "tok", "used"))
	return s
}

func TestSnapshotFields(t *testing.T) {
	s := sampleState()
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	now := start.Add(90 * time.Second)
	snap := s.Snapshot("protect", start, now)

	if snap.Mode != "protect" || snap.UptimeSecs != 90 {
		t.Errorf("mode/uptime wrong: %+v", snap)
	}
	if snap.Total != 3 || snap.Sources != 2 {
		t.Errorf("totals wrong: total=%d sources=%d", snap.Total, snap.Sources)
	}
	if snap.ByKind["decoy_hit"] != 2 || snap.ByKind["canary_triggered"] != 1 {
		t.Errorf("by-kind wrong: %v", snap.ByKind)
	}
	if len(snap.TopSources) == 0 {
		t.Error("expected top sources")
	}
}

func TestSnapshotJSONRoundTrips(t *testing.T) {
	snap := sampleState().Snapshot("observe", time.Now().Add(-time.Minute), time.Now())
	var back Snapshot
	if err := json.Unmarshal(snap.JSON(), &back); err != nil {
		t.Fatalf("snapshot JSON should round-trip: %v", err)
	}
	if back.Total != snap.Total {
		t.Errorf("round-trip total mismatch: %d vs %d", back.Total, snap.Total)
	}
}

func TestSnapshotMarkdownHasSections(t *testing.T) {
	md := sampleState().Snapshot("protect", time.Now().Add(-time.Minute), time.Now()).Markdown()
	for _, want := range []string{"# Heramba watch", "## By kind", "## Top sources", "decoy_hit"} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q", want)
		}
	}
}

func TestExportSnapshotWritesFileAndToast(t *testing.T) {
	dir := t.TempDir()
	m := NewModel(make(chan event.Event), "protect", 50)
	m.snapshotDir = dir
	m.now = time.Now()
	m.state = sampleState()

	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = nm.(Model)
	if !strings.HasPrefix(m.toast, "✓") {
		t.Fatalf("expected a success toast, got %q", m.toast)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 || !strings.HasSuffix(entries[0].Name(), ".md") {
		t.Fatalf("expected one .md snapshot file, got %v", entries)
	}
	body, _ := os.ReadFile(filepath.Join(dir, entries[0].Name()))
	if !strings.Contains(string(body), "# Heramba watch") {
		t.Error("snapshot file should contain the markdown report")
	}
}
