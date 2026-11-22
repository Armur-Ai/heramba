package watch

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Armur-Ai/heramba/internal/event"
)

func TestHelpToggle(t *testing.T) {
	m := NewModel(make(chan event.Event), "observe", 50)
	sized, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = sized.(Model)

	q := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}}
	nm, _ := m.Update(q)
	m = nm.(Model)
	if !m.showHelp {
		t.Fatal("'?' should open help")
	}
	if !strings.Contains(m.View(), "KEYBOARD") {
		t.Error("help body should render the keyboard reference")
	}

	// '?' again toggles off.
	nm, _ = m.Update(q)
	if nm.(Model).showHelp {
		t.Error("'?' again should close help")
	}
}

func TestEscClosesHelpThenQuits(t *testing.T) {
	m := NewModel(make(chan event.Event), "observe", 50)
	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = nm.(Model)

	// First esc closes help, does NOT quit.
	nm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = nm.(Model)
	if m.showHelp {
		t.Error("esc should close help")
	}
	if cmd != nil {
		t.Error("esc that only closes help must not quit")
	}
	// Second esc (no help open) quits.
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Error("esc with no overlay should quit")
	}
}
