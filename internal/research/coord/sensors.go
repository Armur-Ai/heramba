package coord

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/Armur-Ai/heramba/internal/campaign/swarm"
	"github.com/Armur-Ai/heramba/internal/research"
	"github.com/Armur-Ai/heramba/internal/research/stats"
)

// This file implements the H4 missing-sensor sweep: it removes one detection
// signal at a time from the full coordinated predicate and measures recall (on
// held-out coordinated cohorts) and FPR (on the negative pool), to answer H4 —
// does detection degrade *gracefully* rather than *catastrophically* when a
// sensor is missing? The answer is measured, not asserted, and is honest about
// which sensor is load-bearing.

// gatedScore is the full coordinated predicate (with the enumeration gate),
// parameterized so individual signals can be switched off to model a missing
// sensor. With all signals on it is identical to coordinatedScore.
func gatedScore(f swarm.Features, syncWindow time.Duration, sig signalSet) float64 {
	d := swarm.DefaultDetector()
	if f.Sources <= 1 {
		return 0.1
	}
	synced := sig.sync && f.StartSpread <= syncWindow
	shared := sig.shared && f.TargetJaccard >= d.OverlapThreshold
	divided := sig.divided && f.Complementarity >= d.ComplementThreshold
	enumerating := f.DistinctTargets >= d.EnumerationThreshold
	decoy := sig.decoy && f.DeceptionHits > 0
	coordinated := synced && (divided || (shared && (enumerating || decoy)))

	mag := 0.0
	if synced {
		mag += 0.4
	}
	if shared {
		mag += 0.3
	}
	if divided {
		mag += 0.3
	}
	if decoy {
		mag += 0.1
	}
	if mag > 1 {
		mag = 1
	}
	if coordinated {
		return clamp01(0.5 + mag/2)
	}
	return clamp01(0.4 * mag)
}

// SensorResult is the measured degradation when one sensor is removed.
type SensorResult struct {
	MissingSensor string         `json:"missing_sensor"`
	Recall        stats.Interval `json:"recall"`
	FPR           stats.Interval `json:"fpr"`
	Graceful      bool           `json:"graceful"` // recall stays strictly above the request-only floor
	LoadBearing   bool           `json:"load_bearing"`
}

// H4Report is the full missing-sensor sweep.
type H4Report struct {
	SchemaVersion int            `json:"schema_version"`
	CodeRevision  string         `json:"code_revision"`
	FullRecall    stats.Interval `json:"full_recall"`
	FloorRecall   stats.Interval `json:"request_only_recall"` // the baseline to beat
	Sensors       []SensorResult `json:"sensors"`
	Verdict       string         `json:"verdict"`
	Notes         []string       `json:"not_measured"`
}

var h4Sensors = []struct {
	name string
	sig  signalSet
}{
	{"timing (synchrony)", signalSet{sync: false, shared: true, divided: true, decoy: true}},
	{"target-overlap (shared)", signalSet{sync: true, shared: false, divided: true, decoy: true}},
	{"graph (division of labor)", signalSet{sync: true, shared: true, divided: false, decoy: true}},
	{"deception (decoy contact)", signalSet{sync: true, shared: true, divided: true, decoy: false}},
}

