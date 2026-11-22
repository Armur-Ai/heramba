package dashboard

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// demoBackedHandler wires a deterministic, seeded demo feed into a handler
// without the live ticker goroutine.
func demoBackedHandler() (http.Handler, *DemoFeed) {
	// Anchor the feed's clock to real "now" so the trailing-window timeline
	// (which ends at now) includes the synthesized events; the seeded RNG keeps
	// the content deterministic regardless of the clock.
	feed := NewDemoFeedAt(2024, time.Now().UTC())
	feed.Seed(500)
	h := Handler(WithEvents(feed.Recent), WithMetrics(feed.Metrics))
	return h, feed
}

func TestDemoBackedEndToEnd(t *testing.T) {
	h, _ := demoBackedHandler()

	// metrics
	var m metricsJSON
	mustJSON(t, h, "/api/metrics", &m)
	if m.Total < 500 || m.Sources == 0 {
		t.Errorf("metrics total=%d sources=%d, want populated", m.Total, m.Sources)
	}
	if m.Score.Count == 0 {
		t.Error("expected agentic-score samples in metrics")
	}

	// events
	var evs []json.RawMessage
	mustJSON(t, h, "/api/events?n=50", &evs)
	if len(evs) != 50 {
		t.Errorf("events len=%d, want 50", len(evs))
	}

	// timeline
	var tl Timeline
	mustJSON(t, h, "/api/timeline?window=15m", &tl)
	if len(tl.Points) == 0 {
		t.Error("timeline has no points")
	}
	sum := 0
	for _, p := range tl.Points {
		sum += p.Total
	}
	if sum == 0 {
		t.Error("timeline buckets are all empty for a populated feed")
	}

	// sources
	var srcs []SourceAgg
	mustJSON(t, h, "/api/sources", &srcs)
	if len(srcs) == 0 {
		t.Error("no sources aggregated")
	}

	// campaigns — the demo must surface a coordinated swarm
	var camps []Campaign
	mustJSON(t, h, "/api/campaigns", &camps)
	foundCoord := false
	for _, c := range camps {
		if c.Coordinated {
			foundCoord = true
		}
	}
	if !foundCoord {
		t.Error("expected a coordinated swarm campaign in demo data")
	}

	// decoys
	var decoys []DecoyAgg
	mustJSON(t, h, "/api/decoys", &decoys)
	if len(decoys) == 0 {
		t.Error("no decoys aggregated")
	}
}

func mustJSON(t *testing.T, h http.Handler, path string, v any) {
	t.Helper()
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, path, nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("%s: status %d", path, rr.Code)
	}
	if err := json.Unmarshal(rr.Body.Bytes(), v); err != nil {
		t.Fatalf("%s: decode: %v", path, err)
	}
}
