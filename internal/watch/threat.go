package watch

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/Armur-Ai/heramba/internal/event"
)

// ThreatLevel is a coarse banding of the live threat score, for the gauge label
// and color. It exists so the top of the dashboard answers the only question an
// operator has in the first second: how bad is it right now?
type ThreatLevel int

const (
	ThreatCalm     ThreatLevel = iota // nothing of note
	ThreatGuarded                     // low-severity probing
	ThreatElevated                    // sustained or medium-severity activity
	ThreatHigh                        // high-severity or a forming swarm
	ThreatCritical                    // confirmed breach signal / active swarm
)

func (l ThreatLevel) String() string {
	switch l {
	case ThreatCalm:
		return "CALM"
	case ThreatGuarded:
		return "GUARDED"
	case ThreatElevated:
		return "ELEVATED"
	case ThreatHigh:
		return "HIGH"
	case ThreatCritical:
		return "CRITICAL"
	default:
		return "—"
	}
}

// Color is the accent the gauge and level badge render in.
func (l ThreatLevel) Color() lipgloss.Color {
	switch l {
	case ThreatCalm:
		return colGreen
	case ThreatGuarded:
		return colCyan
	case ThreatElevated:
		return colViolet
	case ThreatHigh:
		return colAmber
	case ThreatCritical:
		return colRed
	default:
		return colMuted
	}
}

// ThreatScore condenses the live state into a single 0..1 urgency number. It is
// a saturating weighted blend of the signals that matter, with two overrides:
//   - a triggered canary is a confirmed-breach signal (someone used a planted
//     token that only an intruder could have), so it floors the score high;
//   - an active swarm (one actor fanning across identities) floors it high too,
//     because that is precisely the pattern Heramba exists to surface.
func (s *State) ThreatScore() float64 {
	if s.Total == 0 {
		return 0
	}
	sev := severityFraction(s.HighestSeverity)
	rate := clamp01(s.RateNow() / 8.0)
	swarms := clamp01(float64(s.SwarmCount()) / 2.0)
	volume := clamp01(float64(s.DecoyHits()+s.EgressBlocks()+s.AgenticFlags()) / 20.0)

	score := 0.45*sev + 0.20*swarms + 0.20*rate + 0.15*volume

	if s.SwarmCount() > 0 {
		score = max64(score, 0.80)
	}
	if s.CanaryHits() > 0 {
		score = max64(score, 0.92)
	}
	return clamp01(score)
}

// ThreatLevel bands the current score.
func (s *State) ThreatLevel() ThreatLevel { return threatLevelOf(s.ThreatScore()) }

func threatLevelOf(score float64) ThreatLevel {
	switch {
	case score < 0.15:
		return ThreatCalm
	case score < 0.40:
		return ThreatGuarded
	case score < 0.60:
		return ThreatElevated
	case score < 0.85:
		return ThreatHigh
	default:
		return ThreatCritical
	}
}

// severityFraction maps a severity onto [0,1].
func severityFraction(sev event.Severity) float64 {
	switch sev {
	case event.SeverityLow:
		return 0.25
	case event.SeverityMedium:
		return 0.50
	case event.SeverityHigh:
		return 0.75
	case event.SeverityCritical:
		return 1.0
	default:
		return 0.0
	}
}

func max64(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
