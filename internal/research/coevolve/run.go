package coevolve

import (
	"github.com/Armur-Ai/heramba/internal/research/stats"
)

// SchemaVersion versions the co-evolution report.
const SchemaVersion = 1

// ArchetypeResult aggregates one archetype's rounds under one defender mode.
type ArchetypeResult struct {
	Archetype          string         `json:"archetype"`
	Mode               string         `json:"defender_mode"`
	Rounds             int            `json:"rounds"`
	CounteredRate      stats.Interval `json:"countered_rate"`       // Wilson
	AdaptationLatency  stats.Interval `json:"adaptation_latency_s"` // bootstrap (countered rounds)
	DetectionComponent stats.Interval `json:"detection_component_s"`
	ResponseComponent  stats.Interval `json:"response_component_s"`
	MeanUtility        stats.Interval `json:"mean_utility"`     // bootstrap
	UtilityStdDev      float64        `json:"utility_stddev"`   // adaptation stability
	OscillationRate    stats.Interval `json:"oscillation_rate"` // Wilson
}

// Report is the complete co-evolution artifact.
type Report struct {
	SchemaVersion int               `json:"schema_version"`
	CodeRevision  string            `json:"code_revision"`
	Params        Params            `json:"params"`
	Rounds        int               `json:"rounds_per_race"`
	Seeds         []int64           `json:"seeds"`
	Results       []ArchetypeResult `json:"results"`
	NotMeasured   []string          `json:"not_measured"`
}

func modeName(m DefenderMode) string {
	switch m {
	case NoAdaptation:
		return "no_adaptation"
	case Instant:
		return "instant"
	default:
		return "adaptive"
	}
}

// Run races every archetype against every defender mode over the seeds and
// aggregates with uncertainty. Deterministic given the seeds.
func Run(rounds int, seeds []int64, p Params, codeRevision string) Report {
	rep := Report{
		SchemaVersion: SchemaVersion,
		CodeRevision:  codeRevision,
		Params:        p,
		Rounds:        rounds,
		Seeds:         append([]int64(nil), seeds...),
		NotMeasured: []string{
			"wall-clock CPU: ticks are logical; TickSeconds maps them for reporting only",
			"a non-seeded deployment adds observational nondeterminism; here jitter is seeded and bounded",
		},
	}
	modes := []DefenderMode{Adaptive, NoAdaptation, Instant}
	for _, arch := range Archetypes() {
		for _, mode := range modes {
			var countered, osc []float64
			var lat, detC, respC, util []float64
			total := 0
			for _, seed := range seeds {
				s := &Sim{Archetype: arch, Mode: mode, Params: p}
				rrs := s.Run(rounds, seed)
				for _, r := range rrs {
					total++
					if r.Countered {
						countered = append(countered, 1)
					} else {
						countered = append(countered, 0)
					}
					if r.Oscillated {
						osc = append(osc, 1)
					} else {
						osc = append(osc, 0)
					}
					if r.AdaptationLatency >= 0 {
						lat = append(lat, float64(r.AdaptationLatency)*TickSeconds)
						detC = append(detC, float64(r.DetectionComp)*TickSeconds)
						respC = append(respC, float64(r.ResponseComp)*TickSeconds)
					}
					util = append(util, r.Utility)
				}
			}
			_, sd := stats.MeanStd(util)
			rep.Results = append(rep.Results, ArchetypeResult{
				Archetype:          arch.Name,
				Mode:               modeName(mode),
				Rounds:             total,
				CounteredRate:      wilsonFrac(countered),
				AdaptationLatency:  stats.Bootstrap(lat, 3000, 21),
				DetectionComponent: stats.Bootstrap(detC, 3000, 22),
				ResponseComponent:  stats.Bootstrap(respC, 3000, 23),
				MeanUtility:        stats.Bootstrap(util, 3000, 24),
				UtilityStdDev:      sd,
				OscillationRate:    wilsonFrac(osc),
			})
		}
	}
	return rep
}

func wilsonFrac(xs []float64) stats.Interval {
	succ := 0
	for _, x := range xs {
		if x >= 0.5 {
			succ++
		}
	}
	return stats.Wilson95(succ, len(xs))
}
