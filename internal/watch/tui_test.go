package watch

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Armur-Ai/heramba/internal/event"
)

func TestModelFoldsEventAndRearms(t *testing.T) {
	ch := make(chan event.Event, 1)
	m := NewModel(ch, "observe", 50)
	updated, cmd := m.Update(eventMsg(event.Deception(event.KindDecoyHit, "1.2.3.4", "/.env", "d")))
	nm := updated.(Model)
	if nm.state.Total != 1 || nm.state.DecoyHits() != 1 {
		t.Fatalf("event not folded: total=%d", nm.state.Total)
	}
	if cmd == nil {
		t.Fatal("expected a re-arm command after an event")
	}
}

func TestModelPauseFreezesIngest(t *testing.T) {
	m := NewModel(make(chan event.Event), "observe", 50)
	paused, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	m = paused.(Model)
	if !m.paused {
		t.Fatal("space should pause")
	}
	after, _ := m.Update(eventMsg(event.Deception(event.KindDecoyHit, "x", "/x", "d")))
	if after.(Model).state.Total != 0 {
		t.Fatal("paused model must not ingest events")
	}
}

func TestModelQuitKeys(t *testing.T) {
	m := NewModel(make(chan event.Event), "observe", 50)
	for _, k := range []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune{'q'}},
		{Type: tea.KeyCtrlC},
		{Type: tea.KeyEsc},
	} {
		_, cmd := m.Update(k)
		if cmd == nil {
			t.Fatalf("key %v should return a quit command", k)
		}
	}
}

func TestModelClearResetsState(t *testing.T) {
	m := NewModel(make(chan event.Event), "observe", 50)
	nm, _ := m.Update(eventMsg(event.Deception(event.KindDecoyHit, "x", "/x", "d")))
	m = nm.(Model)
	cleared, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	if cleared.(Model).state.Total != 0 {
		t.Fatal("c should clear state")
	}
}

func TestViewRendersWithoutPanic(t *testing.T) {
	ch := make(chan event.Event)
	m := NewModel(ch, "protect", 50)
	sized, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = sized.(Model)
	// Fold a swarm so every panel has content.
	for _, s := range []string{"1.1.1.1", "2.2.2.2"} {
		e := event.Deception(event.KindDecoyHit, s, "/.env", "d").WithLabel("campaign", "sweep")
		nm, _ := m.Update(eventMsg(e))
		m = nm.(Model)
	}
	out := m.View()
	for _, want := range []string{"HERAMBA", "PROTECT", "LIVE FEED", "TOP SOURCES", "SWARM"} {
		if !strings.Contains(out, want) {
			t.Errorf("view missing %q", want)
		}
	}
}
