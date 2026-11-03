package objective

import (
	"math/rand"

	"github.com/Armur-Ai/heramba/internal/research/stats"
)

// Config parameterizes the whole experiment. All arms share the same depth and
// attacker budget so the comparison is matched.
type Config struct {
	Depth        int
	Decoys       int
	RateLimit    int
	Growth       int
	Budget       int
	Seeds        []int64
	Attacker     Attacker
	CodeRevision string
}

// DefaultConfig returns a matched, calibrated configuration.
func DefaultConfig(seeds []int64) Config {
	return Config{
		Depth:     8,
		Decoys:    12,
		RateLimit: 20,
		Growth:    6,
		Budget:    60,
		Seeds:     seeds,
		Attacker:  Attacker{Budget: 60, DeceptionAware: false, AwareThreshold: 0.6},
	}
}

// Report is the complete, reproducible objective-experiment artifact.
type Report struct {
	SchemaVersion int         `json:"schema_version"`
	CodeRevision  string      `json:"code_revision"`
	Config        Config      `json:"config"`
	Arms          []ArmResult `json:"arms"`
	NotMeasured   []string    `json:"not_measured"`
}

// Run executes every arm over the seeds and aggregates with uncertainty. The
// attacker's budget is taken from Config.Budget (overriding the embedded value)
// so the two cannot drift apart.
func Run(cfg Config) Report {
	atk := cfg.Attacker
	atk.Budget = cfg.Budget
	arms := FiveArms(cfg.Depth, cfg.Decoys, cfg.RateLimit, cfg.Growth)

	rep := Report{
		SchemaVersion: SchemaVersion,
		CodeRevision:  cfg.CodeRevision,
		Config:        cfg,
		NotMeasured: []string{
			"cpu_time / memory_rss: not measured; this is a deterministic behavioral model, not a load test",
			"this arm model is a transparent abstraction of the defenses, not the live engine (see R6 for proxy overhead)",
		},
	}

	for _, arm := range arms {
		var completed []float64
		var ttos []float64
		var diverted []float64
		var retries []float64
		var detLat []float64
		luresSeen, luresDisc := 0, 0
		succ := 0

		for _, seed := range cfg.Seeds {
			t := RunTrial(arm, atk, seed)
			if t.Completed {
				completed = append(completed, 1)
				succ++
				ttos = append(ttos, t.TimeToObjective)
			} else {
				completed = append(completed, 0)
			}
			diverted = append(diverted, t.DivertedFrac)
			retries = append(retries, float64(t.Retries))
			if t.Detected {
				detLat = append(detLat, t.DetectionLatSec)
			}
			luresSeen += t.LuresSeen
			luresDisc += t.LuresDiscovered
		}

		n := len(cfg.Seeds)
		rep.Arms = append(rep.Arms, ArmResult{
			Arm:                 arm.Name,
			Trials:              n,
			ObjectiveCompletion: stats.Wilson95(succ, n),
			TimeToObjectiveSec:  stats.Bootstrap(ttos, 3000, 11),
			DivertedFraction:    stats.Bootstrap(diverted, 3000, 12),
			MeanRetries:         stats.Bootstrap(retries, 3000, 13),
			DetectionLatencySec: stats.Bootstrap(detLat, 3000, 14),
			DeceptionDiscovery:  stats.Wilson95(luresDisc, luresSeen),
			BenignDisruption:    benignDisruption(arm, 99),
		})
	}
	return rep
}

// benignDisruption models how much legitimate traffic an arm wrongly blocks.
// Decoy-based arms disrupt nothing — benign clients never touch a decoy, the
// zero-false-positive property — while a blunt rate limit (conventional) throttles
// heavy-but-legitimate clients. Deterministic given seed.
func benignDisruption(arm Arm, seed int64) float64 {
	if !arm.BenignRateLimited || arm.RateLimit <= 0 {
		return 0 // deception never disrupts benign traffic
	}
	rng := rand.New(rand.NewSource(seed))
	const clients = 200
	var total, blocked int
	for i := 0; i < clients; i++ {
		// Most benign clients are light; a heavy tail of legitimate bulk clients
		// exceeds the rate limit and gets throttled.
		reqs := 3 + rng.Intn(10)
		if rng.Float64() < 0.1 {
			reqs = arm.RateLimit + 1 + rng.Intn(arm.RateLimit) // legitimate bulk user
		}
		total += reqs
		if reqs > arm.RateLimit {
			blocked += reqs - arm.RateLimit
		}
	}
	if total == 0 {
		return 0
	}
	return float64(blocked) / float64(total)
}
