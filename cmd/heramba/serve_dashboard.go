package main

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Armur-Ai/heramba/internal/alert"
	"github.com/Armur-Ai/heramba/internal/config"
	"github.com/Armur-Ai/heramba/internal/dashboard"
	"github.com/Armur-Ai/heramba/internal/decoy"
	"github.com/Armur-Ai/heramba/internal/edge"
)

// liveControl backs the web dashboard's control panel against a RUNNING edge.
// Unlike the read-only viewer (`heramba dashboard`), posture changes made here
// take effect: they build a modified config and atomically swap the engine's
// detection surface via Engine.Reload — the same hot-swap SIGHUP uses — so mode
// and decoy-preset changes apply live without dropping a connection.
//
// It is the single source of truth for the edge's current config, so a
// dashboard change and a SIGHUP reload never clobber each other: the SIGHUP
// handler feeds the reloaded config back in via setConfig.
type liveControl struct {
	mu     sync.Mutex
	cfg    config.Config
	engine *edge.Engine
}

func newLiveControl(cfg config.Config, engine *edge.Engine) *liveControl {
	return &liveControl{cfg: cfg, engine: engine}
}

// Mode returns the edge's current posture (for the dashboard metrics).
func (lc *liveControl) Mode() string {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	return string(lc.cfg.Mode)
}

// setConfig records a config applied out-of-band (e.g. a SIGHUP reload) so the
// dashboard's view and subsequent edits build on current state.
func (lc *liveControl) setConfig(cfg config.Config) {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	lc.cfg = cfg
}

// SetMode flips the edge between observe and protect, live.
func (lc *liveControl) SetMode(mode string) error {
	m := config.Mode(strings.ToLower(strings.TrimSpace(mode)))
	if m != config.ModeObserve && m != config.ModeProtect {
		return fmt.Errorf("unknown mode %q (want observe|protect)", mode)
	}
	return lc.apply(func(c *config.Config) { c.Mode = m })
}

// ApplyDecoyPreset swaps the active framework decoy preset, live.
func (lc *liveControl) ApplyDecoyPreset(preset string) error {
	if _, ok := decoy.GetPreset(preset); !ok {
		return fmt.Errorf("unknown preset %q", preset)
	}
	return lc.apply(func(c *config.Config) { c.Decoys.Preset = preset })
}

// SetPaused is deliberately unsupported: a production edge must not expose a
// "stop defending" button. Switching to observe mode is the supported way to
// stop inline action.
func (lc *liveControl) SetPaused(bool) error {
	return errors.New("pausing is not supported on a live edge — switch to observe mode to stop inline action")
}

// Clear is unsupported on a live edge: session counters are the running audit
// tally and are not resettable from the UI.
func (lc *liveControl) Clear() error {
	return errors.New("clearing session counters on a live edge is not supported")
}

// apply mutates a copy of the current config, validates it, hot-swaps the engine
// to it, and commits it only if the swap succeeds.
func (lc *liveControl) apply(mutate func(*config.Config)) error {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	next := lc.cfg
	mutate(&next)
	if err := next.Validate(); err != nil {
		return err
	}
	if err := lc.engine.Reload(next, nil); err != nil { // nil = keep the engine's sink
		return err
	}
	lc.cfg = next
	return nil
}

// buildDashboardMetrics returns the MetricsFunc the live dashboard reads, drawn
// from the same counting sink and score histogram that feed /metrics, with the
// live posture from lc.
func buildDashboardMetrics(lc *liveControl, counting *alert.CountingSink, engine *edge.Engine, start time.Time) dashboard.MetricsFunc {
	return func() dashboard.Metrics {
		byKind := map[string]int{}
		for _, t := range counting.Snapshot() {
			byKind[string(t.Kind)] = t.Count
		}
		hs := engine.ScoreSnapshot()
		return dashboard.Metrics{
			Mode:       lc.Mode(),
			Total:      counting.Total(),
			ByKind:     byKind,
			BySeverity: counting.BySeverity(),
			Sources:    counting.Sources(),
			Uptime:     time.Since(start),
			Score: dashboard.ScoreHistogram{
				Bounds:    hs.Bounds,
				CumCounts: hs.CumulativeCounts,
				Sum:       hs.Sum,
				Count:     hs.Count,
			},
		}
	}
}
