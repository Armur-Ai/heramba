package decoy

import (
	"strings"
	"sync"

	"github.com/Armur-Ai/heramba/internal/event"
)

// Endpoint is a decoy path. It exists only to be discovered by an attacker (or
// an autonomous enumeration agent). No legitimate client requests it, so a hit
// is a zero-false-positive signal.
type Endpoint struct {
	Path string `json:"path"`
	Note string `json:"note"` // why it's a plausible lure, for triage
}

// Registry holds the decoy endpoints and the canary store. Safe for concurrent
// use (the edge inspects from many goroutines).
type Registry struct {
	mu        sync.RWMutex
	endpoints map[string]Endpoint
	prefixes  map[string]Endpoint // decoy zones: any path UNDER this prefix is a hit
	canaries  *CanaryStore
}

// DefaultDecoyPaths are high-signal lures: paths that automated scanners and
// agentic attackers routinely probe but that a real user/app never requests.
func DefaultDecoyPaths() []Endpoint {
	return []Endpoint{
		{"/.env", "environment/secret file probe"},
		{"/.git/config", "exposed VCS metadata probe"},
		{"/admin", "admin console enumeration"},
		{"/api/internal", "internal API enumeration"},
		{"/actuator/env", "Spring Boot actuator secret probe"},
		{"/.aws/credentials", "cloud credential probe"},
		{"/wp-admin/", "WordPress admin probe"},
		{"/phpinfo.php", "info-leak probe"},
		{"/server-status", "status endpoint probe"},
		{"/swagger-internal.json", "internal API spec probe"},
	}
}

// DefaultDecoyPrefixes are decoy ZONES: path prefixes under which *every* path
// is a lure, because no legitimate client or app ever serves anything beneath
// them. Exact-path decoys miss a scanner that walks a family (/.git/HEAD,
// /.git/refs/heads/main, …); a prefix zone catches the whole family at zero
// false-positive cost. Each prefix ends in "/" so it matches a path segment
// boundary, never a same-prefixed real route.
func DefaultDecoyPrefixes() []Endpoint {
	return []Endpoint{
		{"/.git/", "exposed VCS repository walk"},
		{"/.svn/", "exposed SVN metadata walk"},
		{"/.hg/", "exposed Mercurial metadata walk"},
		{"/.aws/", "cloud credential directory probe"},
		{"/.ssh/", "SSH key directory probe"},
		{"/actuator/", "Spring Boot actuator enumeration"},
		{"/wp-admin/", "WordPress admin enumeration"},
	}
}

// NewRegistry seeds the default decoys + decoy zones plus any extra exact paths,
// and a fresh canary store.
func NewRegistry(extra ...string) *Registry {
	r := &Registry{endpoints: map[string]Endpoint{}, prefixes: map[string]Endpoint{}, canaries: NewCanaryStore()}
	for _, e := range DefaultDecoyPaths() {
		r.endpoints[e.Path] = e
	}
	for _, e := range DefaultDecoyPrefixes() {
		r.prefixes[e.Path] = e
	}
	for _, p := range extra {
		if p != "" {
			r.endpoints[p] = Endpoint{Path: p, Note: "operator-configured decoy"}
		}
	}
	return r
}

// Add registers an additional exact decoy path.
func (r *Registry) Add(path, note string) {
	if path == "" {
		return
	}
	r.mu.Lock()
	r.endpoints[path] = Endpoint{Path: path, Note: note}
	r.mu.Unlock()
}

// AddPrefix registers a decoy zone: any path under prefix becomes a decoy hit.
// A prefix should end in "/" so it matches a segment boundary.
func (r *Registry) AddPrefix(prefix, note string) {
	if prefix == "" {
		return
	}
	r.mu.Lock()
	r.prefixes[prefix] = Endpoint{Path: prefix, Note: note}
	r.mu.Unlock()
}

// IsDecoy reports whether a request path is a decoy — by exact match first, then
// by decoy zone (the longest matching prefix wins, for the most specific note).
func (r *Registry) IsDecoy(path string) (Endpoint, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if e, ok := r.endpoints[path]; ok {
		return e, true
	}
	var best Endpoint
	found := false
	for pfx, e := range r.prefixes {
		if strings.HasPrefix(path, pfx) && len(pfx) > len(best.Path) {
			best, found = e, true
		}
	}
	return best, found
}

// PrefixZones returns a snapshot of the registered decoy zones (prefixes).
func (r *Registry) PrefixZones() []Endpoint {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Endpoint, 0, len(r.prefixes))
	for _, e := range r.prefixes {
		out = append(out, e)
	}
	return out
}

// Canaries exposes the canary store (minting/checking tokens).
func (r *Registry) Canaries() *CanaryStore { return r.canaries }

// Endpoints returns a snapshot of registered decoys.
func (r *Registry) Endpoints() []Endpoint {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Endpoint, 0, len(r.endpoints))
	for _, e := range r.endpoints {
		out = append(out, e)
	}
	return out
}

// InspectPath returns a zero-FP deception event if the path is a decoy.
func (r *Registry) InspectPath(method, path, clientIP string) (event.Event, bool) {
	e, ok := r.IsDecoy(path)
	if !ok {
		return event.Event{}, false
	}
	ev := event.Deception(event.KindDecoyHit, clientIP, path,
		method+" "+path+" — decoy ("+e.Note+")").WithLabel("decoy_note", e.Note)
	return ev, true
}
