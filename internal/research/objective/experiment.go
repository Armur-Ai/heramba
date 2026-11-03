package objective

import (
	"math/rand"

	"github.com/Armur-Ai/heramba/internal/research/stats"
)

// SchemaVersion versions the objective-experiment report.
const SchemaVersion = 1

// actionSeconds maps one attacker action onto wall-clock for latency reporting.
const actionSeconds = 2.0

// Arm is one defensive configuration. It is a transparent, parameterized model
// of a defense — NOT the live engine — so H2 can be probed deterministically.
type Arm struct {
	Name              string  `json:"name"`
	Depth             int     `json:"depth"`               // real steps to the objective
	Decoys            int     `json:"decoys"`              // initial decoys polluting the surface
	LureConsistency   float64 `json:"lure_consistency"`    // believability [0,1]
	DetectOnDecoy     bool    `json:"detect_on_decoy"`     // touching a decoy triggers detection+containment
	AdaptiveGrowth    int     `json:"adaptive_growth"`     // decoys added per contact (0 = static)
	RateLimit         int     `json:"rate_limit"`          // actions before throttle/containment (0 = none)
	BenignRateLimited bool    `json:"benign_rate_limited"` // whether the arm's rate limit disrupts benign traffic
}

// Attacker is the modeled adversary. A deception-aware attacker rejects lures
// whose consistency falls below AwareThreshold (it "checks for inconsistencies
// and rejects obvious lures"), which is exactly what the consistency validator
// defends against.
type Attacker struct {
	Budget         int     `json:"budget"`
	DeceptionAware bool    `json:"deception_aware"`
	AwareThreshold float64 `json:"aware_threshold"`
}

// FiveArms returns the five arms in directive order with matched depth/budget.
func FiveArms(depth, decoys, rateLimit, growth int) []Arm {
	return []Arm{
		{Name: "unprotected", Depth: depth},
		{Name: "conventional", Depth: depth, RateLimit: rateLimit, BenignRateLimited: true},
		{Name: "static_decoys", Depth: depth, Decoys: decoys, LureConsistency: 0.30, DetectOnDecoy: true},
		{Name: "stateful_deception", Depth: depth, Decoys: 2 * decoys, LureConsistency: 0.85, DetectOnDecoy: true},
		{Name: "adaptive_deception", Depth: depth, Decoys: 2 * decoys, LureConsistency: 0.90, DetectOnDecoy: true, AdaptiveGrowth: growth},
	}
}

// Trial is the measured outcome of one seeded attacker run against one arm.
type Trial struct {
	Completed       bool
	TimeToObjective float64 // seconds; only meaningful if Completed
	DivertedFrac    float64
	Retries         int
	Detected        bool
	DetectionLatSec float64
	LuresSeen       int
	LuresDiscovered int
}

// RunTrial simulates one attacker run against one arm. Deterministic given seed.
// The attacker makes progress toward the objective but the surface is polluted
// with decoys; a decoy contact diverts (and, if DetectOnDecoy, contains, forcing
// a retry), unless the attacker is deception-aware and the lure is below its
// consistency threshold — in which case it recognizes and skips the lure.
func RunTrial(arm Arm, atk Attacker, seed int64) Trial {
	rng := rand.New(rand.NewSource(seed))
	realRemaining := arm.Depth
	activeDecoys := arm.Decoys
	var t Trial
	var actionsUsed int
	contained := false

	for a := 1; a <= atk.Budget; a++ {
		actionsUsed = a
		if contained {
			// Attacker spins up a fresh source and retries from where the surface
			// still blocks it; the defender already knows (detection persists).
			t.Retries++
			contained = false
		}
		// Rate-limit containment (conventional): throttled once it has made too
		// many requests without reaching the objective.
		if arm.RateLimit > 0 && a >= arm.RateLimit && realRemaining > 0 {
			if !t.Detected {
				t.Detected = true
				t.DetectionLatSec = float64(a) * actionSeconds
			}
			break // throttled out before completing
		}

		total := realRemaining + activeDecoys
		if total <= 0 {
			break
		}
		hitDecoy := activeDecoys > 0 && rng.Float64() < float64(activeDecoys)/float64(total)
		if hitDecoy {
			t.LuresSeen++
			if atk.DeceptionAware && arm.LureConsistency < atk.AwareThreshold {
				// Recognizes the inconsistency and skips the lure (not diverted,
				// not trapped). This is the deception-aware adversary.
				t.LuresDiscovered++
				continue
			}
			// Diverted by a believable lure.
			if arm.DetectOnDecoy {
				if !t.Detected {
					t.Detected = true
					t.DetectionLatSec = float64(a) * actionSeconds
				}
				contained = true
				if arm.AdaptiveGrowth > 0 {
					activeDecoys += arm.AdaptiveGrowth // targeted lures grow after contact
				}
			}
			continue
		}
		// Productive step toward the objective.
		if realRemaining > 0 {
			realRemaining--
		}
		if realRemaining == 0 {
			t.Completed = true
			t.TimeToObjective = float64(a) * actionSeconds
			break
		}
	}

	if actionsUsed > 0 {
		t.DivertedFrac = float64(t.LuresSeen-t.LuresDiscovered) / float64(actionsUsed)
	}
	return t
}

// ArmResult aggregates an arm's trials with uncertainty.
type ArmResult struct {
	Arm                 string         `json:"arm"`
	Trials              int            `json:"trials"`
	ObjectiveCompletion stats.Interval `json:"objective_completion_rate"` // Wilson
	TimeToObjectiveSec  stats.Interval `json:"time_to_objective_sec"`     // bootstrap, completed only
	DivertedFraction    stats.Interval `json:"diverted_action_fraction"`  // bootstrap
	MeanRetries         stats.Interval `json:"mean_retries"`              // bootstrap
	DetectionLatencySec stats.Interval `json:"detection_latency_sec"`     // bootstrap, detected only
	DeceptionDiscovery  stats.Interval `json:"deception_discovery_rate"`  // Wilson over lures
	BenignDisruption    float64        `json:"benign_disruption"`         // modeled, see below
}
