// Package campaign is Heramba's campaign-intelligence layer. It turns a stream
// of normalized evidence (internal/evidence) into a temporal graph of
// sessions, inferred actors, and campaigns — the structure an analyst needs to
// answer "is this one coordinated effort, and what did it actually do?"
//
// The cardinal rule from the engineering mandate governs everything here: an IP
// address is a transport key, never an identity. Grouping events by source key
// into a Session is an *observation*. Linking sessions across time or across
// source keys into a single Actor is always an *inference*, recorded as an
// evidence.Hypothesis with supporting and contradictory evidence and a
// calibrated confidence — and it is reversible (merge/split correction).
package campaign

import (
	"sort"
	"strings"

	"github.com/Armur-Ai/heramba/internal/evidence"
)

// Fingerprint is a behavioral signature derived from the events in a session.
// It is the raw material for entity resolution: two sessions with a similar
// fingerprint are *candidates* for being the same actor, never a certainty.
// Every component is explainable — there is no opaque embedding.
type Fingerprint struct {
	UserAgents  map[string]struct{} `json:"user_agents"`  // distinct UA strings seen
	Accepts     map[string]struct{} `json:"accepts"`      // distinct Accept headers
	Methods     map[string]struct{} `json:"methods"`      // HTTP methods used
	PathStyles  map[string]struct{} `json:"path_styles"`  // coarse path shape buckets
	ToolSignals map[string]struct{} `json:"tool_signals"` // classifier tool hints
	// IdentityTokens are *distinctive* identifiers that genuinely tie activity
	// across transport source keys: a planted canary token, an authenticated
	// subject, a session id, an API-key id, a unique custom header. Only these
	// — never a generic tool UA — justify merging two source keys into one
	// actor. Everything else is coordination evidence, handled at the campaign
	// layer, not identity evidence.
	IdentityTokens map[string]struct{} `json:"identity_tokens"`
	CadenceBucket  string              `json:"cadence_bucket"` // slow|steady|fast|burst
}

func newFingerprint() Fingerprint {
	return Fingerprint{
		UserAgents:     map[string]struct{}{},
		Accepts:        map[string]struct{}{},
		Methods:        map[string]struct{}{},
		PathStyles:     map[string]struct{}{},
		ToolSignals:    map[string]struct{}{},
		IdentityTokens: map[string]struct{}{},
	}
}

// observe folds one event's normalized attributes into the fingerprint.
func (f *Fingerprint) observe(e evidence.SecurityEvent) {
	add := func(set map[string]struct{}, key string) {
		if v := strings.TrimSpace(e.Attributes[key]); v != "" {
			set[strings.ToLower(v)] = struct{}{}
		}
	}
	add(f.UserAgents, "ua")
	add(f.UserAgents, "user_agent")
	add(f.Accepts, "accept")
	add(f.Methods, "method")
	add(f.ToolSignals, "tool")
	// Distinctive cross-source identity signals.
	add(f.IdentityTokens, "canary")
	add(f.IdentityTokens, "auth_subject")
	add(f.IdentityTokens, "session")
	add(f.IdentityTokens, "api_key_id")
	if p := e.Attributes["path"]; p != "" {
		f.PathStyles[pathStyle(p)] = struct{}{}
	}
	if e.Target != "" {
		f.PathStyles[pathStyle(e.Target)] = struct{}{}
	}
}

// pathStyle buckets a path into a coarse shape so that /users/1 and /users/2
// look alike but /admin and /.env do not. It keeps the first segment and the
// structural shape of the rest.
func pathStyle(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return "/"
	}
	segs := strings.Split(strings.Trim(p, "/"), "/")
	var b strings.Builder
	for i, s := range segs {
		if i > 3 {
			b.WriteString("/…")
			break
		}
		b.WriteByte('/')
		switch {
		case s == "":
			// skip
		case isNumeric(s):
			b.WriteString("{n}")
		case len(s) >= 24 && looksHex(s):
			b.WriteString("{id}")
		case i == 0:
			b.WriteString(strings.ToLower(s))
		default:
			b.WriteString("{s}")
		}
	}
	if b.Len() == 0 {
		return "/"
	}
	return b.String()
}

func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func looksHex(s string) bool {
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'f':
		case r >= 'A' && r <= 'F':
		case r == '-':
		default:
			return false
		}
	}
	return true
}

// Similarity returns an explainable [0,1] similarity between two fingerprints:
// the mean Jaccard overlap of the set-valued components, with a bonus when the
// cadence buckets match. It is symmetric. Empty-vs-empty components are skipped
// so a sparse fingerprint is not unfairly penalized.
func (f Fingerprint) Similarity(o Fingerprint) float64 {
	var sum float64
	var n int
	for _, pair := range [][2]map[string]struct{}{
		{f.UserAgents, o.UserAgents},
		{f.Accepts, o.Accepts},
		{f.Methods, o.Methods},
		{f.PathStyles, o.PathStyles},
		{f.ToolSignals, o.ToolSignals},
	} {
		if len(pair[0]) == 0 && len(pair[1]) == 0 {
			continue
		}
		sum += jaccard(pair[0], pair[1])
		n++
	}
	if n == 0 {
		return 0
	}
	score := sum / float64(n)
	if f.CadenceBucket != "" && f.CadenceBucket == o.CadenceBucket {
		score = score + (1-score)*0.15 // small corroborating bonus
	}
	return score
}

func jaccard(a, b map[string]struct{}) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1
	}
	inter := 0
	for k := range a {
		if _, ok := b[k]; ok {
			inter++
		}
	}
	union := len(a) + len(b) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

// sortedKeys is a small helper for deterministic explanations.
func sortedKeys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
