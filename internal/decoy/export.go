package decoy

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Surface is the full decoy surface (exact-path lures + decoy zones), suitable
// for documenting what the edge watches or pre-seeding an upstream/LB to mirror
// the same 404 behavior.
type Surface struct {
	Paths    []Endpoint `json:"paths"`
	Prefixes []Endpoint `json:"prefixes"`
}

// Export snapshots the registry's decoy surface.
func (r *Registry) Export() Surface {
	return Surface{Paths: r.Endpoints(), Prefixes: r.PrefixZones()}
}

// JSON renders the surface as indented JSON.
func (s Surface) JSON() ([]byte, error) {
	return json.MarshalIndent(s, "", "  ")
}

// Nginx renders the surface as nginx location blocks that return 404 — so an
// edge/LB in front can present the same decoy surface. Exact paths use `location
// =`, zones use a prefix `location`.
func (s Surface) Nginx() string {
	var b strings.Builder
	b.WriteString("# Heramba decoy surface — mirror these on your edge (all return 404).\n")
	for _, e := range s.Paths {
		fmt.Fprintf(&b, "location = %s { return 404; }  # %s\n", e.Path, e.Note)
	}
	for _, e := range s.Prefixes {
		fmt.Fprintf(&b, "location %s { return 404; }  # zone: %s\n", e.Path, e.Note)
	}
	return b.String()
}
