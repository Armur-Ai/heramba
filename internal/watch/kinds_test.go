package watch

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Armur-Ai/heramba/internal/event"
)

// Active-defense counter deployments (poison / counter-lures) must be visible
// and distinct in the dashboard, not collapsed into the muted "·" default.
func TestCounterDeployedSurfaced(t *testing.T) {
	m := NewModel(make(chan event.Event), "protect", 50)
	s, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = s.(Model)
	e := event.New(event.KindCounterDeployed, event.SeverityMedium, "1.2.3.4", "/x", "poison").
		WithLabel("plane", "poison")
	nm, _ := m.Update(eventMsg(e))
	m = nm.(Model)

	out := m.View()
	if !strings.Contains(out, "✦") {
		t.Error("counter_deployed should render its ✦ dot in the feed")
	}
	if !strings.Contains(out, "counter") {
		t.Error("summary should include a counter tally")
	}
}

func TestKindDotDistinctPerKind(t *testing.T) {
	seen := map[string]bool{}
	for _, k := range []event.Kind{
		event.KindDecoyHit, event.KindEgressBlocked, event.KindAgenticTraffic,
		event.KindRateLimited, event.KindCounterDeployed,
	} {
		dot := kindDot(k)
		if dot == "" {
			t.Errorf("kind %s has no dot", k)
		}
		seen[dot] = true
	}
	if len(seen) < 4 {
		t.Errorf("kind dots should be visually distinct, got %d unique", len(seen))
	}
}
