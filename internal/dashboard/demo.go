package dashboard

import (
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/Armur-Ai/heramba/internal/event"
)

// DemoFeed synthesizes a realistic live attack stream so the dashboard runs
// standalone — decoy hits, canary triggers, agentic traffic, egress blocks and
// coordinated multi-IP swarms. It implements both event and metrics sources.
// It is safe for concurrent use and deterministic when seeded.
type DemoFeed struct {
	mu     sync.Mutex
	rnd    *rand.Rand
	events []event.Event // chronological, oldest first
	now    time.Time     // the feed's notion of "now" (advances on Tick)
	cap    int
}

// demoCap bounds the retained history.
const demoCap = 1500

// swarm IPs that act in concert against a single high-value canary — the
// coordinated campaign the swarm panel is meant to surface.
var demoSwarmIPs = []string{
	"45.146.165.11", "45.146.165.12", "45.146.165.37",
	"185.220.101.4", "185.220.101.9",
}

// background noise IPs acting independently.
var demoNoiseIPs = []string{
	"203.0.113.7", "198.51.100.23", "192.0.2.88", "198.51.100.5",
	"203.0.113.44", "141.98.11.2", "80.94.92.60", "193.32.162.19",
}

// decoy targets touched by attackers.
var demoDecoys = []string{
	"/admin/backup.sql", "/.env", "/api/internal/keys", "/wp-admin/",
	"/.git/config", "/actuator/env", "/debug/pprof/", "/api/v1/users/export",
}

// the swarm's shared objective.
const demoSwarmTarget = "/api/internal/keys"

// NewDemoFeed returns a demo feed seeded from the current time, ready to Tick.
func NewDemoFeed() *DemoFeed {
	return NewDemoFeedAt(time.Now().UnixNano(), time.Now().UTC())
}

// NewDemoFeedAt returns a demo feed with a fixed seed and clock — deterministic,
// for tests and reproducible demos.
func NewDemoFeedAt(seed int64, now time.Time) *DemoFeed {
	return &DemoFeed{
		rnd: rand.New(rand.NewSource(seed)),
		now: now.UTC(),
		cap: demoCap,
	}
}

