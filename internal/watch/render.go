package watch

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// This file holds the low-level layout primitives the dashboard renders with.
//
// The cardinal rule here is: NEVER truncate or pad a string that already
// carries ANSI styling. Cutting in the middle of an escape sequence is exactly
// what makes a TUI look "broken" — stray color codes, smeared rows, columns
// that don't line up. So every cell is laid out on its PLAIN text first (fit to
// an exact column width), and the color is applied last, to a string whose
// width we already know. Rows are then assembled from fixed-width cells, so the
// total width is predictable regardless of content.

// dw is the display width of a plain string: wide (CJK/emoji) runes count as 2,
// combining marks as 0. It must only be called on text without ANSI codes.
func dw(s string) int { return lipgloss.Width(s) }

// fit returns s trimmed or space-padded to exactly w display columns. Longer
// input is truncated with a single-column ellipsis; shorter input is right-
// padded with spaces. The result is always exactly w columns wide (for w >= 1).
func fit(s string, w int) string {
	if w <= 0 {
		return ""
	}
	switch cur := dw(s); {
	case cur == w:
		return s
	case cur < w:
		return s + strings.Repeat(" ", w-cur)
	default:
		return truncPlain(s, w)
	}
}

// truncPlain truncates a PLAIN string to at most w display columns, appending a
// one-column ellipsis ("…") when it has to cut. The result is padded back out
// to exactly w columns so callers can rely on the width even when the cut lands
// on a wide-rune boundary. It never returns a string wider than w.
func truncPlain(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if dw(s) <= w {
		return s
	}
	if w == 1 {
		return "…"
	}
	var b strings.Builder
	width := 0
	for _, r := range s {
		rw := dw(string(r))
		if width+rw > w-1 { // leave one column for the ellipsis
			break
		}
		b.WriteRune(r)
		width += rw
	}
	out := b.String() + "…"
	if gap := w - dw(out); gap > 0 {
		out += strings.Repeat(" ", gap)
	}
	return out
}

// padLeft right-aligns a plain string in width w (padding on the left). Longer
// input is truncated to w.
func padLeft(s string, w int) string {
	if cur := dw(s); cur < w {
		return strings.Repeat(" ", w-cur) + s
	}
	return truncPlain(s, w)
}

// cell lays out plain text in an exact-width column and then styles it, so the
// ANSI codes wrap a string whose width is already correct.
func cell(s string, w int, style lipgloss.Style) string {
	return style.Render(fit(s, w))
}

// repeat returns a string of n copies of r (n<=0 yields ""). It is the
// building block for rules, bars, and spacers.
func repeat(r string, n int) string {
	if n <= 0 {
		return ""
	}
	return strings.Repeat(r, n)
}
