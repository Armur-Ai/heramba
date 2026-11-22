package watch

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestSparklineWidth(t *testing.T) {
	for _, w := range []int{1, 5, 20} {
		got := sparkline([]float64{1, 2, 3}, w)
		if lipgloss.Width(got) != w {
			t.Errorf("sparkline width = %d, want %d (%q)", lipgloss.Width(got), w, got)
		}
	}
}

func TestSparklineEmpty(t *testing.T) {
	if sparkline(nil, 10) != "" {
		t.Error("nil series should render empty")
	}
	if sparkline([]float64{1}, 0) != "" {
		t.Error("width 0 should render empty")
	}
}

func TestSparklineUsesTail(t *testing.T) {
	// 10 samples into width 3 keeps the last three; the last is the max so it
	// must be the full block.
	got := []rune(sparkline([]float64{0, 0, 0, 0, 0, 0, 0, 1, 2, 9}, 3))
	if got[len(got)-1] != '█' {
		t.Errorf("peak sample should be the full block, got %q", string(got[len(got)-1]))
	}
}

func TestSparkBlockBounds(t *testing.T) {
	if sparkBlock(0, 10) != '▁' {
		t.Error("zero should be the baseline block")
	}
	if sparkBlock(10, 10) != '█' {
		t.Error("max should be the full block")
	}
	if sparkBlock(5, 0) != '▁' {
		t.Error("a flat-zero series (max<=0) should be the baseline block")
	}
}

func TestSparklineFlatSeries(t *testing.T) {
	// An all-zero series must still fill the width with baseline blocks, not
	// collapse to empty.
	got := sparkline([]float64{0, 0, 0, 0}, 4)
	if got != "▁▁▁▁" {
		t.Errorf("flat series = %q, want four baseline blocks", got)
	}
}
