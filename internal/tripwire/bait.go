package tripwire

import (
	"sort"
	"strings"
	"time"
)

// Bait is one rendered bait surface ready to serve: its body, the content type
// to send it as, and the trip paths it planted (each path carries a signed
// token, so a later hit on any of them is attributable without a lookup).
type Bait struct {
	Body        string
	ContentType string
	Paths       []string
}

// Planter builds bait surfaces, minting a fresh trip token for every planted
// URL under a given Signer. A Planter holds no state beyond its signer and an
// optional absolute base URL (used where a surface wants absolute links, such
// as sitemap.xml); it is safe for concurrent use.
type Planter struct {
	signer *Signer
	base   string // e.g. "https://shop.example.com"; "" yields root-relative paths
}

// NewPlanter returns a Planter that signs trip tokens with s. base may be empty.
func NewPlanter(s *Signer, base string) *Planter {
	return &Planter{signer: s, base: strings.TrimRight(base, "/")}
}

// baitSpec is a juicy-looking location template. The trip token is inserted as
// its own clean path segment between dir and leaf, so the matcher recovers it by
// verifying a single segment (dir and leaf only make it look real).
type baitSpec struct {
	dir  string // juicy parent, leading slash, no trailing slash
	leaf string // filename after the token segment; "" for none
	note string // triage label sealed into the token
}

func (p *Planter) path(sp baitSpec, kind Kind) string {
	tok := p.signer.Mint(kind, sp.note)
	seg := sp.dir + "/" + tok
	if sp.leaf != "" {
		seg += "/" + sp.leaf
	}
	return seg
}

// loc turns a root-relative trip path into whatever form a surface wants: an
// absolute URL when a base was configured, else the path unchanged.
func (p *Planter) loc(path string) string {
	if p.base == "" {
		return path
	}
	return p.base + path
}

// Curated juicy locations. These read like things a careless team left exposed
// (backups, dumps, internal admin tools, cloud creds) — exactly what an
// automated scanner or assessment agent prioritizes, and what a human browsing
// the real site never navigates to.
var (
	robotsSpecs = []baitSpec{
		{"/backup", "db-2024.sql", "robots: exposed database backup"},
		{"/admin/export", "users.csv", "robots: admin user export"},
		{"/internal/debug", "", "robots: internal debug console"},
		{"/.git", "config", "robots: exposed VCS directory"},
		{"/api/v1/admin", "keys", "robots: admin API keys"},
	}
	sitemapSpecs = []baitSpec{
		{"/dashboard/reports", "q3-financials.xlsx", "sitemap: financial report"},
		{"/internal/wiki", "credentials", "sitemap: internal wiki creds page"},
		{"/billing/invoices", "export.csv", "sitemap: billing export"},
		{"/staff/directory", "", "sitemap: staff directory"},
	}
	htmlSpecs = []baitSpec{
		{"/backup", "nightly.tar.gz", "html-comment: nightly backup"},
		{"/admin/tools", "db-console.php", "html-comment: db console"},
		{"/api/internal", "swagger.json", "html-comment: internal api spec"},
	}
	linkSpecs = []baitSpec{
		{"/backup", "snapshot.tar.gz", "link-header: snapshot archive"},
		{"/.aws", "credentials", "link-header: aws credentials"},
	}
	jsConfigSpecs = []baitSpec{
		{"/api/v2/internal", "metrics", "jsconfig: internal metrics endpoint"},
		{"/admin/export", "all.json", "jsconfig: admin export endpoint"},
		{"/debug/vars", "", "jsconfig: debug vars endpoint"},
	}
	apiDocSpecs = []baitSpec{
		{"/api/internal/users", "dump", "apidoc: user dump endpoint"},
		{"/api/internal/secrets", "list", "apidoc: secrets list endpoint"},
		{"/api/internal/config", "raw", "apidoc: raw config endpoint"},
	}
)

// RobotsTxt renders a robots.txt whose Disallow entries point only at trip
// paths. A crawler that "respects" robots by avoiding them is unaffected; one
// that treats Disallow as a treasure map walks straight onto a tripwire.
func (p *Planter) RobotsTxt() Bait {
	var b strings.Builder
	b.WriteString("User-agent: *\n")
	paths := make([]string, 0, len(robotsSpecs))
	for _, sp := range robotsSpecs {
		path := p.path(sp, KindRobots)
		paths = append(paths, path)
		b.WriteString("Disallow: " + path + "\n")
	}
	return Bait{Body: b.String(), ContentType: "text/plain; charset=utf-8", Paths: paths}
}

// SitemapXML renders a sitemaps.org urlset listing only trip URLs, dressed with
// plausible lastmod timestamps.
func (p *Planter) SitemapXML() Bait {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")
	lastmod := time.Now().UTC().AddDate(0, 0, -3).Format("2006-01-02")
	paths := make([]string, 0, len(sitemapSpecs))
	for _, sp := range sitemapSpecs {
		path := p.path(sp, KindSitemap)
		paths = append(paths, path)
		b.WriteString("  <url><loc>" + xmlEscape(p.loc(path)) + "</loc>")
		b.WriteString("<lastmod>" + lastmod + "</lastmod>")
		b.WriteString("<priority>0.9</priority></url>\n")
	}
	b.WriteString("</urlset>\n")
	return Bait{Body: b.String(), ContentType: "application/xml; charset=utf-8", Paths: paths}
}

