// Package research is Heramba's research plane control layer: a
// schema-validated experimental protocol, threat model, pre-registered
// hypotheses, and leakage-proof data partitions that the Phase 2 experiments
// (coordination detection, deception objective, adaptation race) all run
// against.
//
// The protocol is code, not prose, for one reason: it can be validated and
// tests can assert that the experiments honor it. Validation catches the
// mistakes that quietly invalidate security ML — duplicate identifiers, a
// hypothesis with no null, train/test seed leakage, an endpoint no experiment
// reports — before any number is produced.
package research

import (
	"errors"
	"fmt"
	"sort"
)

// SchemaVersion versions the protocol manifest format for reproducibility.
const SchemaVersion = 1

// Partition is a data split. Train/tune/validation/test are kept strictly
// separate; held-out families and attacker strategies are reserved for test.
type Partition string

const (
	Train      Partition = "train"
	Tune       Partition = "tune"
	Validation Partition = "validation"
	Test       Partition = "test"
)

// AllPartitions returns the partitions in pipeline order.
func AllPartitions() []Partition { return []Partition{Train, Tune, Validation, Test} }

// Endpoint is a measured outcome. Primary endpoints are the pre-registered
// decision metrics; secondary endpoints are reported but not decision-driving.
type Endpoint struct {
	Name      string `json:"name"`
	Unit      string `json:"unit"`
	Primary   bool   `json:"primary"`
	Direction string `json:"direction"` // "higher_is_better" | "lower_is_better"
}

// Hypothesis is a pre-registered, falsifiable claim with an explicit null. A
// hypothesis without a null is not falsifiable and fails validation.
type Hypothesis struct {
	ID              string   `json:"id"`
	Statement       string   `json:"statement"`
	Null            string   `json:"null"`
	Prediction      string   `json:"prediction"`
	PrimaryEndpoint string   `json:"primary_endpoint"`
	Secondary       []string `json:"secondary_endpoints,omitempty"`
	Preregistered   bool     `json:"preregistered"`
}

// ActorModel is one entry in the threat model: an attacker (or benign) traffic
// shape with explicit capabilities, budget, and objective. Defining the budget
// and visibility explicitly is what makes a *fair* comparison possible.
type ActorModel struct {
	ID               string   `json:"id"`
	Kind             string   `json:"kind"` // scripted|independent|coordinated|legit_automation|human|deception_aware
	Capabilities     []string `json:"capabilities"`
	RequestBudget    int      `json:"request_budget"`    // max requests per actor
	TimeBudgetSec    int      `json:"time_budget_sec"`   // wall window the actor operates in
	Objective        string   `json:"objective"`         // "" for benign
	TargetVisibility string   `json:"target_visibility"` // full|partial|none
	DefenderCanSee   []string `json:"defender_can_see"`
	DefenderControls []string `json:"defender_controls"`
	Adaptive         bool     `json:"adaptive"` // adjusts to deception across rounds
}

// ScenarioFamily binds an actor model to the seeds and concurrency levels it is
// evaluated at, in a specific partition. Held-out families carry Test and are
// never used for tuning.
type ScenarioFamily struct {
	Name             string    `json:"name"`
	ActorModel       string    `json:"actor_model"`
	Partition        Partition `json:"partition"`
	Seeds            []int64   `json:"seeds"`
	ConcurrencyLevel int       `json:"concurrency_level"`
	BenignMix        bool      `json:"benign_mix"` // includes realistic benign traffic
	HeldOut          bool      `json:"held_out"`   // reserved: never tuned on
}

// Protocol is the full pre-registered experimental protocol.
type Protocol struct {
	SchemaVersion  int              `json:"schema_version"`
	Title          string           `json:"title"`
	Hypotheses     []Hypothesis     `json:"hypotheses"`
	ActorModels    []ActorModel     `json:"actor_models"`
	Families       []ScenarioFamily `json:"scenario_families"`
	Endpoints      []Endpoint       `json:"endpoints"`
	StatMethods    []string         `json:"statistical_methods"`
	StoppingRules  []string         `json:"stopping_rules"`
	LeakageRules   []string         `json:"leakage_prevention"`
	Confounders    []string         `json:"confounders"`
	ThreatsToValid []string         `json:"threats_to_validity"`
}

