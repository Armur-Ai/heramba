package alert

import (
	"testing"

	"github.com/Armur-Ai/heramba/internal/event"
)

func TestCountingSinkTalliesAndDelegates(t *testing.T) {
	inner := &countSink{}
	c := Count(inner)
	_ = c.Emit(event.Deception(event.KindDecoyHit, "", "/a", ""))
	_ = c.Emit(event.Deception(event.KindDecoyHit, "", "/b", ""))
	_ = c.Emit(event.New(event.KindAgenticTraffic, event.SeverityMedium, "", "", ""))

	if c.Total() != 3 {
		t.Fatalf("total = %d, want 3", c.Total())
	}
	if inner.n != 3 {
		t.Fatalf("inner should receive all events, got %d", inner.n)
	}
	snap := c.Snapshot()
	if len(snap) != 2 || snap[0].Kind != event.KindDecoyHit || snap[0].Count != 2 {
		t.Fatalf("snapshot wrong: %+v", snap)
	}
}

func TestCountNilInner(t *testing.T) {
	c := Count(nil)
	_ = c.Emit(event.New(event.KindProbe, event.SeverityInfo, "", "", ""))
	if c.Total() != 1 {
		t.Fatal("nil inner should still count")
	}
}
