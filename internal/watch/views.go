package watch

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/Armur-Ai/heramba/internal/event"
)

// viewMode selects which body the dashboard shows. The overview is the default
// split (feed + sidebar); the others are focused, full-width tables for when an
// operator wants to drill into one dimension.
type viewMode int

const (
	viewOverview viewMode = iota
	viewSources
	viewCampaigns
	viewTimeline
	numViews
)

func (v viewMode) title() string {
	switch v {
	case viewOverview:
		return "OVERVIEW"
	case viewSources:
		return "SOURCES"
	case viewCampaigns:
		return "CAMPAIGNS"
	case viewTimeline:
		return "TIMELINE"
	default:
		return "?"
	}
}

// next / prev cycle through the views, wrapping at the ends.
func (v viewMode) next() viewMode { return (v + 1) % numViews }
func (v viewMode) prev() viewMode { return (v + numViews - 1) % numViews }

// tabBar renders the view switcher: the active tab is reverse-highlighted, the
// rest muted, each prefixed with its number key.
func (m Model) tabBar() string {
	var tabs []string
	for v := viewOverview; v < numViews; v++ {
		label := fmt.Sprintf(" %d %s ", int(v)+1, v.title())
		if v == m.view {
			tabs = append(tabs, lipgloss.NewStyle().Bold(true).Foreground(colBg).Background(colCyan).Render(label))
		} else {
			tabs = append(tabs, styMuted.Render(label))
		}
	}
	return strings.Join(tabs, styMuted.Render("│"))
}

// sourcesView is a full-width table of every known source, ranked by urgency,
// with its decoy/canary/agentic/egress breakdown and a severity tag.
func (m Model) sourcesView(w, h int) string {
	head := styHead.Render("SOURCES") + styMuted.Render(fmt.Sprintf("  %d distinct", m.state.SourceCount()))
	// 7 columns (source, sev, hits, decoy, canary, agent, egress) joined by 6
	// single-space separators. The source column takes whatever is left.
	addrW := w - (4 + 6 + 6 + 6 + 6 + 6) - 6
	if addrW < 10 {
		addrW = 10
	}
	cols := lipgloss.NewStyle().Foreground(colMuted).Bold(true).Render(
		fit("SOURCE", addrW) + " " +
			fit("SEV", 4) + " " + padLeft("HITS", 6) + " " + padLeft("DECOY", 6) + " " +
			padLeft("CANARY", 6) + " " + padLeft("AGENT", 6) + " " + padLeft("EGRESS", 6))
	lines := []string{head, cols}
	for _, s := range m.state.TopSources(h - 3) {
		sev := cell(sevAbbr(s.MaxSeverity), 4, lipgloss.NewStyle().Foreground(sevColor(s.MaxSeverity)))
		row := cell(s.Addr, addrW, lipgloss.NewStyle().Foreground(colFg)) + " " + sev + " " +
			styMuted.Render(padLeft(fmt.Sprintf("%d", s.Count), 6)) + " " +
			numCell(s.Decoys, 6, colRed) + " " +
			numCell(s.Canaries, 6, colRed) + " " +
			numCell(s.Agentic, 6, colViolet) + " " +
			numCell(s.Egress, 6, colAmber)
		lines = append(lines, row)
	}
	if len(lines) == 2 {
		lines = append(lines, styMuted.Render("no sources yet — waiting for traffic…"))
	}
	return strings.Join(lines, "\n")
}

// campaignsView focuses on clustering: swarms (multi-source coordinated
// efforts) first, each with its member sources and severity.
func (m Model) campaignsView(w, h int) string {
	head := styHead.Render("CAMPAIGNS") + styMuted.Render(fmt.Sprintf("  %d clusters · %d swarms", m.state.CampaignCount(), m.state.SwarmCount()))
	lines := []string{head}
	for _, c := range m.state.Campaigns(h - 2) {
		name := strings.TrimPrefix(c.ID, "src:")
		sev := cell(sevAbbr(c.MaxSeverity), 4, lipgloss.NewStyle().Foreground(sevColor(c.MaxSeverity)))
		if c.Swarm() {
			tag := stySwarm.Render(fmt.Sprintf("⚡ SWARM ×%d src", len(c.Sources)))
			title := cell("⚡ "+name, w-24, stySwarm)
			lines = append(lines, title+" "+sev+" "+styMuted.Render(padLeft(fmt.Sprintf("%d hits", c.Count), 9)))
			lines = append(lines, "   "+tag+"  "+styMuted.Render(truncPlain(strings.Join(sortedMembers(c), ", "), w-20)))
		} else {
			title := cell("• "+name, w-20, styMuted)
			lines = append(lines, title+" "+sev+" "+styMuted.Render(padLeft(fmt.Sprintf("%d hits", c.Count), 9)))
		}
	}
	if len(lines) == 1 {
		lines = append(lines, styMuted.Render("no campaigns yet…"))
	}
	return strings.Join(lines, "\n")
}

