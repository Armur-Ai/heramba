package decoy

import (
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

// NewRegistry seeds the default decoys plus any extra paths, and a fresh canary
// store.
func NewRegistry(extra ...string) *Registry {
	r := &Registry{endpoints: map[string]Endpoint{}, canaries: NewCanaryStore()}
	for _, e := range DefaultDecoyPaths() {
		r.endpoints[e.Path] = e
	}
	for _, p := range extra {
		if p != "" {
			r.endpoints[p] = Endpoint{Path: p, Note: "operator-configured decoy"}
		}
	}
	return r
}

// Add registers an additional decoy path.
func (r *Registry) Add(path, note string) {
	if path == "" {
		return
	}
	r.mu.Lock()
	r.endpoints[path] = Endpoint{Path: path, Note: note}
	r.mu.Unlock()
}

// IsDecoy reports whether an exact request path is a decoy.
func (r *Registry) IsDecoy(path string) (Endpoint, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.endpoints[path]
	return e, ok
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
