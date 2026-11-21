package watch

import (
	"context"
	"testing"
	"time"

	"github.com/Armur-Ai/heramba/internal/event"
)

func TestReplayEmitsAllInOrder(t *testing.T) {
	base := time.Now()
	in := []event.Event{
		mkAt(base.Add(2*time.Second), event.KindCanaryTriggered),
		mkAt(base.Add(0), event.KindDecoyHit),
		mkAt(base.Add(1*time.Second), event.KindAgenticTraffic),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var got []event.Kind
	for e := range Replay(ctx, in, 100) { // fast: gaps clamped to minGap
		got = append(got, e.Kind)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 events, got %d", len(got))
	}
	// Replay sorts by timestamp, so decoy (t0) must precede canary (t2).
	if got[0] != event.KindDecoyHit || got[2] != event.KindCanaryTriggered {
		t.Fatalf("events not time-ordered: %v", got)
	}
}

func TestReplayStopsOnCancel(t *testing.T) {
	base := time.Now()
	in := []event.Event{
		mkAt(base, event.KindDecoyHit),
		mkAt(base.Add(10*time.Second), event.KindCanaryTriggered),
	}
	ctx, cancel := context.WithCancel(context.Background())
	ch := Replay(ctx, in, 1)
	<-ch // first event arrives immediately (gap 0)
	cancel()
	select {
	case _, ok := <-ch:
		if ok {
			// draining the (now cancelled) second event is fine; channel must close next.
			_, ok = <-ch
		}
		if ok {
			t.Fatal("channel should close after cancel")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Replay did not stop on cancel")
	}
}

func TestReplayHandlesZeroTimestamps(t *testing.T) {
	in := []event.Event{
		{Kind: event.KindProbe},
		{Kind: event.KindProbe},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var n int
	for range Replay(ctx, in, 100) {
		n++
	}
	if n != 2 {
		t.Fatalf("zero-timestamp events should still replay; got %d", n)
	}
}

func mkAt(ts time.Time, k event.Kind) event.Event {
	e := event.New(k, event.SeverityHigh, "10.0.0.5", "/x", "d")
	e.Time = ts
	return e
}
