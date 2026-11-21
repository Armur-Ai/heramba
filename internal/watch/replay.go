package watch

import (
	"context"
	"sort"
	"time"

	"github.com/Armur-Ai/heramba/internal/event"
)

// Replay streams a recorded slice of events as if they were arriving live,
// preserving the shape of the attack: events are paced by the gaps between
// their timestamps, divided by speed. speed=1 replays in real time; speed=8
// plays an eight-minute capture in one minute; speed<=0 defaults to 1.
//
// This is what makes the demo gif feel alive — a recorded attack against the
// deception app unfolds on the dashboard with the same rhythm it originally
// had, instead of dumping every row at once. Gaps are clamped so a long idle
// stretch in the capture never stalls the demo, and non-monotonic or missing
// timestamps fall back to a fixed cadence.
func Replay(ctx context.Context, events []event.Event, speed float64) <-chan event.Event {
	if speed <= 0 {
		speed = 1
	}
	const (
		maxGap   = 1200 * time.Millisecond // clamp idle stretches
		fixedGap = 400 * time.Millisecond  // used when timestamps are unusable
		minGap   = 20 * time.Millisecond   // keep motion visible
	)
	ordered := make([]event.Event, len(events))
	copy(ordered, events)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].Time.Before(ordered[j].Time)
	})

	out := make(chan event.Event, 16)
	go func() {
		defer close(out)
		var prev time.Time
		for i, e := range ordered {
			gap := fixedGap
			if i > 0 && !e.Time.IsZero() && !prev.IsZero() && e.Time.After(prev) {
				gap = e.Time.Sub(prev)
			}
			if i == 0 {
				gap = 0
			}
			scaled := time.Duration(float64(gap) / speed)
			if i > 0 {
				scaled = clamp(scaled, minGap, maxGap)
			}
			if scaled > 0 && !sleep(ctx, scaled) {
				return
			}
			select {
			case out <- e:
			case <-ctx.Done():
				return
			}
			prev = e.Time
		}
	}()
	return out
}

func clamp(d, lo, hi time.Duration) time.Duration {
	if d < lo {
		return lo
	}
	if d > hi {
		return hi
	}
	return d
}
