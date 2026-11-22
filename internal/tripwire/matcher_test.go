package tripwire

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Armur-Ai/heramba/internal/event"
)

func TestMatchPlantedPaths(t *testing.T) {
	s := newTestSigner(t)
	p := NewPlanter(s, "")
	m := NewMatcher(s)

	for _, path := range p.AllPaths() {
		hit, ok := m.Match(path, "")
		if !ok {
			t.Fatalf("planted path not matched: %q", path)
		}
		if hit.Where != "path" {
			t.Fatalf("expected path match for %q, got %q", path, hit.Where)
		}
		if hit.Claims.Kind == "" || hit.Claims.Label == "" {
			t.Fatalf("hit missing claims for %q: %+v", path, hit.Claims)
		}
	}
}

func TestMatchQueryToken(t *testing.T) {
	s := newTestSigner(t)
	m := NewMatcher(s)
	tok := s.Mint(KindManual, "beacon")
	hit, ok := m.Match("/pixel.gif", "id=42&t="+tok+"&ref=x")
	if !ok {
		t.Fatal("token in query not matched")
	}
	if hit.Where != "query:t" {
		t.Fatalf("where = %q, want query:t", hit.Where)
	}
	if hit.Claims.Label != "beacon" {
		t.Fatalf("label = %q", hit.Claims.Label)
	}
}

func TestNoMatchOnCleanTraffic(t *testing.T) {
	s := newTestSigner(t)
	m := NewMatcher(s)
	clean := []struct{ path, query string }{
		{"/", ""},
		{"/index.html", ""},
		{"/api/v2/products", "page=2&sort=price"},
		{"/static/app.js", ""},
		{"/a.b.c/d.e.f", ""},     // dotty but not a real token
		{"/search", "q=a.b.c.d"}, // query value with 3 dots, not a token
		{"/files/report.final.v2.x", ""},
	}
	for _, c := range clean {
		if _, ok := m.Match(c.path, c.query); ok {
			t.Fatalf("false positive on clean request %q?%q", c.path, c.query)
		}
	}
}

func TestMatchWrongKeyNoHit(t *testing.T) {
	a := newTestSigner(t)
	other, _ := NewSigner([]byte("some-other-key-abcdefghij"))
	m := NewMatcher(a)
	// A token signed by a different key must not match.
	foreign := other.Mint(KindRobots, "/x/")
	if _, ok := m.Match("/decoy/"+foreign+"/file", ""); ok {
		t.Fatal("token from a foreign key must not match")
	}
}

func TestMatcherMaxAge(t *testing.T) {
	s := newTestSigner(t)
	old := s.mintAt(KindRobots, "/old/", time.Now().Add(-48*time.Hour))
	path := "/backup/" + old + "/dump.sql"

	fresh := NewMatcher(s) // no max age
	if _, ok := fresh.Match(path, ""); !ok {
		t.Fatal("no-maxage matcher should accept old token")
	}
	strict := NewMatcher(s).WithMaxAge(time.Hour)
	if _, ok := strict.Match(path, ""); ok {
		t.Fatal("strict matcher should reject a 48h-old token")
	}
}

func TestInspectEventShape(t *testing.T) {
	s := newTestSigner(t)
	p := NewPlanter(s, "")
	m := NewMatcher(s)
	path := p.RobotsTxt().Paths[0]

	ev, ok := m.Inspect("GET", path, "", "203.0.113.9")
	if !ok {
		t.Fatal("expected inspect hit")
	}
	if ev.Kind != event.KindDecoyHit {
		t.Fatalf("kind = %q, want decoy_hit", ev.Kind)
	}
	if ev.Confidence != 1.0 {
		t.Fatalf("confidence = %v, want 1.0", ev.Confidence)
	}
	if ev.Source != "203.0.113.9" || ev.Target != path {
		t.Fatalf("source/target wrong: %q %q", ev.Source, ev.Target)
	}
	for _, k := range []string{"tripwire", "bait_kind", "bait_label", "token_where", "planted_at"} {
		if ev.Labels[k] == "" {
			t.Fatalf("missing label %q in %+v", k, ev.Labels)
		}
	}
	if ev.Labels["bait_kind"] != string(KindRobots) {
		t.Fatalf("bait_kind = %q", ev.Labels["bait_kind"])
	}
}

func TestInspectNoHit(t *testing.T) {
	s := newTestSigner(t)
	m := NewMatcher(s)
	if _, ok := m.Inspect("GET", "/healthz", "", "10.0.0.1"); ok {
		t.Fatal("clean path must not produce an event")
	}
}

func TestInspectRequest(t *testing.T) {
	s := newTestSigner(t)
	p := NewPlanter(s, "")
	m := NewMatcher(s)
	path := p.SitemapXML().Paths[0]
	r := httptest.NewRequest("GET", path, nil)
	if _, ok := m.InspectRequest(r, "198.51.100.2"); !ok {
		t.Fatal("InspectRequest should hit on a planted path")
	}
}
