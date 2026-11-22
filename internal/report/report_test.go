package report

import (
	"strings"
	"testing"
	"time"

	"github.com/Armur-Ai/heramba/internal/event"
	"github.com/Armur-Ai/heramba/internal/watch"
)

func sampleState() *watch.State {
	s := watch.NewState(100)
	base := time.Date(2026, 11, 22, 10, 0, 0, 0, time.UTC)
	for i, src := range []string{"1.1.1.1", "2.2.2.2"} {
		e := event.Deception(event.KindDecoyHit, src, "/.env", "d").WithLabel("campaign", "sweep")
		e.Time = base.Add(time.Duration(i) * time.Minute)
		s.Add(e)
	}
	e := event.New(event.KindEgressBlocked, event.SeverityCritical, "1.1.1.1", "evil.test", "exfil")
	e.Time = base.Add(2 * time.Minute)
	s.Add(e)
	return s
}

func TestMarkdownContainsKeyFacts(t *testing.T) {
	md := Markdown(sampleState(), "Test report")
	for _, want := range []string{
		"# Test report",
		"2026-11-22 10:00:00 UTC", // window start = first event
		"Coordinated swarms:** 1", // two sources, one campaign
		"1.1.1.1",
		"/.env",
		"## Top sources",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q", want)
		}
	}
}

func TestHTMLIsSelfContainedAndEscaped(t *testing.T) {
	s := watch.NewState(10)
	// A decoy hit's target is rendered in the decoy map, so it exercises escaping.
	e := event.Deception(event.KindDecoyHit, "1.1.1.1", "/<script>", "x")
	e.Time = time.Now()
	s.Add(e)
	out := HTML(s, "R")
	if !strings.HasPrefix(out, "<!doctype html>") || !strings.Contains(out, "<style>") {
		t.Fatal("HTML should be self-contained with inline CSS")
	}
	if strings.Contains(out, "/<script>") {
		t.Error("HTML must escape target values")
	}
	if !strings.Contains(out, "&lt;script&gt;") {
		t.Error("expected escaped target")
	}
}

func TestEmptyStateRenders(t *testing.T) {
	md := Markdown(watch.NewState(10), "")
	if !strings.Contains(md, "Heramba incident report") {
		t.Fatal("empty report should still render with default title")
	}
}
