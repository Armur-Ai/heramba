package watch

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Armur-Ai/heramba/internal/event"
)

func TestQuietHeroBeforeTraffic(t *testing.T) {
	m := NewModel(make(chan event.Event), "observe", 50)
	sized, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = sized.(Model)
	out := m.View()
	if !strings.Contains(out, "ALL QUIET") {
		t.Error("an empty dashboard should show the ALL QUIET hero")
	}
	// Once an event lands, the hero gives way to the live body.
	nm, _ := m.Update(eventMsg(event.Deception(event.KindDecoyHit, "1.1.1.1", "/.env", "d")))
	if strings.Contains(nm.(Model).View(), "ALL QUIET") {
		t.Error("hero should disappear after the first event")
	}
}

func TestFooterFitsNarrowWidth(t *testing.T) {
	m := NewModel(make(chan event.Event), "observe", 50)
	for _, w := range []int{200, 80, 40, 20, 10} {
		sized, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: 30})
		m = sized.(Model)
		f := m.footer()
		if lipgloss.Width(f) > w && w >= 20 {
			t.Errorf("footer width %d exceeds terminal width %d: %q", lipgloss.Width(f), w, f)
		}
	}
}
