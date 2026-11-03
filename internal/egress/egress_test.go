package egress

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/Armur-Ai/heramba/internal/event"
)

type capSink struct{ events []event.Event }

func (c *capSink) Emit(e event.Event) error { c.events = append(c.events, e); return nil }

// stubDial records that the base dialer was reached; returns no real conn.
func stubDial(reached *bool) DialFunc {
	return func(context.Context, string, string) (net.Conn, error) {
		*reached = true
		return nil, nil
	}
}

func TestBlocksDisallowedHostAndEmits(t *testing.T) {
	sink := &capSink{}
	var reached bool
	fw := New(true, []string{"a.example"}, sink, stubDial(&reached))

	_, err := fw.DialContext(context.Background(), "tcp", "evil.example:443")
	var be *BlockedError
	if !errors.As(err, &be) {
		t.Fatalf("expected BlockedError, got %v", err)
	}
	if reached {
		t.Fatal("base dialer must not be reached for a blocked host")
	}
	if len(sink.events) != 1 || sink.events[0].Kind != event.KindEgressBlocked {
		t.Fatalf("expected one egress_blocked event, got %+v", sink.events)
	}
}

func TestAllowsAllowlistedHost(t *testing.T) {
	var reached bool
	fw := New(true, []string{"a.example"}, nil, stubDial(&reached))
	if _, err := fw.DialContext(context.Background(), "tcp", "a.example:443"); err != nil {
		t.Fatalf("allowed host should dial: %v", err)
	}
	if !reached {
		t.Fatal("base dialer should be reached for an allowed host")
	}
}

func TestDisabledIsPassThrough(t *testing.T) {
	var reached bool
	fw := New(false, nil, nil, stubDial(&reached))
	if _, err := fw.DialContext(context.Background(), "tcp", "anything.example:80"); err != nil {
		t.Fatalf("disabled firewall must pass through: %v", err)
	}
	if !reached {
		t.Fatal("disabled firewall should still dial")
	}
}
