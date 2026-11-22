package watch

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Armur-Ai/heramba/internal/event"
)

// --- bubbletea messages -----------------------------------------------------

type eventMsg event.Event
type streamClosedMsg struct{}
type tickMsg time.Time

// waitForEvent returns a command that blocks for the next event on ch and
// re-arms itself after each one, so the dashboard streams without polling.
func waitForEvent(ch <-chan event.Event) tea.Cmd {
	return func() tea.Msg {
		e, ok := <-ch
		if !ok {
			return streamClosedMsg{}
		}
		return eventMsg(e)
	}
}

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// --- model ------------------------------------------------------------------

// Model is the `heramba watch` dashboard: a bubbletea program that folds the
// live event stream into State and renders it as a real-time "you're being
// attacked right now" view.
type Model struct {
	state   *State
	ch      <-chan event.Event
	mode    string // observe / protect — informational, set from the flag
	started time.Time
	now     time.Time
	paused  bool
	closed  bool
	view    viewMode // which body is shown (overview / sources / campaigns / timeline)
	width   int
	height  int
}

// NewModel builds a dashboard over the given event channel. mode is shown in
// the header; maxRecent caps the live feed.
func NewModel(ch <-chan event.Event, mode string, maxRecent int) Model {
	if mode == "" {
		mode = "observe"
	}
	return Model{
		state:   NewState(maxRecent),
		ch:      ch,
		mode:    mode,
		started: time.Now(),
		now:     time.Now(),
		width:   100,
		height:  30,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(waitForEvent(m.ch), tick())
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case " ", "p":
			m.paused = !m.paused
			return m, nil
		case "c":
			m.state = NewState(m.state.maxRecent)
			return m, nil
		case "tab", "right", "l":
			m.view = m.view.next()
			return m, nil
		case "shift+tab", "left", "h":
			m.view = m.view.prev()
			return m, nil
		case "1", "2", "3", "4":
			if v := viewMode(msg.String()[0] - '1'); v >= 0 && v < numViews {
				m.view = v
			}
			return m, nil
		}
		return m, nil

	case eventMsg:
		if !m.paused {
			m.state.Add(event.Event(msg))
		}
		return m, waitForEvent(m.ch) // re-arm for the next event

	case streamClosedMsg:
		m.closed = true
		return m, nil

	case tickMsg:
		m.now = time.Time(msg)
		m.state.Tick(m.now) // keep the rate sparkline scrolling when quiet
		return m, tick()
	}
	return m, nil
}

// --- rendering --------------------------------------------------------------

// Theme colors — match demo/watch.tape (Heramba green / cyan / slate).
var (
	colBg     = lipgloss.Color("#0b0e14")
	colFg     = lipgloss.Color("#e6e9ef")
	colGreen  = lipgloss.Color("#3ddc97")
	colCyan   = lipgloss.Color("#57c7ff")
	colAmber  = lipgloss.Color("#f5a524")
	colRed    = lipgloss.Color("#ff6b6b")
	colViolet = lipgloss.Color("#a78bfa")
	colMuted  = lipgloss.Color("#5c6577")

	styTitle   = lipgloss.NewStyle().Bold(true).Foreground(colBg).Background(colGreen).Padding(0, 1)
	styMode    = lipgloss.NewStyle().Bold(true).Padding(0, 1)
	styMuted   = lipgloss.NewStyle().Foreground(colMuted)
	styHead    = lipgloss.NewStyle().Bold(true).Foreground(colCyan)
	stySwarm   = lipgloss.NewStyle().Bold(true).Foreground(colRed)
	styKey     = lipgloss.NewStyle().Foreground(colGreen)
	panelStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(colMuted).Padding(0, 1)
)

// sevAbbr is a fixed 4-column severity tag, so the sources table stays aligned
// (the full "critical" would be truncated to an unreadable "CRI…").
func sevAbbr(s event.Severity) string {
	switch s {
	case event.SeverityCritical:
		return "CRIT"
	case event.SeverityHigh:
		return "HIGH"
	case event.SeverityMedium:
		return "MED"
	case event.SeverityLow:
		return "LOW"
	default:
		return "INFO"
	}
}

func sevColor(s event.Severity) lipgloss.Color {
	switch s {
	case event.SeverityCritical:
		return colRed
	case event.SeverityHigh:
		return colAmber
	case event.SeverityMedium:
		return colViolet
	case event.SeverityLow:
		return colCyan
	default:
		return colMuted
	}
}

func kindDot(k event.Kind) string {
	switch k {
	case event.KindDecoyHit, event.KindCanaryTriggered:
		return lipgloss.NewStyle().Foreground(colRed).Render("●")
	case event.KindEgressBlocked:
		return lipgloss.NewStyle().Foreground(colAmber).Render("◆")
	case event.KindAgenticTraffic:
		return lipgloss.NewStyle().Foreground(colViolet).Render("▲")
	case event.KindRateLimited:
		return lipgloss.NewStyle().Foreground(colCyan).Render("■")
	default:
		return styMuted.Render("·")
	}
}

