package coord

import (
	"bytes"
	"testing"

	"github.com/Armur-Ai/heramba/internal/research"
)

func TestH4SweepMeasuresDegradation(t *testing.T) {
	rep := RunH4(research.DefaultProtocol(), DefaultParams(), "test")
	if len(rep.Sensors) != 4 {
		t.Fatalf("expected 4 sensors, got %d", len(rep.Sensors))
	}
	if rep.FullRecall.Point < 1.0 {
		t.Fatalf("full detector should recall every coordinated cohort, got %.3f", rep.FullRecall.Point)
	}
	byName := map[string]SensorResult{}
	for _, s := range rep.Sensors {
		byName[s.MissingSensor] = s
	}
	// Measured reality for labor-dividing swarms: the redundant signals
	// (target-overlap and decoy contact) degrade gracefully — removing them keeps
	// recall at the full level — while the load-bearing signals (synchrony and the
	// division-of-labor graph) collapse recall when removed. This is the honest,
	// quantified H4 answer: the detector needs synchrony + the applicable
	// co-targeting signal; it is not degradable to a single missing sensor for a
	// given swarm type.
	for _, name := range []string{"target-overlap (shared)", "deception (decoy contact)"} {
		s := byName[name]
		if !s.Graceful || s.Recall.Point < rep.FullRecall.Point {
			t.Errorf("missing %q should be redundant (graceful), got recall %.3f", name, s.Recall.Point)
		}
	}
	for _, name := range []string{"timing (synchrony)", "graph (division of labor)"} {
		s := byName[name]
		t.Logf("missing %q -> recall %.3f (floor %.3f) load_bearing=%v", name, s.Recall.Point, rep.FloorRecall.Point, s.LoadBearing)
		if !s.LoadBearing {
			t.Errorf("%q is expected to be load-bearing for divided-labor swarms, got recall %.3f", name, s.Recall.Point)
		}
	}
}

func TestH4ReportRenders(t *testing.T) {
	rep := RunH4(research.DefaultProtocol(), DefaultParams(), "test")
	var md, js bytes.Buffer
	if err := rep.WriteMarkdown(&md); err != nil {
		t.Fatal(err)
	}
	if err := rep.WriteJSON(&js); err != nil {
		t.Fatal(err)
	}
	if md.Len() == 0 || js.Len() == 0 {
		t.Fatal("empty report")
	}
}
