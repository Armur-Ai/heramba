package tarpit

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLabyrinthDeterministic(t *testing.T) {
	a := NewLabyrinth(42, "/maze", 6, 10)
	b := NewLabyrinth(42, "/maze", 6, 10)
	p1 := a.Page("/maze/amber-ledger-1234")
	p2 := b.Page("/maze/amber-ledger-1234")
	if p1.Title != p2.Title || strings.Join(p1.Links, ",") != strings.Join(p2.Links, ",") {
		t.Fatal("same seed+path must yield identical page")
	}
	// Different seed should (very likely) differ.
	c := NewLabyrinth(43, "/maze", 6, 10)
	if strings.Join(c.Page("/maze").Links, ",") == strings.Join(a.Page("/maze").Links, ",") {
		t.Fatal("different seed should change the maze")
	}
}

func TestLabyrinthFanout(t *testing.T) {
	l := NewLabyrinth(1, "/maze", 7, 10)
	pg := l.Page("/maze")
	if len(pg.Links) != 7 {
		t.Fatalf("fanout = %d, want 7", len(pg.Links))
	}
}

func TestLabyrinthLinksInternallyConsistent(t *testing.T) {
	l := NewLabyrinth(99, "/maze", 5, 8)
	// Every link must stay inside the maze and itself resolve to a page with
	// the same fanout — so a crawler can follow forever.
	pg := l.Page("/maze")
	for _, link := range pg.Links {
		if !strings.HasPrefix(link, "/maze/") {
			t.Fatalf("link escapes maze: %q", link)
		}
		child := l.Page(link)
		if len(child.Links) != 5 {
			t.Fatalf("child page %q has %d links", link, len(child.Links))
		}
		// And the child's links are distinct from the parent's (descending).
		if strings.Join(child.Links, ",") == strings.Join(pg.Links, ",") {
			t.Fatalf("child page duplicates parent links")
		}
	}
}

func TestLabyrinthBoundedDepth(t *testing.T) {
	l := NewLabyrinth(7, "/maze", 3, 4) // maxDepth 4
	// Walk deep following the first link each time; URL depth must never exceed
	// maxDepth, and once at the cap links must loop back to depth 1.
	path := "/maze"
	loopedBack := false
	for step := 0; step < 50; step++ {
		pg := l.Page(path)
		if d := l.depth(path); d > l.maxDepth {
			t.Fatalf("depth %d exceeded maxDepth %d at %q", d, l.maxDepth, path)
		}
		next := pg.Links[0]
		if l.depth(path) >= l.maxDepth {
			// at the cap, the next link should be a depth-1 node
			if l.depth(next) != 1 {
				t.Fatalf("at cap, next depth = %d, want 1 (%q)", l.depth(next), next)
			}
			loopedBack = true
		}
		path = next
	}
	if !loopedBack {
		t.Fatal("deep walk never reached the depth cap")
	}
}

func TestLabyrinthHTMLValidish(t *testing.T) {
	l := NewLabyrinth(3, "/maze", 4, 6)
	html := l.HTML("/maze")
	if !strings.HasPrefix(html, "<!doctype html>") || !strings.Contains(html, "</html>") {
		t.Fatalf("not an html doc:\n%s", html)
	}
	if strings.Count(html, "<a href=") != 4 {
		t.Fatalf("expected 4 anchors, got %d", strings.Count(html, "<a href="))
	}
}

func TestLabyrinthHandler(t *testing.T) {
	l := NewLabyrinth(5, "/maze", 4, 6)
	visited := ""
	h := l.Handler(nil, func(p string) { visited = p })
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/maze/some/deep/node", nil)
	h.ServeHTTP(rec, req)
	if visited != "/maze/some/deep/node" {
		t.Fatalf("onVisit not called with path, got %q", visited)
	}
	if rec.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Fatalf("content-type = %q", rec.Header().Get("Content-Type"))
	}
	if !strings.Contains(rec.Body.String(), "<a href=") {
		t.Fatal("handler body missing links")
	}
}

func TestLabyrinthNormalizesOffMaze(t *testing.T) {
	l := NewLabyrinth(1, "maze", 4, 6) // base without slashes
	if l.Base() != "/maze" {
		t.Fatalf("base = %q", l.Base())
	}
	// A path not under base is pulled back to the root page.
	pg := l.Page("/elsewhere")
	if pg.Path != "/maze" {
		t.Fatalf("off-maze path not normalized: %q", pg.Path)
	}
}
