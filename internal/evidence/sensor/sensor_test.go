package sensor

import (
	"errors"
	"testing"

	"github.com/Armur-Ai/heramba/internal/evidence"
)

// TestCrossSensorIncidentTimeline is the CP7 acceptance test: synthetic API,
// identity, and network events that share a source are fused into one
// evidence-backed, time-ordered incident timeline.
func TestCrossSensorIncidentTimeline(t *testing.T) {
	store := evidence.NewStore()
	p := NewPipeline(store)

	ip := "198.51.100.23"
	p.Add(&AppLogCollector{Source: StaticSource(
		`{"ts":"2026-03-01T10:00:02Z","ip":"198.51.100.23","method":"GET","path":"/api/users/1","status":200,"id":"app-1"}`)})
	p.Add(&AuthLogCollector{Source: StaticSource(
		`{"ts":"2026-03-01T10:00:00Z","ip":"198.51.100.23","subject":"svc-bot","result":"failure","id":"auth-1"}`)})
	p.Add(&NetworkCollector{Source: StaticSource(
		`{"ts":"2026-03-01T10:00:05Z","ip":"198.51.100.23","query":"evil.example.test","kind":"callback","id":"net-1"}`)})

	if n := p.Ingest(); n != 3 {
		t.Fatalf("expected 3 ingested events, got %d", n)
	}
	tl := p.IncidentTimeline(ip)
	if len(tl) != 3 {
		t.Fatalf("expected 3 timeline entries, got %d", len(tl))
	}
	// correct sensors, correct chronological order (auth 10:00:00 → app 10:00:02 → net 10:00:05)
	wantOrder := []string{"auth", "app", "network"}
	for i, w := range wantOrder {
		if tl[i].Sensor != w {
			t.Fatalf("timeline[%d] sensor = %q, want %q (order=%v)", i, tl[i].Sensor, w, sensors(tl))
		}
	}
}

func sensors(tl []TimelineEntry) []string {
	var s []string
	for _, e := range tl {
		s = append(s, e.Sensor)
	}
	return s
}

// TestDeduplication: the same underlying event reported twice (same sensor+ref)
// is counted once.
func TestDeduplication(t *testing.T) {
	store := evidence.NewStore()
	p := NewPipeline(store)
	blob := `{"ts":"2026-03-01T10:00:00Z","ip":"10.0.0.1","method":"GET","path":"/a","status":200,"id":"dup"}`
	p.Add(&AppLogCollector{Source: StaticSource(blob + "\n" + blob)})
	if n := p.Ingest(); n != 1 {
		t.Fatalf("expected 1 after dedup, got %d", n)
	}
	// a second Ingest of the same data adds nothing
	if n := p.Ingest(); n != 0 {
		t.Fatalf("re-ingest should add 0, got %d", n)
	}
	h := p.Health()
	if h[0].Duplicates == 0 {
		t.Fatal("duplicate count not recorded")
	}
}

// TestMissingSensorIsSafe: a failing collector marks itself unhealthy but does
// not block healthy collectors.
func TestMissingSensorIsSafe(t *testing.T) {
	store := evidence.NewStore()
	p := NewPipeline(store)
	p.Add(&failingCollector{})
	p.Add(&AppLogCollector{Source: StaticSource(
		`{"ts":"2026-03-01T10:00:00Z","ip":"10.0.0.2","method":"GET","path":"/ok","status":200,"id":"ok-1"}`)})

	if n := p.Ingest(); n != 1 {
		t.Fatalf("healthy collector should still deliver despite a failing one, got %d", n)
	}
	var sawDown, sawUp bool
	for _, h := range p.Health() {
		if h.Name == "failing" && !h.Healthy {
			sawDown = true
		}
		if h.Name == "app" && h.Healthy {
			sawUp = true
		}
	}
	if !sawDown || !sawUp {
		t.Fatalf("health not tracked correctly: %+v", p.Health())
	}
}

// TestBackpressureCap: a noisy sensor is capped per pass and the overflow is
// counted, not held.
func TestBackpressureCap(t *testing.T) {
	store := evidence.NewStore()
	p := NewPipeline(store)
	p.MaxPerPass = 2
	blob := ""
	for i := 0; i < 10; i++ {
		blob += `{"ts":"2026-03-01T10:00:0` + itoa(i) + `Z","ip":"10.0.0.3","method":"GET","path":"/p` + itoa(i) + `","status":200,"id":"b` + itoa(i) + `"}` + "\n"
	}
	p.Add(&AppLogCollector{Source: StaticSource(blob)})
	n := p.Ingest()
	if n != 2 {
		t.Fatalf("backpressure cap not applied: ingested %d, want 2", n)
	}
	if p.Dropped() == 0 {
		t.Fatal("dropped overflow not counted")
	}
}

type failingCollector struct{}

func (f *failingCollector) Name() string { return "failing" }
func (f *failingCollector) Collect() ([]evidence.SecurityEvent, error) {
	return nil, errors.New("sensor offline")
}

func itoa(n int) string { return string(rune('0' + n)) }
