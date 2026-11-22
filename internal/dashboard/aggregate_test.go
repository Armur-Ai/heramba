package dashboard

import (
	"testing"
	"time"

	"github.com/Armur-Ai/heramba/internal/event"
)

func evAt(k event.Kind, sev event.Severity, src, tgt string, t time.Time) event.Event {
	e := event.New(k, sev, src, tgt, "")
	e.Time = t
	return e
}

func TestParseWindow(t *testing.T) {
	cases := map[string]time.Duration{
		"":     15 * time.Minute,
		"junk": 15 * time.Minute,
		"1h":   time.Hour,
		"5s":   time.Minute,        // clamped up to the 1m floor
		"999h": 7 * 24 * time.Hour, // clamped down to the 7d ceiling
		"30m":  30 * time.Minute,
	}
	for in, want := range cases {
		if got := parseWindow(in); got != want {
			t.Errorf("parseWindow(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestBuildTimelineBucketing(t *testing.T) {
	end := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	window := 10 * time.Minute
	bucket := time.Minute // → 10 buckets, [end-10m, end]
	evs := []event.Event{
		// bucket 0: [end-10m, end-9m)
		evAt(event.KindProbe, event.SeverityLow, "a", "/x", end.Add(-10*time.Minute)),
		evAt(event.KindProbe, event.SeverityLow, "a", "/x", end.Add(-9*time.Minute-time.Second)),
		// bucket 5: [end-5m, end-4m)
		evAt(event.KindDecoyHit, event.SeverityHigh, "b", "/x", end.Add(-5*time.Minute)),
		// before the window — ignored
		evAt(event.KindProbe, event.SeverityLow, "a", "/x", end.Add(-30*time.Minute)),
		// at end — folded into the last bucket (index 9)
		evAt(event.KindCanaryTriggered, event.SeverityCritical, "c", "/y", end),
	}
	tl := buildTimelineAt(evs, window, bucket, end)

	if len(tl.Points) != 10 {
		t.Fatalf("points = %d, want 10", len(tl.Points))
	}
	if tl.WindowSeconds != 600 || tl.BucketSeconds != 60 {
		t.Errorf("window/bucket secs = %d/%d, want 600/60", tl.WindowSeconds, tl.BucketSeconds)
	}
	want := map[int]int{0: 2, 5: 1, 9: 1}
	total := 0
	for i, p := range tl.Points {
		total += p.Total
		if p.Total != want[i] {
			t.Errorf("bucket %d total = %d, want %d", i, p.Total, want[i])
		}
	}
	if total != 4 {
		t.Errorf("total across buckets = %d, want 4 (out-of-window dropped)", total)
	}
	if tl.Points[0].BySeverity["low"] != 2 {
		t.Errorf("bucket 0 low = %d, want 2", tl.Points[0].BySeverity["low"])
	}
	if tl.Points[9].BySeverity["critical"] != 1 {
		t.Errorf("bucket 9 critical = %d, want 1", tl.Points[9].BySeverity["critical"])
	}
	// First bucket starts exactly at end-window.
	if tl.Points[0].T != end.Add(-window).Unix() {
		t.Errorf("bucket 0 start = %d, want %d", tl.Points[0].T, end.Add(-window).Unix())
	}
}

func TestBuildTimelineEmpty(t *testing.T) {
	tl := buildTimeline(nil, time.Hour)
	if len(tl.Points) == 0 {
		t.Fatal("expected fixed-width buckets even with no events")
	}
	for _, p := range tl.Points {
		if p.Total != 0 {
			t.Fatalf("empty timeline has a non-zero bucket: %+v", p)
		}
	}
}

func TestTopSources(t *testing.T) {
	now := time.Now().UTC()
	evs := []event.Event{
		evAt(event.KindDecoyHit, event.SeverityLow, "1.1.1.1", "/a", now),
		evAt(event.KindDecoyHit, event.SeverityHigh, "1.1.1.1", "/b", now.Add(time.Second)),
		evAt(event.KindProbe, event.SeverityMedium, "2.2.2.2", "/a", now),
		evAt(event.KindProbe, event.SeverityInfo, "", "/a", now), // no source → skipped
	}
	got := topSources(evs)
	if len(got) != 2 {
		t.Fatalf("sources = %d, want 2", len(got))
	}
	if got[0].Source != "1.1.1.1" || got[0].Count != 2 {
		t.Errorf("top source = %+v, want 1.1.1.1 count 2", got[0])
	}
	if got[0].MaxSeverity != "high" {
		t.Errorf("max severity = %q, want high", got[0].MaxSeverity)
	}
}

func TestDetectCampaignsSwarm(t *testing.T) {
	now := time.Now().UTC()
	var evs []event.Event
	// 4 distinct IPs hitting the same labeled campaign → coordinated swarm.
	for i, ip := range []string{"a", "b", "c", "d"} {
		e := evAt(event.KindCanaryTriggered, event.SeverityHigh, ip, "/keys", now.Add(time.Duration(i)*time.Second))
		e = e.WithLabel("campaign", "swarm-1")
		evs = append(evs, e)
	}
	// a lone source → filtered out (single-source).
	evs = append(evs, evAt(event.KindProbe, event.SeverityLow, "z", "/lonely", now))

	got := detectCampaigns(evs)
	if len(got) != 1 {
		t.Fatalf("campaigns = %d, want 1 (single-source filtered)", len(got))
	}
	c := got[0]
	if c.ID != "swarm-1" {
		t.Errorf("campaign id = %q, want swarm-1", c.ID)
	}
	if c.SourceCount != 4 || !c.Coordinated {
		t.Errorf("sourceCount=%d coordinated=%v, want 4/true", c.SourceCount, c.Coordinated)
	}
	if c.Count != 4 {
		t.Errorf("count = %d, want 4", c.Count)
	}
}

func TestDetectCampaignsByTarget(t *testing.T) {
	now := time.Now().UTC()
	evs := []event.Event{
		evAt(event.KindDecoyHit, event.SeverityHigh, "a", "/shared", now),
		evAt(event.KindDecoyHit, event.SeverityHigh, "b", "/shared", now),
	}
	got := detectCampaigns(evs)
	if len(got) != 1 {
		t.Fatalf("campaigns = %d, want 1 (grouped by target)", len(got))
	}
	if got[0].SourceCount != 2 || got[0].Coordinated {
		t.Errorf("sourceCount=%d coordinated=%v, want 2/false", got[0].SourceCount, got[0].Coordinated)
	}
}

func TestTopDecoys(t *testing.T) {
	now := time.Now().UTC()
	evs := []event.Event{
		evAt(event.KindDecoyHit, event.SeverityHigh, "a", "/.env", now),
		evAt(event.KindDecoyHit, event.SeverityHigh, "b", "/.env", now),
		evAt(event.KindDecoyHit, event.SeverityMedium, "a", "/admin", now),
		evAt(event.KindProbe, event.SeverityLow, "a", "", now), // no target → skipped
	}
	got := topDecoys(evs)
	if len(got) != 2 {
		t.Fatalf("decoys = %d, want 2", len(got))
	}
	if got[0].Target != "/.env" || got[0].Count != 2 || got[0].SourceCount != 2 {
		t.Errorf("top decoy = %+v, want /.env count2 sources2", got[0])
	}
}

func TestSourceDrill(t *testing.T) {
	now := time.Now().UTC()
	evs := []event.Event{
		// recentEvents hands us newest-first; mirror that here.
		evAt(event.KindCanaryTriggered, event.SeverityCritical, "1.1.1.1", "/keys", now.Add(2*time.Second)),
		evAt(event.KindDecoyHit, event.SeverityHigh, "1.1.1.1", "/.env", now.Add(time.Second)),
		evAt(event.KindDecoyHit, event.SeverityLow, "1.1.1.1", "/.env", now),
		evAt(event.KindProbe, event.SeverityMedium, "2.2.2.2", "/a", now),
	}
	d := sourceDrill(evs, "1.1.1.1")
	if d.Count != 3 {
		t.Fatalf("count = %d, want 3", d.Count)
	}
	if len(d.Events) != 3 {
		t.Fatalf("events = %d, want 3", len(d.Events))
	}
	if d.Events[0].Kind != event.KindCanaryTriggered {
		t.Errorf("first event kind = %q, want canary (newest first preserved)", d.Events[0].Kind)
	}
	if d.MaxSeverity != "critical" {
		t.Errorf("max severity = %q, want critical", d.MaxSeverity)
	}
	if d.Targets["/.env"] != 2 {
		t.Errorf("target /.env count = %d, want 2", d.Targets["/.env"])
	}
	if d.FirstSeen != now || d.LastSeen != now.Add(2*time.Second) {
		t.Errorf("first/last = %v/%v, want %v/%v", d.FirstSeen, d.LastSeen, now, now.Add(2*time.Second))
	}
}

func TestSourceDrillUnknownIsEmptyNotNil(t *testing.T) {
	d := sourceDrill(nil, "nobody")
	if d.Count != 0 {
		t.Errorf("count = %d, want 0", d.Count)
	}
	if d.Events == nil || d.Kinds == nil || d.Targets == nil {
		t.Error("drill maps/slice must never be nil")
	}
}

func TestConfirmedBreaches(t *testing.T) {
	now := time.Now().UTC()
	evs := []event.Event{
		evAt(event.KindCanaryTriggered, event.SeverityHigh, "a", "/keys", now.Add(time.Second)),
		evAt(event.KindCanaryTriggered, event.SeverityHigh, "b", "/keys", now),
		evAt(event.KindDecoyHit, event.SeverityHigh, "a", "/.env", now), // not a breach
	}
	b := confirmedBreaches(evs)
	if b.Count != 2 {
		t.Fatalf("count = %d, want 2", b.Count)
	}
	if b.Sources != 2 {
		t.Errorf("sources = %d, want 2", b.Sources)
	}
	if b.Targets["/keys"] != 2 {
		t.Errorf("target /keys = %d, want 2", b.Targets["/keys"])
	}
	if b.LastSeen != now.Add(time.Second) {
		t.Errorf("last seen = %v, want %v", b.LastSeen, now.Add(time.Second))
	}
	if len(b.Recent) != 2 {
		t.Errorf("recent = %d, want 2", len(b.Recent))
	}
}

func TestConfirmedBreachesEmpty(t *testing.T) {
	b := confirmedBreaches(nil)
	if b.Count != 0 || b.Recent == nil || b.Targets == nil {
		t.Errorf("empty breaches = %+v, want zero count with non-nil fields", b)
	}
}

func TestAggregatesNeverNil(t *testing.T) {
	if topSources(nil) == nil {
		t.Error("topSources(nil) is nil, want empty slice")
	}
	if detectCampaigns(nil) == nil {
		t.Error("detectCampaigns(nil) is nil, want empty slice")
	}
	if topDecoys(nil) == nil {
		t.Error("topDecoys(nil) is nil, want empty slice")
	}
}
