package objective

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/Armur-Ai/heramba/internal/research/stats"
)

// WriteJSON emits the machine-readable report.
func (r Report) WriteJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

// WriteMarkdown emits a human-readable summary with uncertainty.
func (r Report) WriteMarkdown(w io.Writer) error {
	p := func(format string, a ...any) { fmt.Fprintf(w, format, a...) }
	p("# Deception & attack-objective experiment (R3)\n\n")
	p("- code revision: `%s`\n", r.CodeRevision)
	p("- attacker: budget %d, deception-aware=%v (reject threshold %.2f)\n",
		r.Config.Budget, r.Config.Attacker.DeceptionAware, r.Config.Attacker.AwareThreshold)
	p("- matched: depth %d, %d trials per arm\n\n", r.Config.Depth, len(r.Config.Seeds))

	p("| arm | objective completion [95%% CI] | time-to-objective (s) | diverted | detection lat (s) | deception discovery | benign disruption |\n")
	p("|---|---|---|---|---|---|---|\n")
	for _, a := range r.Arms {
		p("| %s | %s | %s | %s | %s | %s | %.3f |\n",
			a.Arm, ci(a.ObjectiveCompletion), ciRaw(a.TimeToObjectiveSec), ci(a.DivertedFraction),
			ciRaw(a.DetectionLatencySec), ci(a.DeceptionDiscovery), a.BenignDisruption)
	}
	p("\n## Not measured\n\n")
	for _, n := range r.NotMeasured {
		p("- %s\n", n)
	}
	return nil
}

func ci(iv stats.Interval) string {
	return fmt.Sprintf("%.3f [%.3f, %.3f]", iv.Point, iv.Lo, iv.Hi)
}

func ciRaw(iv stats.Interval) string {
	if iv.N == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%.1f [%.1f, %.1f]", iv.Point, iv.Lo, iv.Hi)
}