// Seed backfills n events spread across the trailing window ending at the
// feed's current clock, so the dashboard opens with a populated history.
func (d *DemoFeed) Seed(n int) {
	if n <= 0 {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	window := 15 * time.Minute
	start := d.now.Add(-window)
	for i := 0; i < n; i++ {
		t := start.Add(time.Duration(int64(i) * int64(window) / int64(n)))
		d.append(d.gen(t))
	}
}

// Tick advances the feed's clock by dt and appends a few fresh events, as if a
// live stream arrived. It returns the events it generated.
func (d *DemoFeed) Tick(dt time.Duration) []event.Event {
	d.mu.Lock()
	defer d.mu.Unlock()
	if dt <= 0 {
		dt = time.Second
	}
	d.now = d.now.Add(dt)
	k := 1 + d.rnd.Intn(3)
	out := make([]event.Event, 0, k)
	for i := 0; i < k; i++ {
		e := d.gen(d.now.Add(-time.Duration(d.rnd.Intn(int(dt) + 1))))
		d.append(e)
		out = append(out, e)
	}
	return out
}

// append adds an event and trims to capacity.
func (d *DemoFeed) append(e event.Event) {
	d.events = append(d.events, e)
	if len(d.events) > d.cap {
		d.events = d.events[len(d.events)-d.cap:]
	}
}

// gen synthesizes one event at time t using the feed's RNG.
func (d *DemoFeed) gen(t time.Time) event.Event {
	// ~1 in 4 events belongs to the coordinated swarm.
	if d.rnd.Intn(4) == 0 {
		src := demoSwarmIPs[d.rnd.Intn(len(demoSwarmIPs))]
		e := event.Deception(event.KindCanaryTriggered, src, demoSwarmTarget,
			"planted canary token replayed from rotating infrastructure")
		e.Time = t
		e.Confidence = 1.0
		return e.WithLabel("campaign", "swarm-aurora").WithLabel("asn", "AS-REBEL")
	}

	switch d.rnd.Intn(6) {
	case 0:
		src := demoNoiseIPs[d.rnd.Intn(len(demoNoiseIPs))]
		tgt := demoDecoys[d.rnd.Intn(len(demoDecoys))]
		e := event.Deception(event.KindDecoyHit, src, tgt, "request touched decoy endpoint")
		e.Time = t
		return e
	case 1:
		src := demoNoiseIPs[d.rnd.Intn(len(demoNoiseIPs))]
		score := 0.55 + d.rnd.Float64()*0.45
		sev := event.SeverityMedium
		if score > 0.85 {
			sev = event.SeverityHigh
		}
		e := event.New(event.KindAgenticTraffic, sev, src, "/api/v1/search",
			fmt.Sprintf("behavioral classifier score %.2f — non-human cadence", score))
		e.Time = t
		e.Confidence = score
		return e.WithLabel("score", fmt.Sprintf("%.2f", score))
	case 2:
		src := demoNoiseIPs[d.rnd.Intn(len(demoNoiseIPs))]
		e := event.New(event.KindEgressBlocked, event.SeverityHigh, src, "http://exfil.evil.example/c2",
			"out-of-policy callback denied")
		e.Time = t
		e.Confidence = 0.95
		return e
	case 3:
		src := demoNoiseIPs[d.rnd.Intn(len(demoNoiseIPs))]
		e := event.New(event.KindRateLimited, event.SeverityLow, src, "/api/v1/login",
			"source exceeded request-rate budget")
		e.Time = t
		e.Confidence = 0.7
		return e
	case 4:
		src := demoNoiseIPs[d.rnd.Intn(len(demoNoiseIPs))]
		tgt := demoDecoys[d.rnd.Intn(len(demoDecoys))]
		e := event.New(event.KindProbe, event.SeverityMedium, src, tgt,
			"suspicious probing of real surface")
		e.Time = t
		e.Confidence = 0.6
		return e
	default:
		src := demoNoiseIPs[d.rnd.Intn(len(demoNoiseIPs))]
		e := event.New(event.KindCounterDeployed, event.SeverityInfo, src, "/api/v1/search",
			"counter-agent prompt decoy injected")
		e.Time = t
		e.Confidence = 0.5
		return e
	}
}

// Recent returns up to n events, newest first (the EventsFunc shape).
func (d *DemoFeed) Recent(n int) []event.Event {
	d.mu.Lock()
	defer d.mu.Unlock()
	if n <= 0 || len(d.events) == 0 {
		return []event.Event{}
	}
	if n > len(d.events) {
		n = len(d.events)
	}
	out := make([]event.Event, 0, n)
	for i := len(d.events) - 1; i >= 0 && len(out) < n; i-- {
		out = append(out, d.events[i])
	}
	return out
}

// Metrics computes a snapshot over the retained history (the MetricsFunc shape).
func (d *DemoFeed) Metrics() Metrics {
	d.mu.Lock()
	defer d.mu.Unlock()
	m := Metrics{
		Mode:       "protect",
		ByKind:     map[string]int{},
		BySeverity: map[string]int{},
		Total:      len(d.events),
	}
	bounds := []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9}
	cum := make([]uint64, len(bounds)+1)
	srcSet := map[string]struct{}{}
	var sum float64
	var count uint64
	for _, e := range d.events {
		m.ByKind[string(e.Kind)]++
		m.BySeverity[sevLabel(e)]++
		if e.Source != "" {
			srcSet[e.Source] = struct{}{}
		}
		if e.Kind == event.KindAgenticTraffic {
			s := e.Confidence
			sum += s
			count++
			placed := false
			for i, b := range bounds {
				if s <= b {
					for j := i; j < len(cum); j++ {
						cum[j]++
					}
					placed = true
					break
				}
			}
			if !placed {
				cum[len(cum)-1]++
			}
		}
	}
	m.Sources = len(srcSet)
	if count > 0 {
		m.Score = ScoreHistogram{Bounds: bounds, CumCounts: cum, Sum: sum, Count: count}
	}
	return m
}

// DemoHandler builds a dashboard handler backed by a live demo feed. It seeds a
// backlog and starts a background goroutine that appends fresh events, so the
// page streams immediately. Each generated event is also published to a broker
// that drives GET /api/stream, so the live stream reflects the synthetic attack
// traffic the instant it is produced. Intended for `heramba dashboard --demo`.
func DemoHandler(opts ...Option) http.Handler {
	feed := NewDemoFeed()
	feed.Seed(400)
	broker := NewBroker()
	base := []Option{WithEvents(feed.Recent), WithMetrics(feed.Metrics), WithBroker(broker)}
	h := Handler(append(base, opts...)...)
	go func() {
		t := time.NewTicker(900 * time.Millisecond)
		defer t.Stop()
		for range t.C {
			for _, e := range feed.Tick(900 * time.Millisecond) {
				broker.Publish(EventFrame(e))
			}
			broker.Publish(StreamFrame{Type: "metrics", Metrics: ptr(feed.metricsPayload())})
		}
	}()
	return h
}

// metricsPayload adapts the demo feed's snapshot to the stream wire shape,
// mirroring server.metricsPayload's never-nil-map guarantee (uptime is left
// zero — the live handler owns the real uptime).
func (d *DemoFeed) metricsPayload() metricsJSON {
	m := d.Metrics()
	if m.ByKind == nil {
		m.ByKind = map[string]int{}
	}
	if m.BySeverity == nil {
		m.BySeverity = map[string]int{}
	}
	return metricsJSON{
		Mode:       m.Mode,
		Total:      m.Total,
		ByKind:     m.ByKind,
		BySeverity: m.BySeverity,
		Sources:    m.Sources,
		Score:      m.Score,
	}
}
