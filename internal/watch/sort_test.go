package watch

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Armur-Ai/heramba/internal/event"
)

func TestSourcesByHits(t *testing.T) {
	s := NewState(100)
	add := func(src string, n int) {
		for i := 0; i < n; i++ {
			s.Add(event.New(event.KindProbe, event.SeverityLow, src, "/x", "d"))
		}
	}
	add("busy", 5)
	add("quiet", 1)
	add("mid", 3)
	got := s.sourcesBy(sortHits, 10)
	if got[0].Addr != "busy" || got[1].Addr != "mid" || got[2].Addr != "quiet" {
		t.Errorf("sortHits order wrong: %s,%s,%s", got[0].Addr, got[1].Addr, got[2].Addr)
	}
}

func TestSourcesByDecoys(t *testing.T) {
	s := NewState(100)
	s.Add(event.New(event.KindProbe, event.SeverityLow, "noisy", "/x", "d"))
	s.Add(event.New(event.KindProbe, event.SeverityLow, "noisy", "/x", "d"))
	s.Add(event.Deception(event.KindDecoyHit, "trapped", "/.env", "d"))
	got := s.sourcesBy(sortDecoys, 10)
	if got[0].Addr != "trapped" {
		t.Errorf("sortDecoys should rank the deception-hitting source first, got %s", got[0].Addr)
	}
}

func TestSourcesByRecent(t *testing.T) {
	s := NewState(100)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	e1 := event.New(event.KindProbe, event.SeverityLow, "old", "/x", "d")
	e1.Time = base
	s.Add(e1)
	e2 := event.New(event.KindProbe, event.SeverityLow, "new", "/x", "d")
	e2.Time = base.Add(time.Hour)
	s.Add(e2)
	got := s.sourcesBy(sortRecent, 10)
	if got[0].Addr != "new" {
		t.Errorf("sortRecent should rank the most recent source first, got %s", got[0].Addr)
	}
}

func TestOKeyCyclesSortAndShows(t *testing.T) {
	m := NewModel(make(chan event.Event), "observe", 50)
	sized, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = sized.(Model)
	m.view = viewSources
	if m.srcSort != sortUrgency {
		t.Fatal("default sort should be urgency")
	}
	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	m = nm.(Model)
	if m.srcSort != sortHits {
		t.Fatalf("o should advance to hits, got %v", m.srcSort)
	}
	// Add a source so the sources view has content, then confirm the label shows.
	nm2, _ := m.Update(eventMsg(event.New(event.KindProbe, event.SeverityLow, "x", "/y", "d")))
	m = nm2.(Model)
	if !strings.Contains(m.View(), "sort: ") || !strings.Contains(m.View(), "hits") {
		t.Error("sources view should display the active sort label")
	}
}
