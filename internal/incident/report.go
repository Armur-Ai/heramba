package incident

import (
	"encoding/json"
	"fmt"
	"io"
)

// WriteJSON emits the machine-readable incident report.
func (inc Incident) WriteJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(inc)
}

// WriteMarkdown emits an operator-facing incident report: the investigation
// timeline (observed vs inferred, confidence, uncertainty, contradicting
// evidence), the per-action blast radius and reversibility, the human-control
// gates, the retention policy, and the causal caveats.
func (inc Incident) WriteMarkdown(w io.Writer) error {
	p := func(format string, a ...any) { fmt.Fprintf(w, format, a...) }
	p("# Incident %s — %s\n\n", inc.ID, inc.Title)
	if !inc.Window[0].IsZero() {
		p("- window: %s → %s\n", inc.Window[0].Format("2006-01-02 15:04:05"), inc.Window[1].Format("15:04:05"))
	}
	p("- retention: %s\n\n", inc.Retention.MaxAge)

	p("## Investigation timeline (observed → inferred → action → outcome)\n\n")
	p("| time | stage | observed? | summary | conf | uncert | contra | source |\n")
	p("|---|---|---|---|---|---|---|---|\n")
	for _, e := range inc.Entries {
		obs := "inferred"
		if e.Observed {
			obs = "observed"
		}
		p("| %s | %s | %s | %s | %s | %s | %d | %s |\n",
			e.Time.Format("15:04:05"), e.Stage, obs, e.Summary,
			optF(e.Confidence), optF(e.Uncertainty), e.Contradicts, dash(e.SourceHealth))
	}

	p("\n## Actions — blast radius & reversibility\n\n")
	p("| action | type | status | verdict | scope | can affect legit? | reversible | evidence |\n")
	p("|---|---|---|---|---|---|---|---|\n")
	for _, a := range inc.Actions {
		p("| %s | %s | %s | %s | %s=%s | %v | %v (%s) | %d ref(s) |\n",
			short(a.ID), a.Type, a.Status, a.Verdict, a.Blast.ScopeKind, a.Blast.ScopeID,
			a.Blast.CanAffectLegit, a.Blast.Reversible, a.Blast.ReversibleWhy, len(a.EvidenceIDs))
	}

	p("\n## Human-control gates\n\n")
	p("- every consequential action reversible: **%v**\n", inc.AllConsequentialReversible())
	p("- every state-changing action cites evidence: **%v**\n", inc.AllConsequentialHaveEvidence())

	p("\n## Causal caveats\n\n")
	for _, c := range inc.Caveats {
		p("- %s\n", c)
	}
	return nil
}

func optF(f float64) string {
	if f == 0 {
		return "—"
	}
	return fmt.Sprintf("%.2f", f)
}

func dash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func short(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	return s
}
