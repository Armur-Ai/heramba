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

	feedW := m.width*3/5 - 2
	sideW := m.width - feedW - 5
	if feedW < 30 {
		feedW = 30
	}
	if sideW < 24 {
		sideW = 24
	}
	bodyH := m.height - 9
	if bodyH < 6 {
		bodyH = 6
	}

	feed := panelStyle.Width(feedW).Height(bodyH).Render(m.feed(feedW, bodyH-1))
	side := panelStyle.Width(sideW).Height(bodyH).Render(m.sidebar(sideW))
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, feed, side))
	b.WriteString("\n")
	b.WriteString(m.footer())
	return b.String()
}

func (m Model) header() string {
	modeStyle := styMode.Foreground(colBg)
	if m.mode == "protect" {
		modeStyle = modeStyle.Background(colAmber)
	} else {
		modeStyle = modeStyle.Background(colCyan)
	}
	up := m.now.Sub(m.started).Round(time.Second)
	right := styMuted.Render(fmt.Sprintf("uptime %s", fmtDur(up)))
	if m.paused {
		right = stySwarm.Render("⏸ PAUSED") + "  " + right
	}
	if m.closed {
		right = styMuted.Render("stream ended") + "  " + right
	}
	left := styTitle.Render("🛡 HERAMBA · watch") + " " + modeStyle.Render(strings.ToUpper(m.mode))
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

func (m Model) summary() string {
	sev := "QUIET"
	sevStyle := styMuted
	if m.state.Total > 0 {
		sev = strings.ToUpper(m.state.HighestSeverity.String())
		sevStyle = lipgloss.NewStyle().Bold(true).Foreground(sevColor(m.state.HighestSeverity))
	}
	parts := []string{
		sevStyle.Render("⚠ " + sev),
		fmt.Sprintf("%s events", num(m.state.Total)),
		fmt.Sprintf("%s sources", num(m.state.SourceCount())),
	}
	if sw := m.state.SwarmCount(); sw > 0 {
		parts = append(parts, stySwarm.Render(fmt.Sprintf("⚡ %d SWARM", sw)))
	}
	line1 := strings.Join(parts, styMuted.Render("  │  "))

	line2 := strings.Join([]string{
		tally("decoy", m.state.DecoyHits(), colRed),
		tally("canary", m.state.CanaryHits(), colRed),
		tally("agentic", m.state.AgenticFlags(), colViolet),
		tally("egress", m.state.EgressBlocks(), colAmber),
	}, "   ")
	return line1 + "\n" + styMuted.Render(line2)
}

func (m Model) feed(w, rows int) string {
	out := []string{styHead.Render("LIVE FEED")}
	evs := m.state.Recent(rows)
	if len(evs) == 0 {
		out = append(out, styMuted.Render("waiting for traffic…"))
	}
	for _, e := range evs {
		ts := styMuted.Render(e.Time.Local().Format("15:04:05"))
		kind := lipgloss.NewStyle().Foreground(sevColor(e.Severity)).Render(padRight(string(e.Kind), 16))
		src := lipgloss.NewStyle().Foreground(colFg).Render(padRight(e.Source, 15))
		tgt := styMuted.Render(e.Target)
		line := fmt.Sprintf("%s %s %s %s %s", ts, kindDot(e.Kind), kind, src, tgt)
		out = append(out, truncate(line, w))
	}
	return strings.Join(out, "\n")
}

func (m Model) sidebar(w int) string {
	var secs []string

	srcLines := []string{styHead.Render("TOP SOURCES")}
	for _, s := range m.state.TopSources(5) {
		sev := lipgloss.NewStyle().Foreground(sevColor(s.MaxSeverity)).Render(padRight(strings.ToUpper(s.MaxSeverity.String()), 4))
		srcLines = append(srcLines, truncate(fmt.Sprintf("%s %s ×%d", padRight(s.Addr, 15), sev, s.Count), w))
	}
	if len(srcLines) == 1 {
		srcLines = append(srcLines, styMuted.Render("—"))
	}
	secs = append(secs, strings.Join(srcLines, "\n"))

	campLines := []string{styHead.Render("CAMPAIGNS")}
	for _, c := range m.state.Campaigns(4) {
		name := strings.TrimPrefix(c.ID, "src:")
		if c.Swarm() {
			campLines = append(campLines, truncate(stySwarm.Render("⚡ "+name)+styMuted.Render(fmt.Sprintf(" swarm·%d src", len(c.Sources))), w))
		} else {
			campLines = append(campLines, truncate(styMuted.Render("• "+name), w))
		}
	}
	if len(campLines) == 1 {
		campLines = append(campLines, styMuted.Render("—"))
	}
	secs = append(secs, strings.Join(campLines, "\n"))

	decoyLines := []string{styHead.Render("DECOY MAP")}
	for _, d := range m.state.TopDecoys(5) {
		decoyLines = append(decoyLines, truncate(fmt.Sprintf("%s %s", padRight(d.Target, w-6), styMuted.Render(fmt.Sprintf("×%d", d.Hits))), w))
	}
	if len(decoyLines) == 1 {
		decoyLines = append(decoyLines, styMuted.Render("—"))
	}
	secs = append(secs, strings.Join(decoyLines, "\n"))

	return strings.Join(secs, "\n\n")
}

func (m Model) footer() string {
	keys := []string{
		styKey.Render("q") + " quit",
		styKey.Render("space") + " pause",
		styKey.Render("c") + " clear",
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

func padRight(s string, n int) string {
	if lipgloss.Width(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-lipgloss.Width(s))
}

func truncate(s string, w int) string {
	if w <= 1 || lipgloss.Width(s) <= w {
		return s
	}
	// Trim by runes until it fits (styled strings carry ANSI; approximate).
	r := []rune(s)
	for len(r) > 0 && lipgloss.Width(string(r)) > w-1 {
		r = r[:len(r)-1]
	}
	return string(r) + "…"
}
