package watch

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Armur-Ai/heramba/internal/event"
)

func seedFeed(m Model, n int) Model {
	for i := 0; i < n; i++ {
		e := event.Deception(event.KindDecoyHit, "10.0.0.1", "/.env", "decoy touched").
			WithLabel("campaign", "sweep").WithLabel("ua", "curl/8.0")
		nm, _ := m.Update(eventMsg(e))
		m = nm.(Model)
	}
	return m
}

func TestCursorMovesAndClamps(t *testing.T) {
	m := NewModel(make(chan event.Event), "observe", 50)
	m = seedFeed(m, 3)

	down := tea.KeyMsg{Type: tea.KeyDown}
	for i := 0; i < 10; i++ { // over-scroll
		nm, _ := m.Update(down)
		m = nm.(Model)
	}
	if m.cursor != 2 {
		t.Fatalf("cursor should clamp at len-1=2, got %d", m.cursor)
	}
	up := tea.KeyMsg{Type: tea.KeyUp}
	for i := 0; i < 10; i++ {
		nm, _ := m.Update(up)
		m = nm.(Model)
	}
	if m.cursor != 0 {
		t.Fatalf("cursor should clamp at 0, got %d", m.cursor)
	}
}

func TestEnterOpensInspectorWithDetail(t *testing.T) {
	m := NewModel(make(chan event.Event), "protect", 50)
	sized, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = seedFeed(sized.(Model), 2)

	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = nm.(Model)
	if !m.inspecting {
		t.Fatal("enter should open the inspector")
	}
	out := m.View()
	for _, want := range []string{"EVENT INSPECTOR", "LABELS", "campaign", "sweep", "confidence"} {
		if !strings.Contains(out, want) {
			t.Errorf("inspector missing %q", want)
		}
	}

	// esc closes the inspector (does not quit).
	nm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if nm.(Model).inspecting {
		t.Error("esc should close the inspector")
	}
	if cmd != nil {
		t.Error("esc closing the inspector must not quit")
	}
}

func TestSelectedEventFollowsCursor(t *testing.T) {
	m := NewModel(make(chan event.Event), "observe", 50)
	// Three distinguishable events.
	for _, src := range []string{"a", "b", "c"} {
		e := event.New(event.KindProbe, event.SeverityMedium, src, "/x", "d")
		nm, _ := m.Update(eventMsg(e))
		m = nm.(Model)
	}
	// cursor 0 = newest = "c".
	if e, ok := m.selectedEvent(); !ok || e.Source != "c" {
		t.Fatalf("cursor 0 should select newest 'c', got %+v", e)
	}
	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = nm.(Model)
	if e, _ := m.selectedEvent(); e.Source != "b" {
		t.Errorf("cursor 1 should select 'b', got %q", e.Source)
	}
}