// RunH4 sweeps single-sensor removals over the held-out families + benign
// controls. Deterministic given the protocol.
func RunH4(p research.Protocol, params Params, codeRevision string) H4Report {
	if params.MatchedRequests == 0 {
		params = DefaultParams()
	}
	families := p.TestFamilies()
	sw := swarm.DefaultDetector().SyncWindow

	// Materialize labeled cohorts once (held-out families + benign controls).
	type lc struct {
		f     swarm.Features
		coord bool
	}
	var cohorts []lc
	det := swarm.DefaultDetector()
	for _, fam := range families {
		actor, ok := p.ActorByID(fam.ActorModel)
		if !ok {
			continue
		}
		for _, seed := range fam.Seeds {
			c := GenerateCohort(fam, actor, seed, params)
			cohorts = append(cohorts, lc{det.Extract(c.Sessions), c.Coordinated})
		}
	}
	for i := 0; i < params.BenignControls; i++ {
		c := GenerateBenignCohort(int64(7000+i), params)
		cohorts = append(cohorts, lc{det.Extract(c.Sessions), false})
	}

	recallFPR := func(score func(swarm.Features) float64) (stats.Interval, stats.Interval) {
		var tp, fn, fp, tn int
		for _, c := range cohorts {
			pred := score(c.f) >= OperatingThreshold
			switch {
			case pred && c.coord:
				tp++
			case !pred && c.coord:
				fn++
			case pred && !c.coord:
				fp++
			default:
				tn++
			}
		}
		return stats.Wilson95(tp, tp+fn), stats.Wilson95(fp, fp+tn)
	}

	rep := H4Report{SchemaVersion: 1, CodeRevision: codeRevision}
	full := func(f swarm.Features) float64 { return gatedScore(f, sw, fullSignals) }
	rep.FullRecall, _ = recallFPR(full)
	reqOnly := RequestOnly()
	rep.FloorRecall, _ = recallFPR(func(f swarm.Features) float64 { return reqOnly.Score(f, sw) })

	allGraceful := true
	for _, s := range h4Sensors {
		sig := s.sig
		rec, fpr := recallFPR(func(f swarm.Features) float64 { return gatedScore(f, sw, sig) })
		graceful := rec.Point > rep.FloorRecall.Point
		loadBearing := rec.Point <= rep.FloorRecall.Point
		if loadBearing {
			allGraceful = false
		}
		rep.Sensors = append(rep.Sensors, SensorResult{
			MissingSensor: s.name, Recall: rec, FPR: fpr, Graceful: graceful, LoadBearing: loadBearing,
		})
	}
	if allGraceful {
		rep.Verdict = "GRACEFUL: no single missing sensor collapses recall to the request-only floor (H4 null rejected)."
	} else {
		rep.Verdict = "PARTIAL: most sensors degrade gracefully, but at least one is load-bearing (removing it collapses recall) — H4 null holds for that sensor."
	}
	rep.Notes = []string{
		"a 'missing sensor' is modeled as a removed detection signal; recall/FPR are on the held-out set",
		"the floor is the request-only recall — the null hypothesis is that any removal drops to this floor",
	}
	return rep
}

// WriteJSON emits the machine-readable H4 sweep.
func (r H4Report) WriteJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

// WriteMarkdown emits the human-readable H4 sweep.
func (r H4Report) WriteMarkdown(w io.Writer) error {
	p := func(format string, a ...any) { fmt.Fprintf(w, format, a...) }
	p("# H4 — missing-sensor degradation sweep\n\n")
	p("- code revision: `%s`\n", r.CodeRevision)
	p("- full-detector recall: %.3f [%.3f, %.3f]\n", r.FullRecall.Point, r.FullRecall.Lo, r.FullRecall.Hi)
	p("- request-only floor (null): %.3f [%.3f, %.3f]\n\n", r.FloorRecall.Point, r.FloorRecall.Lo, r.FloorRecall.Hi)
	p("| missing sensor | recall [95%% CI] | FPR [95%% CI] | degradation |\n")
	p("|---|---|---|---|\n")
	for _, s := range r.Sensors {
		deg := "graceful"
		if s.LoadBearing {
			deg = "**load-bearing (recall collapses)**"
		}
		p("| %s | %.3f [%.3f, %.3f] | %.3f [%.3f, %.3f] | %s |\n",
			s.MissingSensor, s.Recall.Point, s.Recall.Lo, s.Recall.Hi,
			s.FPR.Point, s.FPR.Lo, s.FPR.Hi, deg)
	}
	p("\n**Verdict:** %s\n\n", r.Verdict)
	for _, n := range r.Notes {
		p("- %s\n", n)
	}
	return nil
}
