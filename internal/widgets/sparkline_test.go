package widgets

import (
	"os"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// TestMain forces a colour profile before any test runs.
//
// lipgloss.Style.Render is a no-op — it returns the plain string with
// no escape codes — whenever its renderer cannot detect a terminal on
// the output stream. That is true both interactively when stdout is
// piped and, just as much, under `go test`, so without this the
// colorize test would silently pass for the wrong reason (comparing
// unstyled text to itself) rather than exercising the styling path at
// all.
func TestMain(m *testing.M) {
	lipgloss.SetColorProfile(termenv.ANSI)
	os.Exit(m.Run())
}

func TestSparklineWidth(t *testing.T) {
	vals := []float64{1, 5, 3, 8, 2, 7, 4}
	for _, w := range []int{1, 5, 7, 24, 40} {
		got := Sparkline(vals, w, nil)
		if lipgloss.Width(got) != w {
			t.Errorf("Sparkline width %d = %d (got %q)", w, lipgloss.Width(got), got)
		}
	}
}

func TestSparklineMapsRangeToFullRamp(t *testing.T) {
	got := Sparkline([]float64{0, 1, 2, 3, 4, 5, 6, 7}, 8, nil)
	if got != "▁▂▃▄▅▆▇█" {
		t.Errorf("Sparkline = %q, want the full ramp", got)
	}
}

// A flat series has no range to normalise against. It must render as a
// consistent mid-height line, not divide by zero.
func TestSparklineFlatSeries(t *testing.T) {
	got := Sparkline([]float64{5, 5, 5, 5}, 4, nil)
	if lipgloss.Width(got) != 4 {
		t.Fatalf("width = %d, want 4", lipgloss.Width(got))
	}
	for _, r := range got {
		if r != '▄' {
			t.Errorf("flat series rendered %q, want all ▄", got)
			break
		}
	}
}

func TestSparklineNegativeValues(t *testing.T) {
	got := Sparkline([]float64{-10, 0, 10}, 3, nil)
	if got != "▁▄█" {
		t.Errorf("Sparkline with negatives = %q, want ▁▄█", got)
	}
}

func TestSparklineEmptyAndZeroWidth(t *testing.T) {
	if got := Sparkline(nil, 5, nil); lipgloss.Width(got) != 5 {
		t.Errorf("nil values must still fill the width, got %q", got)
	}
	if got := Sparkline([]float64{1, 2}, 0, nil); got != "" {
		t.Errorf("zero width = %q, want empty", got)
	}
}

func TestSparklineAppliesColorize(t *testing.T) {
	red := lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	got := Sparkline([]float64{1, 2, 3}, 3, func(float64) lipgloss.Style { return red })

	if lipgloss.Width(got) != 3 {
		t.Errorf("colorized width = %d, want 3", lipgloss.Width(got))
	}
	if got == "▁▄█" {
		t.Error("Colorize was not applied")
	}
}

func TestPrecipBandRamp(t *testing.T) {
	got := PrecipBand([]int{0, 15, 40, 65, 90}, 5)
	if got != "·░▒▓█" {
		t.Errorf("PrecipBand = %q, want ·░▒▓█", got)
	}
}

func TestPrecipBandWidthAndClamping(t *testing.T) {
	if got := PrecipBand([]int{50}, 10); lipgloss.Width(got) != 10 {
		t.Errorf("width = %d, want 10", lipgloss.Width(got))
	}
	// Out-of-range probabilities must clamp, not index out of bounds.
	if got := PrecipBand([]int{-20, 250}, 2); lipgloss.Width(got) != 2 {
		t.Errorf("clamped width = %d, want 2 (got %q)", lipgloss.Width(got), got)
	}
}

func TestPrecipBandEmpty(t *testing.T) {
	got := PrecipBand(nil, 4)
	if got != "····" {
		t.Errorf("PrecipBand(nil, 4) = %q, want ····", got)
	}
}
