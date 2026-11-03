package coevolve

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

// WriteMarkdown emits a human-readable summary with uncertainty, focused on the
// adaptive mode but including the no-adaptation and instant bounds.
func (r Report) WriteMarkdown(w io.Writer) error {
	p := func(format string, a ...any) { fmt.Fprintf(w, format, a...) }
	p("# Adaptation race & co-evolution lab (R4)\n\n")
	p("- code revision: `%s`\n", r.CodeRevision)
	p("- %d rounds/race, %d seeds; strictly local, seeded, event-sourced\n\n", r.Rounds, len(r.Seeds))

	p("## Adaptive defender (adaptation latency = detection + response)\n\n")
	p("| archetype | countered | adaptation latency (s) | detection | response | mean utility | util σ | oscillation |\n")
	p("|---|---|---|---|---|---|---|---|\n")
	for _, res := range r.Results {
		if res.Mode != "adaptive" {
			continue
		}
		p("| %s | %s | %s | %s | %s | %s | %.3f | %s |\n",
			res.Archetype, ci(res.CounteredRate), lat(res.AdaptationLatency), lat(res.DetectionComponent),
			lat(res.ResponseComponent), ci(res.MeanUtility), res.UtilityStdDev, ci(res.OscillationRate))
	}

	p("\n## Ablation: utility by defender mode (mean [95%% CI])\n\n")
	p("| archetype | no_adaptation | adaptive | instant |\n")
	p("|---|---|---|---|\n")
	byArchMode := map[string]map[string]stats.Interval{}
	for _, res := range r.Results {
		if byArchMode[res.Archetype] == nil {
			byArchMode[res.Archetype] = map[string]stats.Interval{}
		}
		byArchMode[res.Archetype][res.Mode] = res.MeanUtility
	}
	for _, arch := range Archetypes() {
		m := byArchMode[arch.Name]
		p("| %s | %s | %s | %s |\n", arch.Name, ci(m["no_adaptation"]), ci(m["adaptive"]), ci(m["instant"]))
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

func lat(iv stats.Interval) string {
	if iv.N == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%.1f [%.1f, %.1f]", iv.Point, iv.Lo, iv.Hi)
}