func (m Model) View() string {
	var b strings.Builder
	b.WriteString(m.header())
	b.WriteString("\n")
	b.WriteString(m.summary())
	b.WriteString("\n")
	b.WriteString(m.tabBar())
	b.WriteString("\n")

	bodyH := m.height - 11
	if bodyH < 6 {
		bodyH = 6
	}
	b.WriteString(m.body(bodyH))
	b.WriteString("\n")
	b.WriteString(m.footer())
	return b.String()
}

// body renders the active view, each in a full-width bordered panel (the
// overview is the split feed + sidebar).
func (m Model) body(bodyH int) string {
	switch m.view {
	case viewSources:
		return panelStyle.Width(m.width - 2).Height(bodyH).Render(m.sourcesView(m.width-4, bodyH-1))
	case viewCampaigns:
		return panelStyle.Width(m.width - 2).Height(bodyH).Render(m.campaignsView(m.width-4, bodyH-1))
	case viewTimeline:
		return panelStyle.Width(m.width - 2).Height(bodyH).Render(m.timelineView(m.width-4, bodyH-1))
	default:
		return m.overview(bodyH)
	}
}

// overview is the default split: live feed on the left, source/campaign/decoy
// sidebar on the right.
func (m Model) overview(bodyH int) string {
	feedW := m.width*3/5 - 2
	sideW := m.width - feedW - 5
	if feedW < 30 {
		feedW = 30
	}
	if sideW < 24 {
		sideW = 24
	}
	// Panels carry 1 column of horizontal padding on each side, so the usable
	// text width is the panel width minus 2. Lay content out to that exact inner
	// width so rows never wrap inside the border.
	feed := panelStyle.Width(feedW).Height(bodyH).Render(m.feed(feedW-2, bodyH-1))
	side := panelStyle.Width(sideW).Height(bodyH).Render(m.sidebar(sideW - 2))
	return lipgloss.JoinHorizontal(lipgloss.Top, feed, side)
}

