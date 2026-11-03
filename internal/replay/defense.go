package replay

import "strings"

// Reaction is a defense's response to an action.
type Reaction struct {
	Detected bool // the defense believes this action is hostile
	Contain  bool // the defense contains (blocks) the source going forward
}

// Defense is a pluggable defensive strategy evaluated by the Runner. Observe is
// called for each action in time order and must NOT read the ground-truth
// labels on the action — only the fields a real defense would see (source,
// path). The IsDecoy label is the exception: it represents the *real* fact that
// the path is a decoy, which a deception defense legitimately knows.
type Defense interface {
	Name() string
	Reset()
	Observe(a Action) Reaction
}

// --- (1) No specialized defense ---

type NoDefense struct{}

func (NoDefense) Name() string            { return "no_defense" }
func (NoDefense) Reset()                  {}
func (NoDefense) Observe(Action) Reaction { return Reaction{} }

// --- (2) Static rules: block requests to a fixed suspicious-path blocklist ---

type StaticRules struct {
	blocklist []string
}

func NewStaticRules() *StaticRules {
	return &StaticRules{blocklist: []string{"/admin", "/config", "/backup", "/.env", "/.git", "/wp-admin"}}
}
func (s *StaticRules) Name() string { return "static_rules" }
func (s *StaticRules) Reset()       {}
func (s *StaticRules) Observe(a Action) Reaction {
	for _, b := range s.blocklist {
		if strings.HasPrefix(a.Path, b) {
			return Reaction{Detected: true, Contain: true}
		}
	}
	return Reaction{}
}

// --- (3) Fixed Heramba deception: detect + contain on a decoy/canary hit ---

type FixedDeception struct{}

func (FixedDeception) Name() string { return "fixed_deception" }
func (FixedDeception) Reset()       {}
func (FixedDeception) Observe(a Action) Reaction {
	if a.IsDecoy { // zero-FP: only an attacker ever touches a decoy
		return Reaction{Detected: true, Contain: true}
	}
	return Reaction{}
}

// --- (4) Adaptive Heramba deception: decoy detection PLUS a behavioral signal
// that escalates to containment after a source enumerates enough distinct
// sensitive paths — so it can act before the decoy trip, without touching
// benign users (who don't enumerate sensitive paths). ---

type AdaptiveDeception struct {
	// EnumBudget: contain a source after it touches this many distinct recon
	// paths, even before a decoy hit.
	EnumBudget int
	seen       map[string]map[string]struct{}
}

func NewAdaptiveDeception() *AdaptiveDeception {
	return &AdaptiveDeception{EnumBudget: 3, seen: map[string]map[string]struct{}{}}
}
func (d *AdaptiveDeception) Name() string { return "adaptive_deception" }
func (d *AdaptiveDeception) Reset()       { d.seen = map[string]map[string]struct{}{} }

var sensitivePrefixes = []string{"/admin", "/api/users", "/api/keys", "/config", "/backup"}

func (d *AdaptiveDeception) Observe(a Action) Reaction {
	if a.IsDecoy {
		return Reaction{Detected: true, Contain: true}
	}
	sensitive := false
	for _, p := range sensitivePrefixes {
		if strings.HasPrefix(a.Path, p) {
			sensitive = true
			break
		}
	}
	if !sensitive {
		return Reaction{}
	}
	set := d.seen[a.Source]
	if set == nil {
		set = map[string]struct{}{}
		d.seen[a.Source] = set
	}
	set[a.Path] = struct{}{}
	if len(set) >= d.EnumBudget {
		// Behavioral escalation: this source is enumerating sensitive surface.
		return Reaction{Detected: true, Contain: true}
	}
	// Watching, not yet containing.
	return Reaction{Detected: true}
}
