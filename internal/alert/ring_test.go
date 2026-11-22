package alert

import (
	"sync"
	"testing"

	"github.com/Armur-Ai/heramba/internal/event"
)

func TestRingSinkNewestFirstAndBounded(t *testing.T) {
	r := NewRing(3)
	for i := 0; i < 5; i++ {
		_ = r.Emit(event.New(event.KindProbe, event.SeverityLow, "", "/p", string(rune('a'+i))))
	}
	// Only the last 3 are kept.
	if r.Len() != 3 {
		t.Fatalf("len = %d, want 3 (bounded)", r.Len())
	}
	got := r.Recent(10)
	if len(got) != 3 {
		t.Fatalf("recent returned %d, want 3", len(got))
	}
	// Newest first: inserted details a,b,c,d,e -> newest are e,d,c.
	wantOrder := []string{"e", "d", "c"}
	for i, w := range wantOrder {
		if got[i].Detail != w {
			t.Errorf("recent[%d].Detail = %q, want %q (newest-first)", i, got[i].Detail, w)
		}
	}
}

func TestRingSinkRecentLimit(t *testing.T) {
	r := NewRing(10)
	for i := 0; i < 4; i++ {
		_ = r.Emit(event.New(event.KindDecoyHit, event.SeverityHigh, "", "/x", string(rune('0'+i))))
	}
	if got := r.Recent(2); len(got) != 2 || got[0].Detail != "3" || got[1].Detail != "2" {
		t.Fatalf("recent(2) wrong: %+v", got)
	}
	// n larger than held returns all.
	if got := r.Recent(100); len(got) != 4 {
		t.Fatalf("recent(100) = %d, want 4", len(got))
	}
}

func TestRingSinkEmpty(t *testing.T) {
	r := NewRing(5)
	if r.Len() != 0 {
		t.Fatalf("new ring should be empty, got %d", r.Len())
	}
	if got := r.Recent(3); len(got) != 0 {
		t.Fatalf("empty recent should be empty, got %v", got)
	}
}

func TestRingSinkZeroSizeClamps(t *testing.T) {
	r := NewRing(0) // clamps to 1
	_ = r.Emit(event.New(event.KindProbe, event.SeverityLow, "", "/a", "first"))
	_ = r.Emit(event.New(event.KindProbe, event.SeverityLow, "", "/b", "second"))
	got := r.Recent(10)
	if len(got) != 1 || got[0].Detail != "second" {
		t.Fatalf("size-1 ring should keep only the newest: %+v", got)
	}
}

func TestRingSinkConcurrent(t *testing.T) {
	r := NewRing(64)
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				_ = r.Emit(event.New(event.KindProbe, event.SeverityLow, "", "/c", "x"))
				_ = r.Recent(10)
			}
		}()
	}
	wg.Wait()
	if r.Len() != 64 {
		t.Fatalf("after concurrent load len = %d, want 64", r.Len())
	}
}
