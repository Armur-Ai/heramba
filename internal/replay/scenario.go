// Package replay is Heramba's research plane: versioned, seeded scenarios that
// replay deterministically so four defensive baselines can be compared on the
// same traffic under identical conditions, with real measured metrics and
// honest uncertainty.
//
// The four baselines, exactly as the engineering mandate specifies:
//
//	(1) no specialized defense
//	(2) static rules
//	(3) fixed Heramba deception
//	(4) adaptive Heramba deception
//
// Nothing here presumes the adaptive baseline wins — it is measured. Metrics
// the harness cannot honestly produce (CPU/memory under load) are marked
// unavailable rather than invented.
package replay

import (
	"math/rand"
	"sort"
	"time"
)

// SchemaVersion versions the scenario format for reproducibility manifests.
const SchemaVersion = 1

// Action is one timestamped request in a scenario. Labels are ground truth used
// only for scoring — the defenses never see them.
type Action struct {
	Offset     time.Duration `json:"offset"` // from scenario start
	Source     string        `json:"source"` // transport source key
	Path       string        `json:"path"`
	IsAttacker bool          `json:"is_attacker"` // ground-truth: from an attacker source
	IsDecoy    bool          `json:"is_decoy"`    // path is a decoy (zero-FP on a hit)
	IsGoal     bool          `json:"is_goal"`     // reaching this = attacker objective completed
}

// Scenario is a reproducible labeled traffic trace plus the config snapshot it
// was generated under.
type Scenario struct {
	Name            string            `json:"name"`
	SchemaVersion   int               `json:"schema_version"`
	Seed            int64             `json:"seed"`
	Actions         []Action          `json:"actions"`
	ConfigSnapshot  map[string]string `json:"config_snapshot"`
	AttackerSources []string          `json:"attacker_sources"`
	BenignSources   []string          `json:"benign_sources"`
}

var benignPaths = []string{"/", "/home", "/pricing", "/api/items", "/health", "/login", "/dashboard"}
var reconPaths = []string{"/admin", "/api/users/1", "/api/users/2", "/config", "/backup", "/api/keys"}
var decoyPaths = []string{"/.env", "/.git/config", "/api/internal/debug", "/wp-admin"}

const goalPath = "/api/admin/export-all"

// GenerateScenario builds a deterministic, labeled scenario from a seed: a set
// of benign users browsing, plus attacker sources that recon, trip a decoy, and
// finally attempt the objective. Same seed → identical scenario (verified in
// tests), which is what makes the four-baseline comparison reproducible.
func GenerateScenario(seed int64) Scenario {
	rng := rand.New(rand.NewSource(seed))
	s := Scenario{
		Name:          "mixed-recon-and-objective",
		SchemaVersion: SchemaVersion,
		Seed:          seed,
		ConfigSnapshot: map[string]string{
			"mode":      "protect",
			"decoys":    "enabled",
			"generator": "replay.GenerateScenario",
			"schema":    "1",
		},
	}
	nBenign := 4 + rng.Intn(3)
	nAttacker := 2 + rng.Intn(2)

	type ev struct {
		off  time.Duration
		src  string
		path string
		atk  bool
		dec  bool
		goal bool
	}
	var evs []ev

	// Benign users: steady browsing of benign paths over 10 minutes.
	for i := 0; i < nBenign; i++ {
		src := benignIP(i)
		s.BenignSources = append(s.BenignSources, src)
		for j := 0; j < 8; j++ {
			evs = append(evs, ev{
				off:  time.Duration(rng.Intn(600)) * time.Second,
				src:  src,
				path: benignPaths[rng.Intn(len(benignPaths))],
			})
		}
	}

	// Attacker sources: recon → decoy trip → objective attempt, clustered in a
	// short synchronized window (coordinated).
	base := time.Duration(120+rng.Intn(60)) * time.Second
	for i := 0; i < nAttacker; i++ {
		src := attackerIP(i)
		s.AttackerSources = append(s.AttackerSources, src)
		off := base + time.Duration(i*5)*time.Second
		// recon
		for _, p := range reconPaths {
			evs = append(evs, ev{off: off, src: src, path: p, atk: true})
			off += time.Duration(2+rng.Intn(3)) * time.Second
		}
		// trip a decoy
		evs = append(evs, ev{off: off, src: src, path: decoyPaths[rng.Intn(len(decoyPaths))], atk: true, dec: true})
		off += 3 * time.Second
		// attempt the objective
		evs = append(evs, ev{off: off, src: src, path: goalPath, atk: true, goal: true})
	}

	// stable chronological order
	sort.SliceStable(evs, func(i, j int) bool { return evs[i].off < evs[j].off })
	for _, e := range evs {
		s.Actions = append(s.Actions, Action{
			Offset: e.off, Source: e.src, Path: e.path,
			IsAttacker: e.atk, IsDecoy: e.dec, IsGoal: e.goal,
		})
	}
	return s
}

func benignIP(i int) string   { return "203.0.113." + itoa(10+i) }
func attackerIP(i int) string { return "192.0.2." + itoa(20+i) }

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [12]byte
	i := len(b)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
