package alert

import "github.com/Armur-Ai/heramba/internal/event"

// ev builds a simple at/above-floor event for wrapper tests.
func ev() event.Event {
	return event.New(event.KindProbe, event.SeverityHigh, "1.2.3.4", "/x", "d")
}

// sinkFunc adapts a function to the Sink interface for tests.
type sinkFunc func(event.Event) error

func (f sinkFunc) Emit(e event.Event) error { return f(e) }
