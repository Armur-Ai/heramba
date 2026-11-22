package watch

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestFitExactWidth(t *testing.T) {
	cases := []struct {
		in string
		w  int
	}{
		{"", 5},
		{"abc", 5},
		{"abcde", 5},
		{"abcdefgh", 5},
		{"日本語テスト", 7}, // wide runes
		{"héllo", 4},
	}
	for _, c := range cases {
		got := fit(c.in, c.w)
		if w := dw(got); w != c.w {
			t.Errorf("fit(%q,%d) width = %d, want %d (got %q)", c.in, c.w, w, c.w, got)
		}
	}
}

func TestFitZeroWidth(t *testing.T) {
	if got := fit("anything", 0); got != "" {
		t.Errorf("fit with w=0 should be empty, got %q", got)
	}
}

func TestTruncPlainEllipsis(t *testing.T) {
	got := truncPlain("abcdefghij", 5)
	if dw(got) != 5 {
		t.Fatalf("width = %d, want 5 (%q)", dw(got), got)
	}
	if !strings.HasSuffix(strings.TrimRight(got, " "), "…") {
		t.Errorf("expected an ellipsis, got %q", got)
	}
	if !strings.HasPrefix(got, "abcd") {
		t.Errorf("expected the head preserved, got %q", got)
	}
}

func TestTruncPlainNoCutWhenShort(t *testing.T) {
	if got := truncPlain("abc", 10); got != "abc" {
		t.Errorf("short string must pass through unchanged, got %q", got)
	}
}

func TestTruncPlainWidthOne(t *testing.T) {
	if got := truncPlain("abcdef", 1); got != "…" {
		t.Errorf("w=1 should be a lone ellipsis, got %q", got)
	}
}

func TestPadLeftRightAligns(t *testing.T) {
	got := padLeft("42", 6)
	if got != "    42" {
		t.Errorf("padLeft = %q, want %q", got, "    42")
	}
	if dw(got) != 6 {
		t.Errorf("padLeft width = %d, want 6", dw(got))
	}
}

func TestCellPreservesWidthUnderStyle(t *testing.T) {
	// A styled cell's VISIBLE width (ANSI stripped) must equal the column width,
	// no matter how long the input was — this is the property that keeps columns
	// aligned. lipgloss.Width strips ANSI before measuring.
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("#ff0000")).Bold(true)
	for _, in := range []string{"", "short", "a-very-long-value-that-overflows"} {
		got := cell(in, 8, style)
		if w := lipgloss.Width(got); w != 8 {
			t.Errorf("cell(%q) visible width = %d, want 8", in, w)
		}
	}
}

func TestRepeat(t *testing.T) {
	if repeat("─", 3) != "───" {
		t.Error("repeat should concatenate")
	}
	if repeat("x", 0) != "" || repeat("x", -2) != "" {
		t.Error("repeat with n<=0 should be empty")
	}
}