// Validation errors.
var (
	ErrNoHypotheses     = errors.New("protocol: no hypotheses")
	ErrNoPrimaryEndpnt  = errors.New("protocol: no primary endpoint defined")
	ErrSeedLeakage      = errors.New("protocol: seed appears in both a tuning and the test partition (leakage)")
	ErrUnknownActor     = errors.New("protocol: scenario family references unknown actor model")
	ErrUnknownEndpoint  = errors.New("protocol: hypothesis references unknown endpoint")
	ErrNoHeldOut        = errors.New("protocol: no held-out test family reserved")
	ErrNullMissing      = errors.New("protocol: hypothesis has no null (not falsifiable)")
	ErrNotPreregistered = errors.New("protocol: hypothesis is not marked pre-registered")
)

// tuningPartition reports whether a partition is used to fit or select the
// detector (anything but Test). Leakage = a Test seed reused in one of these.
func tuningPartition(p Partition) bool { return p != Test }

// Validate enforces the protocol's own integrity rules. It returns a joined
// error describing every problem found (not just the first), so a malformed
// protocol cannot silently drive an experiment.
func (p Protocol) Validate() error {
	var errs []error

	if len(p.Hypotheses) == 0 {
		errs = append(errs, ErrNoHypotheses)
	}

	endpointNames := map[string]bool{}
	hasPrimary := false
	for _, e := range p.Endpoints {
		endpointNames[e.Name] = true
		if e.Primary {
			hasPrimary = true
		}
	}
	if !hasPrimary {
		errs = append(errs, ErrNoPrimaryEndpnt)
	}

	seenHyp := map[string]bool{}
	for _, h := range p.Hypotheses {
		if seenHyp[h.ID] {
			errs = append(errs, fmt.Errorf("protocol: duplicate hypothesis id %q", h.ID))
		}
		seenHyp[h.ID] = true
		if h.Null == "" {
			errs = append(errs, fmt.Errorf("%w: %s", ErrNullMissing, h.ID))
		}
		if !h.Preregistered {
			errs = append(errs, fmt.Errorf("%w: %s", ErrNotPreregistered, h.ID))
		}
		if h.PrimaryEndpoint != "" && !endpointNames[h.PrimaryEndpoint] {
			errs = append(errs, fmt.Errorf("%w: %s -> %s", ErrUnknownEndpoint, h.ID, h.PrimaryEndpoint))
		}
		for _, s := range h.Secondary {
			if !endpointNames[s] {
				errs = append(errs, fmt.Errorf("%w: %s -> %s", ErrUnknownEndpoint, h.ID, s))
			}
		}
	}

	actorIDs := map[string]bool{}
	for _, a := range p.ActorModels {
		if actorIDs[a.ID] {
			errs = append(errs, fmt.Errorf("protocol: duplicate actor model id %q", a.ID))
		}
		actorIDs[a.ID] = true
	}

	// Seed leakage: a seed used in any tuning partition must not reappear in Test.
	tuningSeeds := map[int64]string{}
	testSeeds := map[int64]string{}
	anyHeldOut := false
	for _, f := range p.Families {
		if f.ActorModel != "" && !actorIDs[f.ActorModel] {
			errs = append(errs, fmt.Errorf("%w: %s -> %s", ErrUnknownActor, f.Name, f.ActorModel))
		}
		if f.HeldOut {
			anyHeldOut = true
		}
		for _, s := range f.Seeds {
			if tuningPartition(f.Partition) {
				tuningSeeds[s] = f.Name
			} else {
				testSeeds[s] = f.Name
			}
		}
	}
	for s, famTest := range testSeeds {
		if famTune, ok := tuningSeeds[s]; ok {
			errs = append(errs, fmt.Errorf("%w: seed %d in %q (tuning) and %q (test)", ErrSeedLeakage, s, famTune, famTest))
		}
	}
	if !anyHeldOut {
		errs = append(errs, ErrNoHeldOut)
	}

	return errors.Join(errs...)
}

// PrimaryEndpoints returns the pre-registered decision endpoints.
func (p Protocol) PrimaryEndpoints() []Endpoint {
	var out []Endpoint
	for _, e := range p.Endpoints {
		if e.Primary {
			out = append(out, e)
		}
	}
	return out
}

// TestFamilies returns the held-out families reserved for final evaluation.
func (p Protocol) TestFamilies() []ScenarioFamily {
	var out []ScenarioFamily
	for _, f := range p.Families {
		if f.Partition == Test {
			out = append(out, f)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// ActorByID looks up an actor model.
func (p Protocol) ActorByID(id string) (ActorModel, bool) {
	for _, a := range p.ActorModels {
		if a.ID == id {
			return a, true
		}
	}
	return ActorModel{}, false
}
