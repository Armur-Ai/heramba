package watch

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Armur-Ai/heramba/internal/event"
)

func TestRecentFilteredByFloor(t *testing.T) {
	s := NewState(50)
	s.Add(event.New(event.KindProbe, event.SeverityLow, "a", "/x", "d"))
	s.Add(event.New(event.KindProbe, event.SeverityHigh, "b", "/y", "d"))
	s.Add(event.New(event.KindProbe, event.SeverityMedium, "c", "/z", "d"))

	all := s.RecentFiltered(10, event.SeverityInfo)
	if len(all) != 3 {
		t.Fatalf("no floor should return all 3, got %d", len(all))
	}
	high := s.RecentFiltered(10, event.SeverityHigh)
	if len(high) != 1 || high[0].Source != "b" {
		t.Fatalf("high floor should keep only the high event, got %+v", high)
	}
	med := s.RecentFiltered(10, event.SeverityMedium)
	if len(med) != 2 {
		t.Fatalf("medium floor should keep the high+medium events, got %d", len(med))
	}
}

func TestRecentFilteredKeepsNewestThatPass(t *testing.T) {
	s := NewState(50)
	// Many low events then one high: with n=1 and a high floor we must get the
	// high one, not "the newest overall, filtered away to nothing".
	for i := 0; i < 20; i++ {
		s.Add(event.New(event.KindProbe, event.SeverityLow, "lo", "/x", "d"))
	}
	s.Add(event.New(event.KindDecoyHit, event.SeverityHigh, "hi", "/.env", "d"))
	for i := 0; i < 5; i++ {
		s.Add(event.New(event.KindProbe, event.SeverityLow, "lo", "/x", "d"))
	}
	got := s.RecentFiltered(1, event.SeverityHigh)
	if len(got) != 1 || got[0].Source != "hi" {
		t.Fatalf("filter should find the high event among lows, got %+v", got)
	}
}

func TestFKeyCyclesSeverityFloor(t *testing.T) {
	m := NewModel(make(chan event.Event), "observe", 50)
	want := []event.Severity{
		event.SeverityLow, event.SeverityMedium, event.SeverityHigh,
		event.SeverityCritical, event.SeverityInfo, // wraps back to off
	}
	for _, w := range want {
		nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
		m = nm.(Model)
		if m.sevFloor != w {
			t.Fatalf("f should advance floor to %v, got %v", w, m.sevFloor)
		}
	}
}

func TestFeedShowsActiveFilter(t *testing.T) {
	m := NewModel(make(chan event.Event), "observe", 50)
	sized, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = sized.(Model)
	// Need at least one event so the feed (not the quiet hero) is rendered.
	nm0, _ := m.Update(eventMsg(event.Deception(event.KindDecoyHit, "1.1.1.1", "/.env", "d")))
	m = nm0.(Model)
	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})           // floor = low
	nm2, _ := nm.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}}) // floor = medium
	m = nm2.(Model)
	if !strings.Contains(m.View(), "MEDIUM+") {
		t.Error("feed header should show the active severity filter")
	}
}
