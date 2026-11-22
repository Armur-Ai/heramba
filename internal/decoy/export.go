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

// Caddy renders the surface as Caddyfile `respond` directives that return 404.
// Exact paths use an exact path matcher (e.g. `/.env`); zones use a wildcard
// prefix matcher (e.g. `/.git/*`). The note is emitted as its own comment line
// above each directive (Caddy does not reliably accept trailing inline comments).
func (s Surface) Caddy() string {
	var b strings.Builder
	b.WriteString("# Heramba decoy surface — mirror these in your Caddyfile (all return 404).\n")
	for _, e := range s.Paths {
		fmt.Fprintf(&b, "# %s\nrespond %s 404\n", e.Note, e.Path)
	}
	for _, e := range s.Prefixes {
		fmt.Fprintf(&b, "# zone: %s\nrespond %s* 404\n", e.Note, e.Path)
	}
	return b.String()
}

// HAProxy renders the surface as HAProxy ACLs plus a single 404 deny rule, to
// drop into a frontend/backend. Exact lures use `path`, zones use `path_beg`;
// both ACL lines share the name heramba_decoy, so HAProxy ORs them together.
func (s Surface) HAProxy() string {
	var b strings.Builder
	b.WriteString("# Heramba decoy surface — HAProxy. Add to a frontend/backend (all return 404).\n")
	if len(s.Paths) > 0 {
		b.WriteString("acl heramba_decoy path")
		for _, e := range s.Paths {
			fmt.Fprintf(&b, " %s", e.Path)
		}
		b.WriteString("\n")
	}
	if len(s.Prefixes) > 0 {
		b.WriteString("acl heramba_decoy path_beg")
		for _, e := range s.Prefixes {
			fmt.Fprintf(&b, " %s", e.Path)
		}
		b.WriteString("\n")
	}
	b.WriteString("http-request deny deny_status 404 if heramba_decoy\n")
	return b.String()
}

// Envoy renders the surface as Envoy HTTP route entries that direct_response
// 404 — exact lures via a `path` match, zones via a `prefix` match. Paste the
// routes into a virtual_host's `routes` list. Uses genuine Envoy route syntax.
func (s Surface) Envoy() string {
	var b strings.Builder
	b.WriteString("# Heramba decoy surface — Envoy routes (add under a virtual_host's routes:).\n")
	b.WriteString("routes:\n")
	for _, e := range s.Paths {
		fmt.Fprintf(&b, "  - match: { path: %q }   # %s\n", e.Path, e.Note)
		b.WriteString("    direct_response: { status: 404 }\n")
	}
	for _, e := range s.Prefixes {
		fmt.Fprintf(&b, "  - match: { prefix: %q }   # zone: %s\n", e.Path, e.Note)
		b.WriteString("    direct_response: { status: 404 }\n")
	}
	return b.String()
}

// Traefik renders the surface as a Traefik dynamic configuration (file
// provider): a single router whose rule ORs together a Path matcher per exact
// lure and a PathPrefix matcher per zone, pointing at a service the operator
// wires to return 404 (e.g. a blackhole). Uses genuine Traefik v2/v3 rule
// syntax so it drops straight into a dynamic-config file.
func (s Surface) Traefik() string {
	const bt = "`" // backtick, which Traefik rule matchers require around values
	var rules []string
	for _, e := range s.Paths {
		rules = append(rules, fmt.Sprintf("Path(%s%s%s)", bt, e.Path, bt))
	}
	for _, e := range s.Prefixes {
		rules = append(rules, fmt.Sprintf("PathPrefix(%s%s%s)", bt, e.Path, bt))
	}

	var b strings.Builder
	b.WriteString("# Heramba decoy surface — Traefik dynamic configuration (file provider).\n")
	b.WriteString("# Point the heramba-decoy-404 service at a 404/blackhole backend; every\n")
	b.WriteString("# listed path is zero-false-positive (no legitimate client touches it).\n")
	b.WriteString("http:\n")
	b.WriteString("  routers:\n")
	b.WriteString("    heramba-decoys:\n")
	fmt.Fprintf(&b, "      rule: %q\n", strings.Join(rules, " || "))
	b.WriteString("      service: heramba-decoy-404\n")
	b.WriteString("      priority: 1000\n")
	return b.String()
}
