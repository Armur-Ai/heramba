package watch

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Armur-Ai/heramba/internal/event"
)

// Navigating down must pin the selected event even as new events stream in.
func TestScrollLockPinsSelection(t *testing.T) {
	m := NewModel(make(chan event.Event), "observe", 100)
	sized, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = sized.(Model)
	for _, s := range []string{"a", "b", "c"} {
		nm, _ := m.Update(eventMsg(event.New(event.KindProbe, event.SeverityMedium, s, "/x", "d")))
		m = nm.(Model)
	}
	// Move down twice → select "a" (oldest of the three; newest-first b/c order: c,b,a).
	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = nm.(Model)
	nm, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = nm.(Model)
	sel, _ := m.selectedEvent()
	if sel.Source != "a" {
		t.Fatalf("cursor 2 should select 'a', got %q", sel.Source)
	}
	// Now five more events arrive — the selection must still be "a".
	for i := 0; i < 5; i++ {
		nm, _ := m.Update(eventMsg(event.New(event.KindProbe, event.SeverityHigh, "new", "/y", "d")))
		m = nm.(Model)
	}
	if sel2, _ := m.selectedEvent(); sel2.Source != "a" {
		t.Errorf("selection must stay pinned under scroll-lock, got %q", sel2.Source)
	}
	// The feed shows the scroll-lock indicator with the new-event count.
	if !strings.Contains(m.View(), "SCROLL-LOCK") {
		t.Error("feed should show the scroll-lock indicator while navigating")
	}
}

func TestScrollLockReleasesAtTop(t *testing.T) {
	m := NewModel(make(chan event.Event), "observe", 100)
	for _, s := range []string{"a", "b"} {
		nm, _ := m.Update(eventMsg(event.New(event.KindProbe, event.SeverityMedium, s, "/x", "d")))
		m = nm.(Model)
	}
	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = nm.(Model)
	if len(m.frozen) == 0 {
		t.Fatal("navigating down should freeze the feed")
	}
	nm, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp}) // back to top
	m = nm.(Model)
	if m.frozen != nil {
		t.Error("returning to the top should release the scroll-lock")
	}
}

func TestClearReleasesScrollLock(t *testing.T) {
	m := NewModel(make(chan event.Event), "observe", 100)
	nm, _ := m.Update(eventMsg(event.New(event.KindProbe, event.SeverityMedium, "a", "/x", "d")))
	m = nm.(Model)
	nm, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = nm.(Model)
	nm, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m = nm.(Model)
	if m.frozen != nil || m.cursor != 0 {
		t.Error("clear should reset scroll-lock and cursor")
	}
}
