package tarpit

import (
	"fmt"
	"hash/fnv"
	"net/http"
	"strconv"
	"strings"
)

// Labyrinth generates an endless tree of believable-but-fake interlinked pages.
// A crawler that enters it keeps following links forever without ever escaping
// or exhausting our memory: each page is a pure function of its own URL and a
// seed, so serving one is O(1) in time and memory (fanout is a small constant)
// and no global state ever grows. URL depth is capped, so links loop back onto
// the maze rather than growing unbounded — the crawler never runs out of maze,
// and we never run out of RAM.
type Labyrinth struct {
	seed     int64
	base     string // path prefix the maze lives under, e.g. "/maze"
	fanout   int    // links per page
	maxDepth int    // cap on URL depth; deeper links loop back to depth 1
}

// NewLabyrinth returns a Labyrinth under base with the given seed. fanout and
// maxDepth are clamped to sane defaults (fanout 8, maxDepth 12) when non-
// positive. base is normalized to a leading-slash, no-trailing-slash prefix.
func NewLabyrinth(seed int64, base string, fanout, maxDepth int) *Labyrinth {
	if fanout <= 0 {
		fanout = 8
	}
	if maxDepth <= 0 {
		maxDepth = 12
	}
	base = "/" + strings.Trim(base, "/")
	if base == "/" {
		base = "/maze"
	}
	return &Labyrinth{seed: seed, base: base, fanout: fanout, maxDepth: maxDepth}
}

// Base returns the path prefix the maze is served under.
func (l *Labyrinth) Base() string { return l.base }

// Page is one generated maze page.
type Page struct {
	Path  string
	Title string
	Links []string // absolute paths, all under the maze, bounded in length
}

// depth counts path segments below the base.
func (l *Labyrinth) depth(path string) int {
	rel := strings.Trim(strings.TrimPrefix(path, l.base), "/")
	if rel == "" {
		return 0
	}
	return strings.Count(rel, "/") + 1
}

// hash64 mixes the seed and the given strings into a stable 64-bit value.
func (l *Labyrinth) hash64(parts ...string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(strconv.FormatInt(l.seed, 10)))
	h.Write([]byte{0})
	for _, p := range parts {
		_, _ = h.Write([]byte(p))
		h.Write([]byte{0})
	}
	return h.Sum64()
}

var labAdjs = []string{"amber", "north", "quartz", "delta", "orbit", "cedar", "vivid", "lunar", "ferric", "basalt"}
var labNouns = []string{"ledger", "atlas", "harbor", "matrix", "archive", "catalog", "dossier", "manifest", "registry", "vault"}
var labTitles = []string{"Archive", "Catalog", "Report", "Index", "Registry", "Export", "Dossier", "Manifest"}

// segment derives a deterministic slug-like path segment for the i-th child of
// path. Same (path, i) always yields the same segment, so the maze is self-
// consistent across visits.
func (l *Labyrinth) segment(path string, i int) string {
	hv := l.hash64(path, "child", strconv.Itoa(i))
	adj := labAdjs[hv%uint64(len(labAdjs))]
	noun := labNouns[(hv>>8)%uint64(len(labNouns))]
	return fmt.Sprintf("%s-%s-%04x", adj, noun, (hv>>16)&0xffff)
}

// Page builds the maze page for path. For any path under the base it returns a
// consistent page with exactly fanout links. Links descend until maxDepth, then
// loop back to depth 1 so URLs stay bounded while the maze stays endless.
func (l *Labyrinth) Page(path string) Page {
	if !strings.HasPrefix(path, l.base) {
		path = l.base
	}
	d := l.depth(path)
	links := make([]string, 0, l.fanout)
	for i := 0; i < l.fanout; i++ {
		seg := l.segment(path, i)
		var child string
		if d >= l.maxDepth {
			child = l.base + "/" + seg // loop back to depth 1, bounded length
		} else {
			child = strings.TrimRight(path, "/") + "/" + seg
		}
		links = append(links, child)
	}
	tv := l.hash64(path, "title")
	title := fmt.Sprintf("%s %04x", labTitles[tv%uint64(len(labTitles))], tv&0xffff)
	return Page{Path: path, Title: title, Links: links}
}

// HTML renders a maze page as a small, believable HTML document. It is bounded
// in size (fanout links) and references only other maze pages.
func (l *Labyrinth) HTML(path string) string {
	pg := l.Page(path)
	var b strings.Builder
	b.WriteString("<!doctype html><html><head><title>")
	b.WriteString(htmlEscape(pg.Title))
	b.WriteString("</title></head><body>\n<h1>")
	b.WriteString(htmlEscape(pg.Title))
	b.WriteString("</h1>\n<p>Internal document index. Select a record to continue.</p>\n<ul>\n")
	for _, link := range pg.Links {
		label := link[strings.LastIndexByte(link, '/')+1:]
		b.WriteString(`  <li><a href="` + htmlEscape(link) + `">` + htmlEscape(label) + "</a></li>\n")
	}
	b.WriteString("</ul>\n</body></html>\n")
	return b.String()
}

// Handler serves the maze over HTTP. If a Tarpit is given, pages are dripped out
// slowly; otherwise they are written at once. onVisit, if non-nil, is called
// with each visited path (so the edge can record a decoy hit per request).
func (l *Labyrinth) Handler(tp *Tarpit, onVisit func(path string)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if onVisit != nil {
			onVisit(r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		body := []byte(l.HTML(r.URL.Path))
		if tp != nil {
			_, _ = tp.Drip(r.Context(), w, body)
			return
		}
		_, _ = w.Write(body)
	})
}

func htmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&#39;")
	return r.Replace(s)
}
