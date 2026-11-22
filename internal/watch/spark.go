package watch

import "strings"

// sparkBlocks are the eight partial-block glyphs used to draw a sparkline, from
// lowest to highest. A zero value renders as the lowest block so an all-quiet
// series still reads as a flat baseline rather than empty space.
var sparkBlocks = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// sparkline renders the tail of a numeric series as a compact unicode bar
// chart exactly `width` columns wide. Values are scaled against the series max
// so the shape is visible regardless of absolute magnitude. When there are
// fewer samples than width, the line is left-padded with the baseline glyph so
// it still fills (and right-aligns to "now"). A nil/empty series or width<=0
// yields an empty string.
func sparkline(values []float64, width int) string {
	if width <= 0 || len(values) == 0 {
		return ""
	}
	// Keep only the most recent `width` samples.
	if len(values) > width {
		values = values[len(values)-width:]
	}
	max := 0.0
	for _, v := range values {
		if v > max {
			max = v
		}
	}
	var b strings.Builder
	// Left-pad with the baseline block so the sparkline is always full width.
	for i := 0; i < width-len(values); i++ {
		b.WriteRune(sparkBlocks[0])
	}
	for _, v := range values {
		b.WriteRune(sparkBlock(v, max))
	}
	return b.String()
}

// sparkBlock maps a single value in [0,max] onto one of the eight block glyphs.
// With max<=0 (a flat-zero series) it returns the baseline block.
func sparkBlock(v, max float64) rune {
	if max <= 0 || v <= 0 {
		return sparkBlocks[0]
	}
	if v >= max {
		return sparkBlocks[len(sparkBlocks)-1]
	}
	idx := int(v / max * float64(len(sparkBlocks)))
	if idx >= len(sparkBlocks) {
		idx = len(sparkBlocks) - 1
	}
	return sparkBlocks[idx]
}