func (m Model) header() string {
	modeStyle := styMode.Foreground(colBg)
	if m.mode == "protect" {
		modeStyle = modeStyle.Background(colAmber)
	} else {
		modeStyle = modeStyle.Background(colCyan)
	}
	left := styTitle.Render("🛡 HERAMBA · watch") + " " + modeStyle.Render(strings.ToUpper(m.mode))

	// Right cluster: live rate sparkline, status, uptime.
	up := m.now.Sub(m.started).Round(time.Second)
	var rightParts []string
	if spark := m.rateBadge(); spark != "" {
		rightParts = append(rightParts, spark)
	}
	if m.paused {
		rightParts = append(rightParts, stySwarm.Render("⏸ PAUSED"))
	} else if m.closed {
		rightParts = append(rightParts, styMuted.Render("stream ended"))
	}
	rightParts = append(rightParts, styMuted.Render("uptime "+fmtDur(up)))
	right := strings.Join(rightParts, styMuted.Render("  ·  "))

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

// rateBadge is the live events/sec sparkline plus the current rate, shown in
// the header's right cluster when the terminal is wide enough to carry it.
func (m Model) rateBadge() string {
	if m.width < 80 {
		return ""
	}
	spark := lipgloss.NewStyle().Foreground(colCyan).Render(sparkline(m.state.RateSeries(), 16))
	return spark + " " + styMuted.Render(fmt.Sprintf("%.0f/s", m.state.RateNow()))
}

func (m Model) summary() string {
	// Line 1: the threat gauge — a colored meter + level + score, the first
	// thing the eye lands on.
	level := m.state.ThreatLevel()
	score := m.state.ThreatScore()
	gaugeW := m.width/3 - 6
	if gaugeW < 10 {
		gaugeW = 10
	}
	if gaugeW > 32 {
		gaugeW = 32
	}
	levelStyle := lipgloss.NewStyle().Bold(true).Foreground(level.Color())
	gauge := strings.Join([]string{
		styMuted.Render("THREAT"),
		coloredMeter(score, gaugeW, level.Color()),
		levelStyle.Render(fmt.Sprintf("%s %2.0f%%", level.String(), score*100)),
	}, " ")

	counts := []string{
		fmt.Sprintf("%s events", num(m.state.Total)),
		fmt.Sprintf("%s sources", num(m.state.SourceCount())),
	}
	if sw := m.state.SwarmCount(); sw > 0 {
		counts = append(counts, stySwarm.Render(fmt.Sprintf("⚡ %d SWARM", sw)))
	}
	line1 := gauge + styMuted.Render("   │   ") + strings.Join(counts, styMuted.Render("  ·  "))

	// Line 2: the headline kind tallies.
	line2 := strings.Join([]string{
		tally("decoy", m.state.DecoyHits(), colRed),
		tally("canary", m.state.CanaryHits(), colRed),
		tally("agentic", m.state.AgenticFlags(), colViolet),
		tally("egress", m.state.EgressBlocks(), colAmber),
		tally("rate-limited", m.state.ByKind[event.KindRateLimited], colCyan),
	}, "   ")
	return line1 + "\n" + line2
}

func (m Model) feed(w, rows int) string {
	out := []string{styHead.Render("LIVE FEED") + styMuted.Render(fmt.Sprintf("  (%d)", m.state.Total))}
	evs := m.state.Recent(rows)
	if len(evs) == 0 {
		out = append(out, styMuted.Render("waiting for traffic…"))
		return strings.Join(out, "\n")
	}
	// Fixed-width columns, each laid out on plain text then styled, so rows
	// stay aligned and no ANSI code is ever cut. Columns: time, dot, kind,
	// source, then target fills the remainder.
	const tsW, kindW, srcW = 8, 16, 16
	gutter := 1 + 1 + 1 + 1 // single spaces between the five columns
	tgtW := w - tsW - 1 - kindW - srcW - gutter
	if tgtW < 6 {
		tgtW = 6
	}
	for _, e := range evs {
		ts := cell(e.Time.Local().Format("15:04:05"), tsW, styMuted)
		kind := cell(string(e.Kind), kindW, lipgloss.NewStyle().Foreground(sevColor(e.Severity)))
		src := cell(e.Source, srcW, lipgloss.NewStyle().Foreground(colFg))
		tgt := cell(e.Target, tgtW, styMuted)
		out = append(out, strings.Join([]string{ts, kindDot(e.Kind), kind, src, tgt}, " "))
	}
	return strings.Join(out, "\n")
}

func (m Model) sidebar(w int) string {
	var secs []string

	// TOP SOURCES — "addr SEV ×count", count right-aligned.
	srcLines := []string{styHead.Render("TOP SOURCES")}
	for _, s := range m.state.TopSources(5) {
		sev := cell(sevAbbr(s.MaxSeverity), 4, lipgloss.NewStyle().Foreground(sevColor(s.MaxSeverity)))
		cnt := styMuted.Render(padLeft(fmt.Sprintf("×%d", s.Count), 5))
		addr := cell(s.Addr, w-4-5-2, lipgloss.NewStyle().Foreground(colFg))
		srcLines = append(srcLines, addr+" "+sev+" "+cnt)
	}
	if len(srcLines) == 1 {
		srcLines = append(srcLines, styMuted.Render("—"))
	}
	secs = append(secs, strings.Join(srcLines, "\n"))

	// CAMPAIGNS — swarms highlighted.
	campLines := []string{styHead.Render("CAMPAIGNS")}
	for _, c := range m.state.Campaigns(4) {
		name := strings.TrimPrefix(c.ID, "src:")
		if c.Swarm() {
			tag := fmt.Sprintf(" ⚡%d src", len(c.Sources))
			campLines = append(campLines, cell("⚡ "+name, w-dw(tag), stySwarm)+stySwarm.Render(tag))
		} else {
			campLines = append(campLines, cell("• "+name, w, styMuted))
		}
	}
	if len(campLines) == 1 {
		campLines = append(campLines, styMuted.Render("—"))
	}
	secs = append(secs, strings.Join(campLines, "\n"))

	// DECOY MAP — target with a hit bar.
	decoyLines := []string{styHead.Render("DECOY MAP")}
	peak := 1
	for _, d := range m.state.TopDecoys(5) {
		if d.Hits > peak {
			peak = d.Hits
		}
	}
	for _, d := range m.state.TopDecoys(5) {
		cnt := styMuted.Render(padLeft(fmt.Sprintf("×%d", d.Hits), 5))
		barW := 6
		bar := coloredMeter(float64(d.Hits)/float64(peak), barW, colRed)
		tgt := cell(d.Target, w-barW-5-2, lipgloss.NewStyle().Foreground(colFg))
		decoyLines = append(decoyLines, tgt+" "+bar+" "+cnt)
	}
	if len(decoyLines) == 1 {
		decoyLines = append(decoyLines, styMuted.Render("—"))
	}
	secs = append(secs, strings.Join(decoyLines, "\n"))

	return strings.Join(secs, "\n\n")
}

func (m Model) footer() string {
	keys := []string{
		styKey.Render("tab") + "/" + styKey.Render("1-4") + " views",
		styKey.Render("space") + " pause",
		styKey.Render("c") + " clear",
		styKey.Render("q") + " quit",
	}
	return styMuted.Render(strings.Join(keys, "   ·   "))
}

// --- small helpers ----------------------------------------------------------

func tally(label string, n int, c lipgloss.Color) string {
	return lipgloss.NewStyle().Foreground(c).Bold(true).Render(fmt.Sprintf("%d", n)) + " " + styMuted.Render(label)
}

func num(n int) string { return fmt.Sprintf("%d", n) }

func fmtDur(d time.Duration) string {
	h := int(d.Hours())
	mm := int(d.Minutes()) % 60
	ss := int(d.Seconds()) % 60
	return fmt.Sprintf("%02d:%02d:%02d", h, mm, ss)
}
