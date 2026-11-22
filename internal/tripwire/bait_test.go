package tripwire

import (
	"encoding/json"
	"encoding/xml"
	"strings"
	"testing"
)

func newTestPlanter(t *testing.T, base string) *Planter {
	t.Helper()
	return NewPlanter(newTestSigner(t), base)
}

// everyPathCarriesValidToken asserts that each planted path contains a path
// segment that verifies under the planter's signer and decodes to the given
// bait kind.
func everyPathCarriesValidToken(t *testing.T, s *Signer, paths []string, want Kind) {
	t.Helper()
	if len(paths) == 0 {
		t.Fatal("no paths planted")
	}
	for _, p := range paths {
		found := false
		for _, seg := range strings.Split(p, "/") {
			if c, err := s.Verify(seg); err == nil {
				found = true
				if c.Kind != want {
					t.Fatalf("path %q token kind = %q, want %q", p, c.Kind, want)
				}
				if c.Label == "" {
					t.Fatalf("path %q token has empty label", p)
				}
			}
		}
		if !found {
			t.Fatalf("path %q carries no valid trip token", p)
		}
	}
}

func TestRobotsTxtValid(t *testing.T) {
	s := newTestSigner(t)
	b := NewPlanter(s, "").RobotsTxt()
	if !strings.HasPrefix(b.Body, "User-agent: *") {
		t.Fatalf("robots.txt missing user-agent line:\n%s", b.Body)
	}
	disallow := 0
	for _, line := range strings.Split(b.Body, "\n") {
		if strings.HasPrefix(line, "Disallow: ") {
			disallow++
			path := strings.TrimPrefix(line, "Disallow: ")
			if !strings.HasPrefix(path, "/") {
				t.Fatalf("disallow path not absolute: %q", path)
			}
		}
	}
	if disallow != len(b.Paths) {
		t.Fatalf("disallow lines %d != planted paths %d", disallow, len(b.Paths))
	}
	everyPathCarriesValidToken(t, s, b.Paths, KindRobots)
}

func TestSitemapXMLValidAndAbsolute(t *testing.T) {
	s := newTestSigner(t)
	b := NewPlanter(s, "https://shop.example.com").SitemapXML()
	// Must parse as XML.
	var doc struct {
		URLs []struct {
			Loc string `xml:"loc"`
		} `xml:"url"`
	}
	if err := xml.Unmarshal([]byte(b.Body), &doc); err != nil {
		t.Fatalf("sitemap not valid XML: %v\n%s", err, b.Body)
	}
	if len(doc.URLs) != len(b.Paths) {
		t.Fatalf("sitemap url count %d != planted %d", len(doc.URLs), len(b.Paths))
	}
	for _, u := range doc.URLs {
		if !strings.HasPrefix(u.Loc, "https://shop.example.com/") {
			t.Fatalf("sitemap loc not absolute under base: %q", u.Loc)
		}
	}
	everyPathCarriesValidToken(t, s, b.Paths, KindSitemap)
}

func TestHTMLCommentsAreComments(t *testing.T) {
	s := newTestSigner(t)
	b := NewPlanter(s, "").HTMLComments()
	for _, line := range strings.Split(strings.TrimSpace(b.Body), "\n") {
		if !strings.HasPrefix(line, "<!--") || !strings.HasSuffix(line, "-->") {
			t.Fatalf("not an HTML comment: %q", line)
		}
	}
	everyPathCarriesValidToken(t, s, b.Paths, KindHTMLComment)
}

func TestLinkHeaderWellFormed(t *testing.T) {
	s := newTestSigner(t)
	b := NewPlanter(s, "").LinkHeader()
	// Each comma-separated element must be <url>; rel="...".
	for _, part := range strings.Split(b.Body, ", ") {
		if !strings.HasPrefix(part, "<") || !strings.Contains(part, ">; rel=") {
			t.Fatalf("malformed Link element: %q", part)
		}
	}
	everyPathCarriesValidToken(t, s, b.Paths, KindLinkHeader)
}

func TestJSConfigValidish(t *testing.T) {
	s := newTestSigner(t)
	b := NewPlanter(s, "").JSConfig()
	if !strings.Contains(b.Body, "window.__APP_CONFIG__") || !strings.Contains(b.Body, "internalEndpoints") {
		t.Fatalf("js config missing expected shape:\n%s", b.Body)
	}
	everyPathCarriesValidToken(t, s, b.Paths, KindJSConfig)
}

func TestAPIDocsValidJSON(t *testing.T) {
	s := newTestSigner(t)
	b := NewPlanter(s, "").APIDocs()
	var doc struct {
		Endpoints []struct {
			Path string `json:"path"`
		} `json:"endpoints"`
	}
	if err := json.Unmarshal([]byte(b.Body), &doc); err != nil {
		t.Fatalf("api docs not valid JSON: %v\n%s", err, b.Body)
	}
	if len(doc.Endpoints) != len(b.Paths) {
		t.Fatalf("endpoint count %d != planted %d", len(doc.Endpoints), len(b.Paths))
	}
	everyPathCarriesValidToken(t, s, b.Paths, KindAPIDoc)
}

func TestAllPathsSortedAndUnique(t *testing.T) {
	s := newTestSigner(t)
	paths := NewPlanter(s, "").AllPaths()
	if len(paths) == 0 {
		t.Fatal("no paths")
	}
	seen := map[string]bool{}
	for i, p := range paths {
		if seen[p] {
			t.Fatalf("duplicate planted path: %q", p)
		}
		seen[p] = true
		if i > 0 && paths[i-1] > p {
			t.Fatal("AllPaths not sorted")
		}
	}
}
