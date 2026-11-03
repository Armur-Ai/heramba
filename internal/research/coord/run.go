package coord

import (
	"sort"
	"time"

	"github.com/Armur-Ai/heramba/internal/campaign"
	"github.com/Armur-Ai/heramba/internal/campaign/swarm"
	"github.com/Armur-Ai/heramba/internal/research"
	"github.com/Armur-Ai/heramba/internal/research/stats"
)

// SchemaVersion versions the coordination-experiment report format.
const SchemaVersion = 1

// MethodResult is one method's pooled performance across all evaluated cohorts.
type MethodResult struct {
	Method    string          `json:"method"`
	Threshold float64         `json:"threshold"`
	Confusion stats.Confusion `json:"confusion"`
	Recall    stats.Interval  `json:"recall"`    // Wilson 95%
	FPR       stats.Interval  `json:"fpr"`       // Wilson 95%
	Precision stats.Interval  `json:"precision"` // Wilson 95%
	PRAUC     float64         `json:"pr_auc"`
}

// FamilyConfusion is a per-family (stratified) confusion matrix for one method.
type FamilyConfusion struct {
	Family      string          `json:"family"`
	ActorKind   string          `json:"actor_kind"`
	Coordinated bool            `json:"coordinated_ground_truth"`
	Method      string          `json:"method"`
	Confusion   stats.Confusion `json:"confusion"`
}

// Report is the complete, reproducible coordination-experiment artifact.
type Report struct {
	SchemaVersion    int                `json:"schema_version"`
	CodeRevision     string             `json:"code_revision"`
	Params           Params             `json:"params"`
	Families         []string           `json:"families"`
	SeedsByFamily    map[string][]int64 `json:"seeds_by_family"`
	Cohorts          int                `json:"cohorts"`
	Positives        int                `json:"positive_cohorts"`
	Negatives        int                `json:"negative_cohorts"`
	Methods          []MethodResult     `json:"methods"`
	Ablations        []MethodResult     `json:"ablations"`
	PerFamily        []FamilyConfusion  `json:"per_family_confusion"`
	DetectionLatency stats.Interval     `json:"detection_latency_sec"` // FullHeramba, coordinated cohorts, session-granular
	ThroughputRPS    float64            `json:"throughput_classifications_per_sec"`
	NotMeasured      []string           `json:"not_measured"`
}

// Options configures a run.
type Options struct {
	Families     []research.ScenarioFamily // which families to evaluate (default: held-out test)
	Params       Params
	CodeRevision string
}

// scored is a per-cohort score with its ground-truth label and family.
type scored struct {
	family    string
	actorKind string
	coord     bool
	score     float64
}

