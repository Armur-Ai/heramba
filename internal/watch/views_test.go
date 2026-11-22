package watch

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Armur-Ai/heramba/internal/event"
)

func feedSwarm(m Model) Model {
	for _, ip := range []string{"1.1.1.1", "2.2.2.2", "3.3.3.3"} {
		e := event.Deception(event.KindDecoyHit, ip, "/.env", "d").WithLabel("campaign", "sweep")
		nm, _ := m.Update(eventMsg(e))
		m = nm.(Model)
	}
	e := event.Deception(event.KindCanaryTriggered, "1.1.1.1", "tok", "used")
	nm, _ := m.Update(eventMsg(e))
	return nm.(Model)
}

func TestTabCyclesViews(t *testing.T) {
	m := NewModel(make(chan event.Event), "observe", 50)
	if m.view != viewOverview {
		t.Fatal("default view should be overview")
	}
	for want := viewSources; want < numViews; want++ {
		nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
		m = nm.(Model)
		if m.view != want {
			t.Fatalf("tab should advance to %v, got %v", want, m.view)
		}
	}
	// Wraps back to overview.
	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if nm.(Model).view != viewOverview {
		t.Fatal("tab should wrap to overview")
	}
}

func TestNumberKeysJumpToView(t *testing.T) {
	m := NewModel(make(chan event.Event), "observe", 50)
	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	if nm.(Model).view != viewCampaigns {
		t.Fatalf("key 3 should select campaigns, got %v", nm.(Model).view)
	}
}

func TestEachViewRendersWithoutPanic(t *testing.T) {
	m := NewModel(make(chan event.Event), "protect", 50)
	sized, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = feedSwarm(sized.(Model))
	for v := viewOverview; v < numViews; v++ {
		m.view = v
		out := m.View()
		if !strings.Contains(out, v.title()) {
			t.Errorf("view %v output should contain its title", v)
		}
		if strings.TrimSpace(out) == "" {
			t.Errorf("view %v rendered empty", v)
		}
	}
}

func TestSourcesViewListsSources(t *testing.T) {
	m := NewModel(make(chan event.Event), "observe", 50)
	sized, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = feedSwarm(sized.(Model))
	m.view = viewSources
	out := m.View()
	if !strings.Contains(out, "1.1.1.1") {
		t.Error("sources view should list a known source IP")
	}
	if !strings.Contains(out, "CANARY") {
		t.Error("sources view should show the per-source breakdown header")
	}
}

func TestTimelineViewShowsDistributions(t *testing.T) {
	m := NewModel(make(chan event.Event), "observe", 50)
	sized, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = feedSwarm(sized.(Model))
	m.view = viewTimeline
	out := m.View()
	for _, want := range []string{"BY KIND", "BY SEVERITY", "decoy_hit"} {
		if !strings.Contains(out, want) {
			t.Errorf("timeline view missing %q", want)
		}
	}
}
