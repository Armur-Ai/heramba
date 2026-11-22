package watch

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestMeterWidthExact(t *testing.T) {
	for _, w := range []int{1, 10, 40} {
		for _, f := range []float64{-1, 0, 0.37, 0.5, 1, 2} {
			got := meter(f, w)
			if lipgloss.Width(got) != w {
				t.Errorf("meter(%v,%d) width = %d, want %d", f, w, lipgloss.Width(got), w)
			}
		}
	}
}

func TestMeterFillProportion(t *testing.T) {
	got := meter(0.5, 10)
	on := strings.Count(got, "█")
	if on != 5 {
		t.Errorf("half meter should have 5 filled cells, got %d (%q)", on, got)
	}
	if strings.Count(meter(0, 10), "█") != 0 {
		t.Error("empty meter should have no filled cells")
	}
	if strings.Count(meter(1, 10), "█") != 10 {
		t.Error("full meter should be all filled")
	}
}

func TestMeterClampsOutOfRange(t *testing.T) {
	if strings.Count(meter(5, 8), "█") != 8 {
		t.Error("frac>1 should clamp to full")
	}
	if strings.Count(meter(-3, 8), "█") != 0 {
		t.Error("frac<0 should clamp to empty")
	}
}

func TestColoredMeterVisibleWidth(t *testing.T) {
	got := coloredMeter(0.6, 20, colGreen)
	if lipgloss.Width(got) != 20 {
		t.Errorf("colored meter visible width = %d, want 20", lipgloss.Width(got))
	}
}

func TestClamp01(t *testing.T) {
	if clamp01(-0.1) != 0 || clamp01(1.1) != 1 || clamp01(0.4) != 0.4 {
		t.Error("clamp01 bounds wrong")
	}
}
