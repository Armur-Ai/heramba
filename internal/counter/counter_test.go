package counter

import (
	"strings"
	"testing"
)

func TestGenerateHasKeyCommentPaths(t *testing.T) {
	l := Generate(0)
	if len(l.Key) != 12 {
		t.Fatalf("key should be 12 hex chars, got %q", l.Key)
	}
	if !strings.Contains(l.Comment, l.Key) {
		t.Fatal("comment should embed the tracking key")
	}
	if len(l.Paths) == 0 {
		t.Fatal("lure should reference fabricated decoy paths")
	}
}

func TestGenerateKeysUnique(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		k := Generate(-1).Key
		if seen[k] {
			t.Fatal("keys must be unique")
		}
		seen[k] = true
	}
}

func TestInjectHTMLBeforeBody(t *testing.T) {
	l := Generate(1)
	in := []byte("<html><body><h1>hi</h1></body></html>")
	out := string(InjectHTML(in, l))
	if !strings.Contains(out, "<!-- "+l.Comment+" -->") {
		t.Fatalf("comment not injected: %s", out)
	}
	// must sit before </body> and preserve the original content
	if strings.Index(out, l.Key) > strings.LastIndex(strings.ToLower(out), "</body>") {
		t.Fatal("comment should be injected before </body>")
	}
	if !strings.Contains(out, "<h1>hi</h1>") {
		t.Fatal("original body must be preserved")
	}
}

func TestInjectHTMLNoBodyAppends(t *testing.T) {
	l := Generate(2)
	out := string(InjectHTML([]byte("<p>fragment</p>"), l))
	if !strings.Contains(out, "<p>fragment</p>") || !strings.Contains(out, l.Key) {
		t.Fatalf("fragment should be preserved and comment appended: %s", out)
	}
}

func TestInjectJS(t *testing.T) {
	l := Generate(3)
	out := string(InjectJS([]byte("console.log(1)"), l))
	if !strings.HasPrefix(out, "/* ") || !strings.Contains(out, "console.log(1)") {
		t.Fatalf("JS comment should lead and preserve code: %s", out)
	}
}