// HTMLComments renders a block of HTML comments that read like developer notes
// left in the markup, each pointing at a trip path. Body is meant to be injected
// into a served page (e.g. just before </body>); it is inert text to a browser.
func (p *Planter) HTMLComments() Bait {
	notes := []string{
		"TODO: remove before launch — staging backup at %s",
		"legacy admin tool (kill after migration): %s",
		"internal API spec, do not link publicly: %s",
	}
	var b strings.Builder
	paths := make([]string, 0, len(htmlSpecs))
	for i, sp := range htmlSpecs {
		path := p.path(sp, KindHTMLComment)
		paths = append(paths, path)
		note := notes[i%len(notes)]
		b.WriteString("<!-- " + sprintfSafe(note, path) + " -->\n")
	}
	return Bait{Body: b.String(), ContentType: "text/html; charset=utf-8", Paths: paths}
}

// LinkHeader renders a value for an HTTP Link: response header referencing trip
// paths with prefetch/preload rels — the sort of hint an automated client
// follows but a human never sees.
func (p *Planter) LinkHeader() Bait {
	rels := []string{"prefetch", "preload"}
	parts := make([]string, 0, len(linkSpecs))
	paths := make([]string, 0, len(linkSpecs))
	for i, sp := range linkSpecs {
		path := p.path(sp, KindLinkHeader)
		paths = append(paths, path)
		parts = append(parts, "<"+p.loc(path)+`>; rel="`+rels[i%len(rels)]+`"`)
	}
	return Bait{Body: strings.Join(parts, ", "), ContentType: "", Paths: paths}
}

// JSConfig renders a decoy front-end config blob that salts an "internal
// endpoints" map with trip paths — the kind of thing an agent that parses JS
// greedily extracts and then requests.
func (p *Planter) JSConfig() Bait {
	var b strings.Builder
	b.WriteString("window.__APP_CONFIG__ = {\n")
	b.WriteString(`  "env": "production",` + "\n")
	b.WriteString(`  "apiBase": "/api/v2",` + "\n")
	b.WriteString(`  "internalEndpoints": {` + "\n")
	paths := make([]string, 0, len(jsConfigSpecs))
	keys := []string{"metrics", "adminExport", "debugVars"}
	for i, sp := range jsConfigSpecs {
		path := p.path(sp, KindJSConfig)
		paths = append(paths, path)
		comma := ","
		if i == len(jsConfigSpecs)-1 {
			comma = ""
		}
		b.WriteString(`    "` + keys[i%len(keys)] + `": "` + jsEscape(path) + `"` + comma + "\n")
	}
	b.WriteString("  }\n};\n")
	return Bait{Body: b.String(), ContentType: "application/javascript; charset=utf-8", Paths: paths}
}

// APIDocs renders a fake internal API-documentation JSON listing privileged-
// looking endpoints, each a trip path.
func (p *Planter) APIDocs() Bait {
	var b strings.Builder
	b.WriteString("{\n")
	b.WriteString(`  "title": "Internal Operations API",` + "\n")
	b.WriteString(`  "version": "1.4.2",` + "\n")
	b.WriteString(`  "note": "internal only — not for external consumers",` + "\n")
	b.WriteString(`  "endpoints": [` + "\n")
	paths := make([]string, 0, len(apiDocSpecs))
	for i, sp := range apiDocSpecs {
		path := p.path(sp, KindAPIDoc)
		paths = append(paths, path)
		comma := ","
		if i == len(apiDocSpecs)-1 {
			comma = ""
		}
		b.WriteString(`    {"method": "GET", "path": "` + jsEscape(path) + `", "auth": "admin"}` + comma + "\n")
	}
	b.WriteString("  ]\n}\n")
	return Bait{Body: b.String(), ContentType: "application/json; charset=utf-8", Paths: paths}
}

// AllPaths renders every surface and returns the union of planted trip paths,
// sorted. Useful for seeding a decoy/canary registry or for tests.
func (p *Planter) AllPaths() []string {
	var all []string
	all = append(all, p.RobotsTxt().Paths...)
	all = append(all, p.SitemapXML().Paths...)
	all = append(all, p.HTMLComments().Paths...)
	all = append(all, p.LinkHeader().Paths...)
	all = append(all, p.JSConfig().Paths...)
	all = append(all, p.APIDocs().Paths...)
	sort.Strings(all)
	return all
}

func xmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&apos;")
	return r.Replace(s)
}

// jsEscape escapes a value for embedding inside a double-quoted JS/JSON string.
func jsEscape(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\r", `\r`, "<", `<`, ">", `>`)
	return r.Replace(s)
}

// sprintfSafe inserts arg for a single %s without pulling in fmt's verbs on
// attacker-influenced data (the template is ours; the path is URL-safe).
func sprintfSafe(tmpl, arg string) string {
	return strings.Replace(tmpl, "%s", arg, 1)
}
