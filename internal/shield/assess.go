package shield

import "sort"

// This file rolls a set of per-tool Results into one defensive verdict and
// decides, under an explicit policy, whether a run should fail (so `heramba
// shield` can gate CI). Coverage is first-class: a tool that isn't installed is
// an honest gap that can itself fail a strict policy — never a silent pass.

// Coverage summarises how much of the suite actually ran.
type Coverage struct {
	Total       int `json:"total"`
	Ran         int `json:"ran"`         // clean or findings
	Unavailable int `json:"unavailable"` // not installed / not yet shipped
	Errored     int `json:"errored"`     // ran but failed
}

// Assessment is the aggregated defensive verdict over all tools.
type Assessment struct {
	Overall       Status   `json:"overall"` // findings|error|clean|unavailable
	TotalFindings int      `json:"total_findings"`
	Coverage      Coverage `json:"coverage"`
	Gaps          []string `json:"gaps,omitempty"`   // tools that could not run (unavailable)
	Errors        []string `json:"errors,omitempty"` // tools that errored
}

// Assess aggregates per-tool results into one verdict. Overall is the single
// worst outcome observed: any findings → "findings"; else any error → "error";
// else if nothing ran → "unavailable"; else "clean".
func Assess(results []Result) Assessment {
	a := Assessment{Overall: StatusClean}
	a.Coverage.Total = len(results)
	anyFindings, anyError := false, false
	for _, r := range results {
		switch r.Status {
		case StatusFindings:
			a.Coverage.Ran++
			a.TotalFindings += r.Findings
			anyFindings = true
		case StatusClean:
			a.Coverage.Ran++
		case StatusUnavailable:
			a.Coverage.Unavailable++
			a.Gaps = append(a.Gaps, r.Tool)
		case StatusError:
			a.Coverage.Errored++
			a.Errors = append(a.Errors, r.Tool)
			anyError = true
		}
	}
	sort.Strings(a.Gaps)
	sort.Strings(a.Errors)

	switch {
	case anyFindings:
		a.Overall = StatusFindings
	case anyError:
		a.Overall = StatusError
	case a.Coverage.Ran == 0 && a.Coverage.Total > 0:
		a.Overall = StatusUnavailable
	default:
		a.Overall = StatusClean
	}
	return a
}

// FailPolicy is how strict `shield` is about what counts as a failure.
type FailPolicy string

const (
	// FailNever never fails the run (reporting only) — the default.
	FailNever FailPolicy = "none"
	// FailOnFindings fails when any tool found something.
	FailOnFindings FailPolicy = "findings"
	// FailOnError fails on findings OR a tool error.
	FailOnError FailPolicy = "error"
	// FailOnGap is the strictest: findings, errors, OR any missing coverage.
	FailOnGap FailPolicy = "gap"
)

// ParseFailPolicy validates a --fail-on value, returning ok=false for an
// unknown policy so the caller can report it rather than guess.
func ParseFailPolicy(s string) (FailPolicy, bool) {
	switch FailPolicy(s) {
	case FailNever, FailOnFindings, FailOnError, FailOnGap:
		return FailPolicy(s), true
	default:
		return "", false
	}
}

// ShouldFail reports whether the assessment fails under the given policy.
func (a Assessment) ShouldFail(p FailPolicy) bool {
	switch p {
	case FailOnFindings:
		return a.TotalFindings > 0
	case FailOnError:
		return a.TotalFindings > 0 || a.Coverage.Errored > 0
	case FailOnGap:
		return a.TotalFindings > 0 || a.Coverage.Errored > 0 || a.Coverage.Unavailable > 0
	default: // FailNever or unknown
		return false
	}
}
