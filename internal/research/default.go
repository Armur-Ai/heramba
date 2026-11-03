package research

import (
	"encoding/json"
	"io"
)

// Endpoint names referenced across the protocol and experiments. Using
// constants keeps the hypotheses, families, and experiment reporters from
// drifting apart (a typo would fail Validate rather than silently mislabel).
const (
	EndRecall            = "recall"
	EndFPR               = "false_positive_rate"
	EndPrecision         = "precision"
	EndPRAUC             = "pr_auc"
	EndDetectionLatency  = "detection_latency_sec"
	EndObjectiveComplete = "objective_completion_rate"
	EndObjectiveTime     = "time_to_objective_sec"
	EndDiversion         = "diverted_action_fraction"
	EndAdaptationLatency = "adaptation_latency_sec"
	EndDefenderUtility   = "defender_utility"
	EndBenignDisruption  = "benign_disruption"
	EndThroughput        = "throughput_rps"
	EndDeceptionDiscov   = "deception_discovery_rate"
)

// DefaultProtocol returns Heramba's pre-registered Phase 2 protocol. This is the
// single source of truth for H1-H5, the threat-model actor families, the
// endpoints, and the leakage-free data partitions. It is asserted valid in
// tests, so it cannot drift into an inconsistent state.
func DefaultProtocol() Protocol {
	p := Protocol{
		SchemaVersion: SchemaVersion,
		Title:         "Heramba Phase 2: coordinated autonomous attack detection & mitigation",
		Endpoints: []Endpoint{
			{Name: EndRecall, Unit: "fraction", Primary: true, Direction: "higher_is_better"},
			{Name: EndFPR, Unit: "fraction", Primary: true, Direction: "lower_is_better"},
			{Name: EndPrecision, Unit: "fraction", Primary: false, Direction: "higher_is_better"},
			{Name: EndPRAUC, Unit: "fraction", Primary: false, Direction: "higher_is_better"},
			{Name: EndDetectionLatency, Unit: "seconds", Primary: false, Direction: "lower_is_better"},
			{Name: EndObjectiveComplete, Unit: "fraction", Primary: true, Direction: "lower_is_better"},
			{Name: EndObjectiveTime, Unit: "seconds", Primary: false, Direction: "higher_is_better"},
			{Name: EndDiversion, Unit: "fraction", Primary: false, Direction: "higher_is_better"},
			{Name: EndAdaptationLatency, Unit: "seconds", Primary: true, Direction: "lower_is_better"},
			{Name: EndDefenderUtility, Unit: "score", Primary: false, Direction: "higher_is_better"},
			{Name: EndBenignDisruption, Unit: "fraction", Primary: true, Direction: "lower_is_better"},
			{Name: EndThroughput, Unit: "rps", Primary: false, Direction: "higher_is_better"},
			{Name: EndDeceptionDiscov, Unit: "fraction", Primary: false, Direction: "lower_is_better"},
		},
		Hypotheses: []Hypothesis{
			{
				ID: "H1", Preregistered: true,
				Statement:       "Coordinated autonomous activity is distinguishable from independent scanning and legitimate automation using behavioral coordination features, without relying on IP or user-agent.",
				Null:            "Coordination features provide no recall/FPR improvement over a request-rate-only heuristic at matched request budgets.",
				Prediction:      "Full Heramba achieves higher recall at equal-or-lower FPR than request-only and static-correlation baselines on held-out coordinated families.",
				PrimaryEndpoint: EndRecall, Secondary: []string{EndFPR, EndPrecision, EndPRAUC, EndDetectionLatency},
			},
			{
				ID: "H2", Preregistered: true,
				Statement:       "Adaptive deception reduces attacker objective completion relative to no defense, conventional rate limiting, and static decoys.",
				Null:            "Adaptive deception yields no reduction in objective completion versus the predeclared baseline.",
				Prediction:      "Adaptive deception lowers objective-completion rate and raises time-to-objective versus all weaker arms.",
				PrimaryEndpoint: EndObjectiveComplete, Secondary: []string{EndObjectiveTime, EndDiversion, EndDeceptionDiscov},
			},
			{
				ID: "H3", Preregistered: true,
				Statement:       "The defender can adapt to a bounded attacker strategy shift quickly enough to retain positive defender utility.",
				Null:            "Defender adaptation latency is unbounded or net defender utility is non-positive after a strategy shift.",
				Prediction:      "Adaptation latency stays within the policy-safe response target and net utility remains positive after shifts.",
				PrimaryEndpoint: EndAdaptationLatency, Secondary: []string{EndDefenderUtility, EndObjectiveComplete},
			},
			{
				ID: "H4", Preregistered: true,
				Statement:       "Detection degrades gracefully, not catastrophically, when a sensor is missing.",
				Null:            "Removing any single sensor collapses recall to the request-only baseline.",
				Prediction:      "Recall under a missing sensor stays above the request-only baseline, with a measured, bounded drop.",
				PrimaryEndpoint: EndRecall, Secondary: []string{EndFPR},
			},
			{
				ID: "H5", Preregistered: true,
				Statement:       "Safety holds and overhead stays bounded: attacker-controlled text never changes policy, and added latency/disruption stay within target.",
				Null:            "Attacker-controlled input can alter a policy decision, or added latency/disruption exceed target.",
				Prediction:      "Zero policy changes from attacker-controlled fields; benign disruption and added latency within target.",
				PrimaryEndpoint: EndBenignDisruption, Secondary: []string{EndThroughput},
			},
		},
		ActorModels: []ActorModel{
			{
				ID: "scripted_scanner", Kind: "scripted",
				Capabilities:  []string{"sequential_enumeration", "fixed_wordlist"},
				RequestBudget: 200, TimeBudgetSec: 600, Objective: "find_admin_surface",
				TargetVisibility: "none",
				DefenderCanSee:   []string{"requests", "decoy_hits", "timing"},
				DefenderControls: []string{"rate_limit", "decoy", "restrict_session"},
			},
			{
				ID: "independent_scanners", Kind: "independent",
				Capabilities:  []string{"parallel_sources", "no_shared_memory"},
				RequestBudget: 200, TimeBudgetSec: 600, Objective: "find_admin_surface",
				TargetVisibility: "none",
				DefenderCanSee:   []string{"requests", "decoy_hits", "timing"},
				DefenderControls: []string{"rate_limit", "decoy"},
			},
			{
				ID: "coordinated_swarm", Kind: "coordinated",
				Capabilities:  []string{"shared_observations", "divided_targeting", "synchronized_timing"},
				RequestBudget: 200, TimeBudgetSec: 600, Objective: "find_and_exfiltrate",
				TargetVisibility: "partial",
				DefenderCanSee:   []string{"requests", "decoy_hits", "timing", "target_overlap"},
				DefenderControls: []string{"rate_limit", "decoy", "restrict_session", "escalate"},
			},
			{
				ID: "legit_automation", Kind: "legit_automation",
				Capabilities:  []string{"high_volume", "narrow_known_endpoints"},
				RequestBudget: 400, TimeBudgetSec: 600, Objective: "",
				TargetVisibility: "full",
				DefenderCanSee:   []string{"requests", "timing"},
				DefenderControls: []string{"rate_limit"},
			},
			{
				ID: "human_tester", Kind: "human",
				Capabilities:  []string{"low_volume", "exploratory", "irregular_timing"},
				RequestBudget: 60, TimeBudgetSec: 600, Objective: "manual_probe",
				TargetVisibility: "partial",
				DefenderCanSee:   []string{"requests", "timing"},
				DefenderControls: []string{"rate_limit"},
			},
			{
				ID: "deception_aware_swarm", Kind: "deception_aware",
				Capabilities:  []string{"shared_observations", "consistency_checking", "lure_rejection"},
				RequestBudget: 200, TimeBudgetSec: 600, Objective: "find_and_exfiltrate",
				TargetVisibility: "partial",
				DefenderCanSee:   []string{"requests", "decoy_hits", "timing", "target_overlap"},
				DefenderControls: []string{"rate_limit", "decoy", "restrict_session", "escalate"},
				Adaptive:         true,
			},
		},
		// Partitions: tuning seeds (train/tune/validation) are disjoint from test
		// seeds. Held-out families use fresh seed ranges and are never tuned on.
		Families: []ScenarioFamily{
			{Name: "coord-train", ActorModel: "coordinated_swarm", Partition: Train, Seeds: seedRange(1000, 8), ConcurrencyLevel: 3, BenignMix: true},
			{Name: "indep-train", ActorModel: "independent_scanners", Partition: Train, Seeds: seedRange(1100, 8), ConcurrencyLevel: 3, BenignMix: true},
			{Name: "legit-tune", ActorModel: "legit_automation", Partition: Tune, Seeds: seedRange(1200, 6), ConcurrencyLevel: 2, BenignMix: true},
			{Name: "coord-val", ActorModel: "coordinated_swarm", Partition: Validation, Seeds: seedRange(1300, 6), ConcurrencyLevel: 4, BenignMix: true},
			{Name: "coord-test", ActorModel: "coordinated_swarm", Partition: Test, Seeds: seedRange(9000, 10), ConcurrencyLevel: 5, BenignMix: true, HeldOut: true},
			{Name: "indep-test", ActorModel: "independent_scanners", Partition: Test, Seeds: seedRange(9100, 10), ConcurrencyLevel: 5, BenignMix: true, HeldOut: true},
			{Name: "legit-test", ActorModel: "legit_automation", Partition: Test, Seeds: seedRange(9200, 10), ConcurrencyLevel: 4, BenignMix: true, HeldOut: true},
			{Name: "deception-aware-test", ActorModel: "deception_aware_swarm", Partition: Test, Seeds: seedRange(9300, 10), ConcurrencyLevel: 5, BenignMix: true, HeldOut: true},
		},
		StatMethods: []string{
			"Wilson score interval for proportions (recall/FPR/objective-completion)",
			"nonparametric bootstrap (10000 resamples) for mean metrics and differences",
			"per-scenario confusion matrices with stratified reporting",
			"effect sizes reported alongside point estimates; no p-value-only claims",
		},
		StoppingRules: []string{
			"seeds and sample sizes fixed before running; no peeking-then-extending",
			"at 1% target FPR, evaluate >= 300 benign sessions per arm for a usable Wilson interval",
			"a run is valid only if it emits complete artifacts (config, seeds, revision, raw, summary)",
		},
		LeakageRules: []string{
			"test seeds disjoint from all tuning seeds (enforced by Protocol.Validate)",
			"no feature derived from IP octets, user-agent string, or simulator-specific constants",
			"held-out attacker families (deception_aware_swarm) never used for tuning",
			"the scoring labels (IsAttacker/IsGoal) are never visible to any detector",
		},
		Confounders: []string{
			"request volume correlated with actor kind -> matched request budgets",
			"timing artifacts of the generator -> jittered, seed-varied offsets",
			"target-set size -> equalized target visibility where feasible",
		},
		ThreatsToValid: []string{
			"synthetic traffic is more separable than production; numbers are not field claims",
			"deception-aware adversary is a modeled heuristic, not a real LLM agent",
			"deterministic harness cannot measure CPU/memory/recovery; those are marked not-measured",
		},
	}
	return p
}

func seedRange(start int64, n int) []int64 {
	out := make([]int64, n)
	for i := 0; i < n; i++ {
		out[i] = start + int64(i)
	}
	return out
}

// WriteManifest writes the protocol as indented JSON — the reproducibility
// manifest committed alongside experiment artifacts.
func (p Protocol) WriteManifest(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(p)
}
