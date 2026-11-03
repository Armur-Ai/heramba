package lab01

import (
	"testing"
	"time"

	"github.com/Armur-Ai/heramba/internal/classify"
	"github.com/Armur-Ai/heramba/internal/event"
	"github.com/Armur-Ai/heramba/internal/evidence"
	"github.com/Armur-Ai/heramba/labs/labkit"
)

// Lab 01 — Baseline telemetry and detection.
//
// Objective: see how Heramba scores a source as agentic from transparent
// behavioral signals (no LLM), and how a flat detection is lifted into the
// normalized evidence model with provenance. Run: `go test -v ./labs/01-...`.
func TestLab01(t *testing.T) {
	labkit.Banner(t, "Lab 01 — Baseline telemetry and detection")

	labkit.Step(t, 1, "classify an obvious tool client vs a browser (deterministic)")
	c := classify.New()
	botUA := "curl/8.4.0"
	bot := c.Classify(classify.Observation{
		Source: "10.0.0.9", UserAgent: botUA, Method: "GET", Path: "/",
		Headers: map[string]string{"Accept": "*/*"}, Time: time.Now(),
	})
	human := c.Classify(classify.Observation{
		Source:    "10.0.0.8",
		UserAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36",
		Method:    "GET", Path: "/",
		Headers: map[string]string{"Accept": "text/html,application/xhtml+xml", "Accept-Language": "en-US"},
		Time:    time.Now(),
	})
	labkit.Observe(t, "tool score=%.2f (%s), browser score=%.2f (%s)", bot.Score, bot.Label, human.Score, human.Label)
	labkit.Require(t, bot.Score > human.Score, "a tool client should score more agentic than a browser")
	labkit.Require(t, len(bot.Signals) > 0, "the verdict must list the signals that fired (explainability)")

	labkit.Step(t, 2, "a decoy hit is a zero-false-positive detection (confidence 1.0)")
	hit := event.Deception(event.KindDecoyHit, "10.0.0.9", "/.env", "secret-file probe")
	labkit.Require(t, hit.Confidence == 1.0, "a decoy hit must carry confidence 1.0 by construction")

	labkit.Step(t, 3, "lift the flat detection into normalized evidence with provenance")
	se := evidence.FromDetection(hit, "edge", "req-42")
	labkit.Require(t, se.Validate() == nil, "the lifted SecurityEvent must validate")
	labkit.Require(t, se.Provenance.Sensor == "edge", "provenance must name the sensor")
	labkit.Require(t, se.Stage == evidence.StageDeceptionTouch, "a decoy hit maps to the deception-touch stage")
	labkit.Observe(t, "evidence %s: kind=%s stage=%s conf=%.1f sensor=%s", se.ID, se.Kind, se.Stage, se.Confidence, se.Provenance.Sensor)

	labkit.Step(t, 4, "persist and reload — evidence round-trips")
	store := evidence.NewStore()
	labkit.Require(t, store.PutEvent(se) == nil, "storing valid evidence should succeed")
	ev, _, _, _ := store.Counts()
	labkit.Require(t, ev == 1, "exactly one event should be stored")
}
