package dashboard

import (
	"testing"

	"github.com/Armur-Ai/heramba/internal/event"
)

func TestBrokerFanOut(t *testing.T) {
	b := NewBroker()
	s1 := b.subscribe()
	s2 := b.subscribe()
	if b.Subscribers() != 2 {
		t.Fatalf("subscribers = %d, want 2", b.Subscribers())
	}
	e := event.New(event.KindDecoyHit, event.SeverityHigh, "1.2.3.4", "/.env", "d")
	b.Publish(EventFrame(e))
	for i, s := range []*subscriber{s1, s2} {
		select {
		case f := <-s.ch:
			if f.Type != "event" || f.Event == nil || f.Event.Source != "1.2.3.4" {
				t.Errorf("sub %d got %+v, want decoy event", i, f)
			}
		default:
			t.Errorf("sub %d received nothing", i)
		}
	}
}

func TestBrokerUnsubscribe(t *testing.T) {
	b := NewBroker()
	s := b.subscribe()
	b.unsubscribe(s)
	if b.Subscribers() != 0 {
		t.Fatalf("subscribers after unsubscribe = %d, want 0", b.Subscribers())
	}
	if _, ok := <-s.ch; ok {
		t.Error("channel should be closed after unsubscribe")
	}
	// unsubscribe is idempotent (must not panic / double-close).
	b.unsubscribe(s)
	// publishing to nobody must not block or panic.
	b.Publish(StreamFrame{Type: "metrics"})
}

func TestBrokerDropsOnSlowSubscriber(t *testing.T) {
	b := NewBroker()
	s := b.subscribe()
	// Overfill well past the buffer; Publish must never block.
	for i := 0; i < subBuffer*4; i++ {
		b.Publish(StreamFrame{Type: "event"})
	}
	// The buffer holds at most subBuffer frames; the rest were dropped.
	got := 0
	for {
		select {
		case <-s.ch:
			got++
			continue
		default:
		}
		break
	}
	if got != subBuffer {
		t.Errorf("buffered frames = %d, want %d (drop-on-slow)", got, subBuffer)
	}
}

func TestBrokerClose(t *testing.T) {
	b := NewBroker()
	s := b.subscribe()
	b.Close()
	if b.Subscribers() != 0 {
		t.Errorf("subscribers after close = %d, want 0", b.Subscribers())
	}
	if _, ok := <-s.ch; ok {
		t.Error("subscriber channel should be closed after Close")
	}
	b.Publish(StreamFrame{Type: "event"}) // no-op, must not panic
	b.Close()                             // idempotent
}
