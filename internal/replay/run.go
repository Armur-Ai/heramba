package replay

import (
	"math"
	"time"
)

// Metrics are the measured outcomes of running one scenario through one
// defense. Fields the harness cannot honestly measure are omitted and called
// out in the report as "not measured" rather than fabricated.
type Metrics struct {
	Defense               string        `json:"defense"`
	DetectionLatency      time.Duration `json:"detection_latency"` // first attacker action → first true detection
	DetectedAttacker      bool          `json:"detected_attacker"`
	AttackerObjectiveDone bool          `json:"attacker_objective_done"` // any attacker reached the goal uncontained
	TruePositiveSources   int           `json:"true_positive_sources"`   // attacker sources contained
	FalsePositiveSources  int           `json:"false_positive_sources"`  // benign sources contained
	AttackerSources       int           `json:"attacker_sources"`
	BenignSources         int           `json:"benign_sources"`
	LegitActionsBlocked   int           `json:"legit_actions_blocked"` // benign requests denied (disruption)
	BenignActionsTotal    int           `json:"benign_actions_total"`
}

// TPR is the true-positive rate over sources.
func (m Metrics) TPR() float64 {
	if m.AttackerSources == 0 {
		return 0
	}
	return float64(m.TruePositiveSources) / float64(m.AttackerSources)
}

// FPR is the false-positive rate over benign sources.
func (m Metrics) FPR() float64 {
	if m.BenignSources == 0 {
		return 0
	}
	return float64(m.FalsePositiveSources) / float64(m.BenignSources)
}

// LegitDisruption is the fraction of benign requests wrongly blocked.
func (m Metrics) LegitDisruption() float64 {
	if m.BenignActionsTotal == 0 {
		return 0
	}
	return float64(m.LegitActionsBlocked) / float64(m.BenignActionsTotal)
}

// RunScenario replays one scenario through one defense and measures outcomes.
// A contained source has all its subsequent actions blocked; if a benign source
// is contained, its blocked actions count as legitimate disruption; if an
// attacker reaches the goal before being contained, the objective completes.
func RunScenario(s Scenario, d Defense) Metrics {
	d.Reset()
	m := Metrics{
		Defense:         d.Name(),
		AttackerSources: len(s.AttackerSources),
		BenignSources:   len(s.BenignSources),
	}
	contained := map[string]bool{}
	detectedSrc := map[string]bool{}

	var firstAttackerAt time.Duration = -1
	var firstDetectionAt time.Duration = -1

	for _, a := range s.Actions {
		if a.IsAttacker && firstAttackerAt < 0 {
			firstAttackerAt = a.Offset
		}
		if !a.IsAttacker {
			m.BenignActionsTotal++
		}
		// A contained source's request is blocked before the defense sees it.
		if contained[a.Source] {
			if !a.IsAttacker {
				m.LegitActionsBlocked++
			}
			continue
		}

		r := d.Observe(a)
		if r.Detected && a.IsAttacker && !detectedSrc[a.Source] {
			detectedSrc[a.Source] = true
			if firstDetectionAt < 0 {
				firstDetectionAt = a.Offset
			}
			m.DetectedAttacker = true
		}
		if r.Contain {
			contained[a.Source] = true
		}
		// Objective completion: an attacker reached the goal without being
		// contained in this same step.
		if a.IsGoal && a.IsAttacker && !contained[a.Source] {
			m.AttackerObjectiveDone = true
		}
	}

	// Tally contained sources by ground truth.
	for src := range contained {
		if isIn(s.AttackerSources, src) {
			m.TruePositiveSources++
		} else if isIn(s.BenignSources, src) {
			m.FalsePositiveSources++
		}
	}
	if firstDetectionAt >= 0 && firstAttackerAt >= 0 {
		m.DetectionLatency = firstDetectionAt - firstAttackerAt
		if m.DetectionLatency < 0 {
			m.DetectionLatency = 0
		}
	}
	return m
}

// Aggregate is the mean ± sample stddev of a metric across trials.
type Aggregate struct {
	Mean   float64 `json:"mean"`
	StdDev float64 `json:"stddev"`
	N      int     `json:"n"`
}

func aggregate(xs []float64) Aggregate {
	n := len(xs)
	if n == 0 {
		return Aggregate{}
	}
	var sum float64
	for _, x := range xs {
		sum += x
	}
	mean := sum / float64(n)
	var ss float64
	for _, x := range xs {
		ss += (x - mean) * (x - mean)
	}
	sd := 0.0
	if n > 1 {
		sd = math.Sqrt(ss / float64(n-1))
	}
	return Aggregate{Mean: mean, StdDev: sd, N: n}
}

// BaselineResult aggregates one defense's metrics across repeated trials.
type BaselineResult struct {
	Defense             string    `json:"defense"`
	Trials              int       `json:"trials"`
	TPR                 Aggregate `json:"tpr"`
	FPR                 Aggregate `json:"fpr"`
	ObjectiveCompletion Aggregate `json:"objective_completion_rate"`
	DetectionLatencySec Aggregate `json:"detection_latency_sec"`
	LegitDisruption     Aggregate `json:"legit_disruption"`
}

// Experiment is the full four-baseline comparison.
type Experiment struct {
	SchemaVersion int              `json:"schema_version"`
	Seeds         []int64          `json:"seeds"`
	Results       []BaselineResult `json:"results"`
	// NotMeasured documents metrics intentionally not produced by this harness.
	NotMeasured []string `json:"not_measured"`
}

// AllDefenses returns the four baselines in mandate order.
func AllDefenses() []Defense {
	return []Defense{NoDefense{}, NewStaticRules(), FixedDeception{}, NewAdaptiveDeception()}
}

// RunFourBaselineExperiment runs every baseline over the same set of seeded
// scenarios (identical conditions) and aggregates with uncertainty. This is the
// single reproducible entry point the CLI `heramba experiment` command calls.
func RunFourBaselineExperiment(seeds []int64) Experiment {
	exp := Experiment{
		SchemaVersion: SchemaVersion,
		Seeds:         append([]int64(nil), seeds...),
		NotMeasured: []string{
			"cpu_time: not measured in this deterministic harness",
			"memory_rss: not measured in this deterministic harness",
			"recovery_time: containment is logical here; wall-clock recovery needs a live deployment",
		},
	}
	scenarios := make([]Scenario, len(seeds))
	for i, sd := range seeds {
		scenarios[i] = GenerateScenario(sd)
	}
	for _, d := range AllDefenses() {
		var tpr, fpr, obj, lat, disrupt []float64
		for _, sc := range scenarios {
			m := RunScenario(sc, d)
			tpr = append(tpr, m.TPR())
			fpr = append(fpr, m.FPR())
			if m.AttackerObjectiveDone {
				obj = append(obj, 1)
			} else {
				obj = append(obj, 0)
			}
			lat = append(lat, m.DetectionLatency.Seconds())
			disrupt = append(disrupt, m.LegitDisruption())
		}
		exp.Results = append(exp.Results, BaselineResult{
			Defense:             d.Name(),
			Trials:              len(scenarios),
			TPR:                 aggregate(tpr),
			FPR:                 aggregate(fpr),
			ObjectiveCompletion: aggregate(obj),
			DetectionLatencySec: aggregate(lat),
			LegitDisruption:     aggregate(disrupt),
		})
	}
	return exp
}

func isIn(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}
