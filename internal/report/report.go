// Package report renders a shareable incident/campaign report from the same
// aggregated detection state the live dashboard uses. Point it at a recorded
// JSONL capture and hand someone a summary of what the edge saw — who attacked,
// how it clustered, and what deception they walked into.
package report

import (
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/Armur-Ai/heramba/internal/watch"
)

// Markdown renders the report as GitHub-flavored Markdown.
func Markdown(s *watch.State, title string) string {
	if title == "" {
		title = "Heramba incident report"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", title)
	fmt.Fprintf(&b, "- **Window:** %s → %s\n", fmtTime(s.First), fmtTime(s.Last))
	fmt.Fprintf(&b, "- **Detections:** %d across %d source(s), %d campaign(s)\n", s.Total, s.SourceCount(), s.CampaignCount())
	fmt.Fprintf(&b, "- **Highest severity:** %s\n", s.HighestSeverity)
	if sw := s.SwarmCount(); sw > 0 {
		fmt.Fprintf(&b, "- **⚡ Coordinated swarms:** %d\n", sw)
	}
	fmt.Fprintf(&b, "- **Tally:** %d decoy · %d canary · %d agentic · %d egress-blocked\n\n",
		s.DecoyHits(), s.CanaryHits(), s.AgenticFlags(), s.EgressBlocks())

	b.WriteString("## Top sources\n\n| Source | Severity | Events | Decoys | Canaries |\n|---|---|---|---|---|\n")
	for _, src := range s.TopSources(10) {
		fmt.Fprintf(&b, "| `%s` | %s | %d | %d | %d |\n", src.Addr, src.MaxSeverity, src.Count, src.Decoys, src.Canaries)
	}

	b.WriteString("\n## Campaigns\n\n")
	for _, c := range s.Campaigns(10) {
		tag := "•"
		if c.Swarm() {
			tag = "⚡ swarm"
		}
		fmt.Fprintf(&b, "- %s **%s** — %d events, %d source(s)\n", tag, strings.TrimPrefix(c.ID, "src:"), c.Count, len(c.Sources))
	}

	b.WriteString("\n## Decoy / canary map\n\n")
	for _, d := range s.TopDecoys(15) {
		fmt.Fprintf(&b, "- `%s` ×%d\n", d.Target, d.Hits)
	}
	return b.String()
}

// HTML renders a self-contained HTML report (inline CSS, no external assets).
func HTML(s *watch.State, title string) string {
	if title == "" {
		title = "Heramba incident report"
	}
	var b strings.Builder
	b.WriteString("<!doctype html><html><head><meta charset=\"utf-8\">")
	fmt.Fprintf(&b, "<title>%s</title>", html.EscapeString(title))
	b.WriteString(`<style>
body{font-family:system-ui,sans-serif;max-width:900px;margin:2rem auto;padding:0 1rem;color:#1a1a2e}
h1{color:#0b0e14}table{border-collapse:collapse;width:100%}th,td{border:1px solid #ddd;padding:.4rem .6rem;text-align:left}
th{background:#0b0e14;color:#e6e9ef}.swarm{color:#c0392b;font-weight:700}code{background:#f0f0f4;padding:.1rem .3rem;border-radius:3px}
.sum li{margin:.2rem 0}</style></head><body>`)
	fmt.Fprintf(&b, "<h1>%s</h1>", html.EscapeString(title))
	fmt.Fprintf(&b, `<ul class="sum"><li><b>Window:</b> %s → %s</li>`, fmtTime(s.First), fmtTime(s.Last))
	fmt.Fprintf(&b, "<li><b>Detections:</b> %d across %d source(s), %d campaign(s)</li>", s.Total, s.SourceCount(), s.CampaignCount())
	fmt.Fprintf(&b, "<li><b>Highest severity:</b> %s</li>", s.HighestSeverity)
	if sw := s.SwarmCount(); sw > 0 {
		fmt.Fprintf(&b, `<li class="swarm">⚡ Coordinated swarms: %d</li>`, sw)
	}
	fmt.Fprintf(&b, "<li><b>Tally:</b> %d decoy · %d canary · %d agentic · %d egress-blocked</li></ul>",
		s.DecoyHits(), s.CanaryHits(), s.AgenticFlags(), s.EgressBlocks())

	b.WriteString("<h2>Top sources</h2><table><tr><th>Source</th><th>Severity</th><th>Events</th><th>Decoys</th><th>Canaries</th></tr>")
	for _, src := range s.TopSources(10) {
		fmt.Fprintf(&b, "<tr><td><code>%s</code></td><td>%s</td><td>%d</td><td>%d</td><td>%d</td></tr>",
			html.EscapeString(src.Addr), src.MaxSeverity, src.Count, src.Decoys, src.Canaries)
	}
	b.WriteString("</table><h2>Campaigns</h2><ul>")
	for _, c := range s.Campaigns(10) {
		cls := ""
		label := "•"
		if c.Swarm() {
			cls, label = ` class="swarm"`, "⚡ swarm"
		}
		fmt.Fprintf(&b, "<li%s>%s <b>%s</b> — %d events, %d source(s)</li>",
			cls, label, html.EscapeString(strings.TrimPrefix(c.ID, "src:")), c.Count, len(c.Sources))
	}
	b.WriteString("</ul><h2>Decoy / canary map</h2><ul>")
	for _, d := range s.TopDecoys(15) {
		fmt.Fprintf(&b, "<li><code>%s</code> ×%d</li>", html.EscapeString(d.Target), d.Hits)
	}
	b.WriteString("</ul></body></html>")
	return b.String()
}

func fmtTime(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.UTC().Format("2006-01-02 15:04:05 UTC")
}
