package coord

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/Armur-Ai/heramba/internal/research/stats"
)

// WriteJSON emits the machine-readable report (deterministic except throughput).
func (r Report) WriteJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

// WriteMarkdown emits a human-readable summary with uncertainty.
func (r Report) WriteMarkdown(w io.Writer) error {
	p := func(format string, a ...any) error {
		_, err := fmt.Fprintf(w, format, a...)
		return err
	}
	if err := p("# Fair coordination-detection experiment (R2)\n\n"); err != nil {
		return err
	}
	p("- code revision: `%s`\n", r.CodeRevision)
	p("- cohorts: %d (%d coordinated / %d not)\n", r.Cohorts, r.Positives, r.Negatives)
	p("- families: %v\n", r.Families)
	p("- matched: %d requests, %s window per cohort (aggregate rate is not discriminative)\n\n", r.Params.MatchedRequests, r.Params.Window)

	p("## Methods compared (operating threshold %.2f)\n\n", OperatingThreshold)
	p("| method | recall [95%% CI] | FPR [95%% CI] | precision | PR-AUC |\n")
	p("|---|---|---|---|---|\n")
	for _, m := range r.Methods {
		p("| %s | %s | %s | %s | %.3f |\n", m.Method, ci(m.Recall), ci(m.FPR), ci(m.Precision), m.PRAUC)
	}
	p("\n## Ablations (full detector, one signal removed)\n\n")
	p("| ablation | recall [95%% CI] | FPR [95%% CI] | PR-AUC |\n")
	p("|---|---|---|---|\n")
	for _, m := range r.Ablations {
		p("| %s | %s | %s | %.3f |\n", m.Method, ci(m.Recall), ci(m.FPR), m.PRAUC)
	}

	p("\n## Per-family confusion (full_heramba, stratified)\n\n")
	p("| family | kind | coordinated | TP | FP | TN | FN |\n")
	p("|---|---|---|---|---|---|---|\n")
	for _, fc := range r.PerFamily {
		if fc.Method != "full_heramba" {
			continue
		}
		c := fc.Confusion
		p("| %s | %s | %v | %d | %d | %d | %d |\n", fc.Family, fc.ActorKind, fc.Coordinated, c.TP, c.FP, c.TN, c.FN)
	}

	p("\n## Secondary endpoints\n\n")
	p("- detection latency (full_heramba, coordinated, session-granular): %.1fs [%.1f, %.1f] over n=%d\n",
		r.DetectionLatency.Point, r.DetectionLatency.Lo, r.DetectionLatency.Hi, r.DetectionLatency.N)
	p("- throughput: %.0f classifications/sec (timing measurement, non-deterministic)\n\n", r.ThroughputRPS)

	p("## Not measured\n\n")
	for _, n := range r.NotMeasured {
		p("- %s\n", n)
	}
	return nil
}

func ci(iv stats.Interval) string {
	return fmt.Sprintf("%.3f [%.3f, %.3f]", iv.Point, iv.Lo, iv.Hi)
}
