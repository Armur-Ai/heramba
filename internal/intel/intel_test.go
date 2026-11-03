package intel

import (
	"testing"

	"github.com/Armur-Ai/heramba/internal/alert"
	"github.com/Armur-Ai/heramba/internal/campaign/swarm"
	"github.com/Armur-Ai/heramba/internal/event"
	"github.com/Armur-Ai/heramba/internal/evidence"
)

func TestIngestorIsAnAlertSink(t *testing.T) {
	// It must satisfy alert.Sink so it can be fanned into the live edge.
	var _ alert.Sink = NewIngestor(evidence.NewStore(), "edge")
}

func TestIngestLiftsDetectionsIntoEvidence(t *testing.T) {
	ing := NewIngestor(evidence.NewStore(), "edge")
	_ = ing.Emit(event.Deception(event.KindDecoyHit, "10.0.0.1", "/.env", "probe"))
	_ = ing.Emit(event.New(event.KindAgenticTraffic, event.SeverityMedium, "10.0.0.1", "/admin", "agentic"))
	seen, lifted := ing.Stats()
	if seen != 2 || lifted != 2 {
		t.Fatalf("expected 2 seen/2 lifted, got %d/%d", seen, lifted)
	}
	ev, _, _, _ := ing.Store().Counts()
	if ev != 2 {
		t.Fatalf("expected 2 events in store, got %d", ev)
	}
}

func TestIngestNeverBreaksAlertPathOnBadEvent(t *testing.T) {
	ing := NewIngestor(evidence.NewStore(), "")
	// An event with an empty kind can't be lifted, but Emit must still succeed
	// (the other sinks must keep working).
	if err := ing.Emit(event.Event{}); err != nil {
		t.Fatalf("Emit must not error on an unliftable event: %v", err)
	}
	if _, lifted := ing.Stats(); lifted != 0 {
		t.Fatalf("an invalid event should not be lifted, got %d", lifted)
	}
}

func TestSummarizeReconstructsAndClassifies(t *testing.T) {
	ing := NewIngestor(evidence.NewStore(), "edge")
	// Two sources enumerating + tripping decoys → the summary should see actors
	// and classify coordination.
	for _, src := range []string{"192.0.2.1", "192.0.2.2"} {
		for _, p := range []string{"/api/users/1", "/api/users/2", "/admin", "/.env"} {
			if p == "/.env" {
				_ = ing.Emit(event.Deception(event.KindDecoyHit, src, p, "decoy"))
			} else {
				_ = ing.Emit(event.New(event.KindProbe, event.SeverityMedium, src, p, "probe").WithLabel("ua", "nuclei"))
			}
		}
	}
	s := ing.Summarize()
	if s.Events == 0 || s.Actors == 0 {
		t.Fatalf("summary should reconstruct events/actors, got %+v", s)
	}
	total := 0
	for _, n := range s.SwarmClasses {
		total += n
	}
	if total != s.Actors {
		t.Fatalf("every actor should be classified: classified %d of %d", total, s.Actors)
	}
	// sanity: swarm classes are from the known set
	for class := range s.SwarmClasses {
		switch class {
		case swarm.ClassSequential, swarm.ClassIndependent, swarm.ClassCoordinated, swarm.ClassLegitAutomation:
		default:
			t.Fatalf("unknown swarm class %q", class)
		}
	}
}