// Run executes the fair coordination experiment: it generates every cohort,
// extracts the shipped swarm features once per cohort, scores it with every
// method and ablation, and aggregates pooled and per-family metrics with
// uncertainty. Deterministic given the protocol seeds and params (throughput is
// a timing measurement and will vary).
func Run(p research.Protocol, opt Options) Report {
	params := opt.Params
	if params.MatchedRequests == 0 {
		params = DefaultParams()
	}
	families := opt.Families
	if len(families) == 0 {
		families = p.TestFamilies()
	}
	det := swarm.DefaultDetector()
	syncWindow := det.SyncWindow

	rep := Report{
		SchemaVersion: SchemaVersion,
		CodeRevision:  opt.CodeRevision,
		Params:        params,
		SeedsByFamily: map[string][]int64{},
		NotMeasured: []string{
			"cpu_time / memory_rss: not measured by this deterministic harness",
			"calibration reliability curve: scores are heuristic, not probabilistic; PR-AUC is reported instead",
		},
	}

	methods := AllMethods()
	ablations := Ablations()

	// Pooled scores per method/ablation.
	pooled := map[string][]scored{}
	// Per-family confusion accumulation.
	type key struct{ fam, method string }
	famConf := map[key]*FamilyConfusion{}
	var latencies []float64

	for _, fam := range families {
		actor, ok := p.ActorByID(fam.ActorModel)
		if !ok {
			continue
		}
		rep.Families = append(rep.Families, fam.Name)
		rep.SeedsByFamily[fam.Name] = append([]int64(nil), fam.Seeds...)
		for _, seed := range fam.Seeds {
			c := GenerateCohort(fam, actor, seed, params)
			f := det.Extract(c.Sessions)
			rep.Cohorts++
			if c.Coordinated {
				rep.Positives++
			} else {
				rep.Negatives++
			}
			for _, m := range append(append([]Method{}, methods...), ablations...) {
				s := m.Score(f, syncWindow)
				pooled[m.Name] = append(pooled[m.Name], scored{fam.Name, c.ActorKind, c.Coordinated, s})
				k := key{fam.Name, m.Name}
				if famConf[k] == nil {
					famConf[k] = &FamilyConfusion{Family: fam.Name, ActorKind: c.ActorKind, Coordinated: c.Coordinated, Method: m.Name}
				}
				acc := famConf[k]
				pred := s >= OperatingThreshold
				switch {
				case pred && c.Coordinated:
					acc.Confusion.TP++
				case pred && !c.Coordinated:
					acc.Confusion.FP++
				case !pred && c.Coordinated:
					acc.Confusion.FN++
				default:
					acc.Confusion.TN++
				}
			}
			// Detection latency for the full detector on coordinated cohorts.
			if c.Coordinated {
				if lat, ok := detectionLatency(c.Sessions, det, syncWindow); ok {
					latencies = append(latencies, lat.Seconds())
				}
			}
		}
	}

	// Realistic benign-control negatives, rate-matched, added to the negative
	// pool so the FPR is measured against representative benign clients.
	if params.BenignControls > 0 {
		rep.Families = append(rep.Families, "benign-control")
		var benignSeeds []int64
		for i := 0; i < params.BenignControls; i++ {
			seed := int64(7000 + i)
			benignSeeds = append(benignSeeds, seed)
			c := GenerateBenignCohort(seed, params)
			f := det.Extract(c.Sessions)
			rep.Cohorts++
			rep.Negatives++
			for _, m := range append(append([]Method{}, methods...), ablations...) {
				s := m.Score(f, syncWindow)
				pooled[m.Name] = append(pooled[m.Name], scored{"benign-control", "benign", false, s})
				k := key{"benign-control", m.Name}
				if famConf[k] == nil {
					famConf[k] = &FamilyConfusion{Family: "benign-control", ActorKind: "benign", Coordinated: false, Method: m.Name}
				}
				acc := famConf[k]
				if s >= OperatingThreshold {
					acc.Confusion.FP++
				} else {
					acc.Confusion.TN++
				}
			}
		}
		rep.SeedsByFamily["benign-control"] = benignSeeds
	}

	rep.Methods = summarize(methods, pooled)
	rep.Ablations = summarize(ablations, pooled)

	// Stratified per-family confusion matrices, stable order.
	for _, fc := range famConf {
		rep.PerFamily = append(rep.PerFamily, *fc)
	}
	sort.Slice(rep.PerFamily, func(i, j int) bool {
		if rep.PerFamily[i].Method != rep.PerFamily[j].Method {
			return rep.PerFamily[i].Method < rep.PerFamily[j].Method
		}
		return rep.PerFamily[i].Family < rep.PerFamily[j].Family
	})

	rep.DetectionLatency = stats.Bootstrap(latencies, 5000, 1)
	rep.ThroughputRPS = measureThroughput(p, families, params, det, syncWindow)
	return rep
}

func summarize(methods []Method, pooled map[string][]scored) []MethodResult {
	var out []MethodResult
	for _, m := range methods {
		rows := pooled[m.Name]
		scores := make([]float64, len(rows))
		labels := make([]bool, len(rows))
		for i, r := range rows {
			scores[i] = r.score
			labels[i] = r.coord
		}
		c := stats.ConfusionAt(scores, labels, OperatingThreshold)
		out = append(out, MethodResult{
			Method:    m.Name,
			Threshold: OperatingThreshold,
			Confusion: c,
			Recall:    stats.Wilson95(c.TP, c.TP+c.FN),
			FPR:       stats.Wilson95(c.FP, c.FP+c.TN),
			Precision: stats.Wilson95(c.TP, c.TP+c.FP),
			PRAUC:     stats.PRAUC(scores, labels),
		})
	}
	return out
}

// detectionLatency replays a cohort session-by-session in start order and
// returns the delay from the first session's start to the first moment the full
// detector would flag coordination. This is a session-granular measurement, not
// a per-request one (documented as such in the report).
func detectionLatency(sessions []campaign.Session, det swarm.Detector, syncWindow time.Duration) (time.Duration, bool) {
	ordered := append([]campaign.Session(nil), sessions...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Start.Before(ordered[j].Start) })
	if len(ordered) == 0 {
		return 0, false
	}
	first := ordered[0].Start
	for i := 1; i <= len(ordered); i++ {
		f := det.Extract(ordered[:i])
		if fullScore(f, syncWindow, fullSignals) >= OperatingThreshold {
			return ordered[i-1].Start.Sub(first), true
		}
	}
	return 0, false
}

func measureThroughput(p research.Protocol, families []research.ScenarioFamily, params Params, det swarm.Detector, sw time.Duration) float64 {
	// Build a representative cohort set once, then time repeated classification.
	var cohorts [][]campaign.Session
	for _, fam := range families {
		actor, ok := p.ActorByID(fam.ActorModel)
		if !ok {
			continue
		}
		for _, seed := range fam.Seeds {
			cohorts = append(cohorts, GenerateCohort(fam, actor, seed, params).Sessions)
		}
	}
	if len(cohorts) == 0 {
		return 0
	}
	const reps = 50
	start := time.Now()
	n := 0
	for r := 0; r < reps; r++ {
		for _, cs := range cohorts {
			f := det.Extract(cs)
			_ = fullScore(f, sw, fullSignals)
			n++
		}
	}
	elapsed := time.Since(start).Seconds()
	if elapsed <= 0 {
		return 0
	}
	return float64(n) / elapsed
}
