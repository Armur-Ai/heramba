package watch

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// meter draws a horizontal bar `width` columns wide, filled left-to-right in
// proportion to frac (clamped to [0,1]). The filled portion uses a solid block
// and the remainder a light track glyph, so the bar reads clearly even without
// color. The returned string is always exactly `width` columns wide.
func meter(frac float64, width int) string {
	if width <= 0 {
		return ""
	}
	frac = clamp01(frac)
	on := int(frac*float64(width) + 0.5)
	if on > width {
		on = width
	}
	return strings.Repeat("█", on) + strings.Repeat("░", width-on)
}

// coloredMeter is meter() with the filled portion painted in fg and the empty
// track in a muted color. The two runs are styled separately so no ANSI code is
// ever cut mid-sequence.
func coloredMeter(frac float64, width int, fg lipgloss.Color) string {
	if width <= 0 {
		return ""
	}
	frac = clamp01(frac)
	on := int(frac*float64(width) + 0.5)
	if on > width {
		on = width
	}
	filled := lipgloss.NewStyle().Foreground(fg).Render(strings.Repeat("█", on))
	track := lipgloss.NewStyle().Foreground(colMuted).Render(strings.Repeat("░", width-on))
	return filled + track
}

// clamp01 constrains x to the unit interval.
func clamp01(x float64) float64 {
	switch {
	case x < 0:
		return 0
	case x > 1:
		return 1
	default:
		return x
	}
}
