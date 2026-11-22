package tripwire

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Armur-Ai/heramba/internal/event"
)

// Hit describes a request that touched a tripwire: the authenticated claims, the
// token found, and where in the request it appeared.
type Hit struct {
	Claims Claims
	Token  string
	Where  string // "path" or "query:<key>"
}

// Matcher inspects incoming requests for trip tokens. It verifies tokens
// cryptographically, so it needs no registry of planted paths: any request
// carrying a token this Matcher's key signed is a hit, and nothing else is.
type Matcher struct {
	signer *Signer
	maxAge time.Duration // 0 = accept any age
}

// NewMatcher returns a Matcher that authenticates tokens with s.
func NewMatcher(s *Signer) *Matcher { return &Matcher{signer: s} }

// WithMaxAge returns a copy of the Matcher that rejects tokens older than d
// (stale bait the operator has rotated past). A d <= 0 disables the check.
func (m *Matcher) WithMaxAge(d time.Duration) *Matcher {
	cp := *m
	cp.maxAge = d
	return &cp
}

// looksLikeToken is a cheap pre-filter: every minted token has exactly three
// '.' separators, so a segment without them can't be one and is skipped before
// the HMAC work.
func looksLikeToken(seg string) bool {
	return strings.Count(seg, ".") == 3
}

func (m *Matcher) verify(tok string) (Claims, bool) {
	c, err := m.signer.VerifyFresh(tok, m.maxAge)
	if err != nil {
		return Claims{}, false
	}
	return c, true
}

// Match reports whether a request path or query carries a valid trip token. The
// path is split on "/" and each segment is checked; then each query value is
// checked. The first match wins (path segments before query values).
func (m *Matcher) Match(path, rawQuery string) (Hit, bool) {
	for _, seg := range strings.Split(path, "/") {
		if seg == "" || !looksLikeToken(seg) {
			continue
		}
		if c, ok := m.verify(seg); ok {
			return Hit{Claims: c, Token: seg, Where: "path"}, true
		}
	}
	if rawQuery != "" {
		if vals, err := url.ParseQuery(rawQuery); err == nil {
			// Iterate keys in sorted order for deterministic attribution.
			for _, key := range sortedKeys(vals) {
				for _, v := range vals[key] {
					if !looksLikeToken(v) {
						continue
					}
					if c, ok := m.verify(v); ok {
						return Hit{Claims: c, Token: v, Where: "query:" + key}, true
					}
				}
			}
		}
	}
	return Hit{}, false
}

// MatchRequest is Match over an *http.Request.
func (m *Matcher) MatchRequest(r *http.Request) (Hit, bool) {
	return m.Match(r.URL.Path, r.URL.RawQuery)
}

// Inspect returns a zero-false-positive deception event if the request touched a
// tripwire. The event is a KindDecoyHit at confidence 1.0, labeled with the bait
// kind, the triage label, where the token appeared, and when it was planted.
func (m *Matcher) Inspect(method, path, rawQuery, clientIP string) (event.Event, bool) {
	hit, ok := m.Match(path, rawQuery)
	if !ok {
		return event.Event{}, false
	}
	detail := method + " " + path + " — tripwire (" + string(hit.Claims.Kind) + ")"
	ev := event.Deception(event.KindDecoyHit, clientIP, path, detail).
		WithLabel("tripwire", "true").
		WithLabel("bait_kind", string(hit.Claims.Kind)).
		WithLabel("bait_label", hit.Claims.Label).
		WithLabel("token_where", hit.Where).
		WithLabel("planted_at", hit.Claims.Planted.Format(time.RFC3339))
	return ev, true
}

// InspectRequest is Inspect over an *http.Request, deriving the client IP from
// the supplied value (the edge already resolves the real source).
func (m *Matcher) InspectRequest(r *http.Request, clientIP string) (event.Event, bool) {
	return m.Inspect(r.Method, r.URL.Path, r.URL.RawQuery, clientIP)
}

func sortedKeys(v url.Values) []string {
	keys := make([]string, 0, len(v))
	for k := range v {
		keys = append(keys, k)
	}
	// small n; insertion sort avoids importing sort twice but sort is already
	// used elsewhere in the package — keep it simple and correct.
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}
