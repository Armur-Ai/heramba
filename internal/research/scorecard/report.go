package scorecard

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/Armur-Ai/heramba/internal/research/stats"
)

// WriteJSON emits the machine-readable scorecard.
func (s Scorecard) WriteJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(s)
}

// WriteMarkdown emits the target-vs-observed scorecard.
func (s Scorecard) WriteMarkdown(w io.Writer) error {
	p := func(format string, a ...any) { fmt.Fprintf(w, format, a...) }
	p("# Targets vs. observed scorecard (R8)\n\n")
	p("- code revision: `%s`\n", s.CodeRevision)
	p("- targets are **aspirational evaluation thresholds, not claims**\n\n")

	p("| metric | target | observed [95%% CI] | met? | context |\n")
	p("|---|---|---|---|---|\n")
	for _, r := range s.Rows {
		p("| %s | %s | %s | %s | %s |\n", r.Metric, r.Target, ci(r.Observed), yesno(r.Met), r.Context)
	}

	p("\n## Effect sizes (matched comparisons, bootstrap 95%% CI)\n\n")
	for _, e := range s.EffectSizes {
		p("- **%s**: %s — %s\n", e.Name, ci(e.Estimate), e.Context)
	}

	p("\n## Notes\n\n")
	for _, n := range s.Notes {
		p("- %s\n", n)
	}
	return nil
}

func ci(iv stats.Interval) string {
	return fmt.Sprintf("%.3f [%.3f, %.3f] (n=%d)", iv.Point, iv.Lo, iv.Hi, iv.N)
}

func yesno(b bool) string {
	if b {
		return "✅ met"
	}
	return "❌ MISS"
}

func f3(x float64) string { return strconv.FormatFloat(x, 'f', 3, 64) }
func itoa(n int) string   { return strconv.Itoa(n) }
