package replay

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// WriteJSON emits the machine-readable experiment report.
func (e Experiment) WriteJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(e)
}

// WriteMarkdown emits a human-readable comparison table. Numbers are the
// measured means with ± sample standard deviation across trials. It states the
// comparison plainly and does not editorialize about which baseline "wins".
func (e Experiment) WriteMarkdown(w io.Writer) error {
	var b strings.Builder
	fmt.Fprintf(&b, "# Heramba four-baseline experiment\n\n")
	fmt.Fprintf(&b, "Schema v%d · %d seeded trials per baseline · identical scenarios.\n\n", e.SchemaVersion, len(e.Seeds))
	fmt.Fprintf(&b, "| baseline | TPR | FPR | objective completion | detect latency (s) | legit disruption |\n")
	fmt.Fprintf(&b, "|---|---|---|---|---|---|\n")
	for _, r := range e.Results {
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s |\n",
			r.Defense,
			fmtAgg(r.TPR), fmtAgg(r.FPR), fmtAgg(r.ObjectiveCompletion),
			fmtAggRaw(r.DetectionLatencySec), fmtAgg(r.LegitDisruption))
	}
	b.WriteString("\n**Reading the table.** TPR/FPR are over sources; objective completion is the")
	b.WriteString(" fraction of trials an attacker reached the goal uncontained; detection latency is")
	b.WriteString(" seconds from the first attacker action to the first true detection; legit disruption")
	b.WriteString(" is the fraction of benign requests wrongly blocked.\n\n")
	if len(e.NotMeasured) > 0 {
		b.WriteString("**Not measured by this harness (explicitly):**\n")
		for _, nm := range e.NotMeasured {
			fmt.Fprintf(&b, "- %s\n", nm)
		}
	}
	_, err := io.WriteString(w, b.String())
	return err
}

func fmtAgg(a Aggregate) string {
	return fmt.Sprintf("%.2f ± %.2f", a.Mean, a.StdDev)
}
func fmtAggRaw(a Aggregate) string {
	return fmt.Sprintf("%.1f ± %.1f", a.Mean, a.StdDev)
}