// sortedMembers returns a campaign's member sources in stable order for display.
func sortedMembers(c Campaign) []string {
	out := make([]string, 0, len(c.Sources))
	for s := range c.Sources {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// numCell renders a count right-aligned in width w, muted when zero so the live
// numbers stand out against a sea of zeros.
func numCell(n, w int, c lipgloss.Color) string {
	if n == 0 {
		return styMuted.Render(padLeft("·", w))
	}
	return lipgloss.NewStyle().Foreground(c).Render(padLeft(fmt.Sprintf("%d", n), w))
}

// timelineView shows a large events/sec sparkline over the retained window plus
// the per-kind and per-severity distributions as labeled bars.
func (m Model) timelineView(w, h int) string {
	lines := []string{styHead.Render("TIMELINE") + styMuted.Render(fmt.Sprintf("  peak %.0f/s", m.state.RatePeak()))}

	sparkW := w - 2
	spark := lipgloss.NewStyle().Foreground(colCyan).Render(sparkline(m.state.RateSeries(), sparkW))
	lines = append(lines, spark, styMuted.Render(fmt.Sprintf("events/sec over the last %ds", len(m.state.RateSeries()))), "")

	lines = append(lines, styHead.Render("BY KIND"))
	lines = append(lines, distBars(kindCounts(m.state), w)...)
	lines = append(lines, "", styHead.Render("BY SEVERITY"))
	lines = append(lines, distBars(severityCounts(m.state), w)...)
	return strings.Join(lines, "\n")
}

// labeledCount is one row of a distribution.
type labeledCount struct {
	label string
	count int
	color lipgloss.Color
}

// distBars renders labeled counts as proportional bars (shared max scale).
func distBars(items []labeledCount, w int) []string {
	if len(items) == 0 {
		return []string{styMuted.Render("—")}
	}
	max := 1
	for _, it := range items {
		if it.count > max {
			max = it.count
		}
	}
	labelW := 16
	barW := w - labelW - 10
	if barW < 8 {
		barW = 8
	}
	out := make([]string, 0, len(items))
	for _, it := range items {
		bar := coloredMeter(float64(it.count)/float64(max), barW, it.color)
		out = append(out, cell(it.label, labelW, lipgloss.NewStyle().Foreground(colFg))+" "+bar+" "+styMuted.Render(padLeft(fmt.Sprintf("%d", it.count), 6)))
	}
	return out
}

func kindCounts(s *State) []labeledCount {
	order := []event.Kind{
		event.KindDecoyHit, event.KindCanaryTriggered, event.KindAgenticTraffic,
		event.KindEgressBlocked, event.KindRateLimited, event.KindCounterDeployed, event.KindProbe,
	}
	colors := map[event.Kind]lipgloss.Color{
		event.KindDecoyHit: colRed, event.KindCanaryTriggered: colRed,
		event.KindAgenticTraffic: colViolet, event.KindEgressBlocked: colAmber,
		event.KindRateLimited: colCyan, event.KindCounterDeployed: colGreen, event.KindProbe: colMuted,
	}
	var out []labeledCount
	for _, k := range order {
		if n := s.ByKind[k]; n > 0 {
			out = append(out, labeledCount{string(k), n, colors[k]})
		}
	}
	return out
}

func severityCounts(s *State) []labeledCount {
	order := []event.Severity{
		event.SeverityCritical, event.SeverityHigh, event.SeverityMedium,
		event.SeverityLow, event.SeverityInfo,
	}
	var out []labeledCount
	for _, sev := range order {
		if n := s.BySeverity[sev]; n > 0 {
			out = append(out, labeledCount{strings.ToUpper(sev.String()), n, sevColor(sev)})
		}
	}
	return out
}
