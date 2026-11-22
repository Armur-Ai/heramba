package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Armur-Ai/heramba/internal/alert"
	"github.com/Armur-Ai/heramba/internal/event"
)

func writeCapture(t *testing.T, events []event.Event) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "capture.jsonl")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	sink := alert.NewJSONL(f)
	for _, e := range events {
		if err := sink.Emit(e); err != nil {
			t.Fatal(err)
		}
	}
	_ = f.Close()
	return p
}

func TestReadCaptureRoundTrips(t *testing.T) {
	want := []event.Event{
		event.Deception(event.KindDecoyHit, "10.0.0.1", "/.env", "probe"),
		event.New(event.KindProbe, event.SeverityMedium, "10.0.0.2", "/scan", "sweep"),
	}
	p := writeCapture(t, want)
	got, err := readCapture(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Fatalf("read %d events, want %d", len(got), len(want))
	}
	if got[0].Target != "/.env" || got[1].Kind != event.KindProbe {
		t.Errorf("round-trip mismatch: %+v", got)
	}
}

func TestReadCaptureMissingFile(t *testing.T) {
	if _, err := readCapture(filepath.Join(t.TempDir(), "nope.jsonl")); err == nil {
		t.Fatal("expected error reading missing capture")
	}
}

func TestReplayThroughUnpacedFeedsEverySink(t *testing.T) {
	events := []event.Event{
		event.Deception(event.KindDecoyHit, "10.0.0.1", "/.env", ""),
		event.Deception(event.KindDecoyHit, "10.0.0.1", "/.git/config", ""),
		event.New(event.KindAgenticTraffic, event.SeverityMedium, "10.0.0.2", "", ""),
	}
	counting := alert.Count(nil)
	n := replayThrough(context.Background(), counting, events, 0)
	if n != 3 {
		t.Fatalf("emitted %d, want 3", n)
	}
	if counting.Total() != 3 {
		t.Fatalf("sink saw %d, want 3", counting.Total())
	}
	snap := counting.Snapshot()
	if snap[0].Kind != event.KindDecoyHit || snap[0].Count != 2 {
		t.Errorf("tally wrong: %+v", snap)
	}
}

func TestReplayThroughPacedDeliversAll(t *testing.T) {
	events := []event.Event{
		event.New(event.KindProbe, event.SeverityLow, "a", "/1", ""),
		event.New(event.KindProbe, event.SeverityLow, "b", "/2", ""),
	}
	counting := alert.Count(nil)
	// High speed so timestamp pacing collapses to the minimum gap; must still
	// deliver every event.
	n := replayThrough(context.Background(), counting, events, 1000)
	if n != 2 || counting.Total() != 2 {
		t.Fatalf("paced replay emitted %d (sink %d), want 2", n, counting.Total())
	}
}

func TestReplayThroughUnpacedStopsOnCancel(t *testing.T) {
	events := make([]event.Event, 100)
	for i := range events {
		events[i] = event.New(event.KindProbe, event.SeverityLow, "a", "/x", "")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled
	counting := alert.Count(nil)
	n := replayThrough(ctx, counting, events, 0)
	if n != 0 {
		t.Fatalf("cancelled replay should emit nothing, got %d", n)
	}
}
