package widgets

import (
	"os"
	"strings"
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

func TestPrecipNumbersAtHourColumns(t *testing.T) {
	// Four values on twelve columns: one slot of three columns per
	// hour, each value left-aligned at its hour's column. Values that
	// touch stay readable — the % delimits them.
	got := PrecipNumbers([]int{5, 45, 90, 65}, 12, nil)
	if got != "5% 45%90%65%" {
		t.Errorf("PrecipNumbers = %q, want %q", got, "5% 45%90%65%")
	}
}

// Each figure carries its own percent sign.
func TestPrecipNumbersPercentOnEachFigure(t *testing.T) {
	probs := []int{0, 5, 100}
	for i, want := range []string{"0%", "5%", "100%"} {
		if got := PrecipNumbers([]int{probs[i]}, 6, nil); !strings.Contains(got, want) {
			t.Errorf("PrecipNumbers(%d) = %q, must carry %q", probs[i], got, want)
		}
	}
}

func TestPrecipNumbersWidth(t *testing.T) {
	probs := make([]int, 24)
	for i := range probs {
		probs[i] = i * 4
	}
	for _, w := range []int{1, 8, 49, 69, 89, 109} {
		if got := PrecipNumbers(probs, w, nil); lipgloss.Width(got) != w {
			t.Errorf("width %d = %d (got %q)", w, lipgloss.Width(got), got)
		}
	}
}

// At widths too tight for every hour, a value that would collide is
// skipped — a missing number is honest, a garbled one is not.
func TestPrecipNumbersSkipsCollisions(t *testing.T) {
	// Six hours on six columns: only the first and fourth values fit,
	// packed shoulder to shoulder.
	got := PrecipNumbers([]int{11, 22, 33, 44, 55, 66}, 6, nil)
	if got != "11%44%" {
		t.Errorf("PrecipNumbers = %q, want %q", got, "11%44%")
	}
}

func TestPrecipNumbersClamping(t *testing.T) {
	// Out-of-range probabilities must clamp to 0–100, not print a
	// negative sign or five columns of "1000%".
	if got := PrecipNumbers([]int{-20, 250}, 8, nil); got != "0%  100%" {
		t.Errorf("PrecipNumbers(-20, 250) = %q, want %q", got, "0%  100%")
	}
}

func TestPrecipNumbersEmpty(t *testing.T) {
	if got := PrecipNumbers(nil, 4, nil); got != "    " {
		t.Errorf("PrecipNumbers(nil, 4, nil) = %q, want four spaces", got)
	}
	if got := PrecipNumbers([]int{50}, 0, nil); got != "" {
		t.Errorf("PrecipNumbers(_, 0) = %q, want empty", got)
	}
}

// Each figure wears the colour of the chance it states, not one
// colour for the row, and the row still fills its width exactly.
func TestPrecipNumbersColorizesEachFigure(t *testing.T) {
	dry := lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
	wet := lipgloss.NewStyle().Foreground(lipgloss.Color("27"))
	hours := []int{5, 90}

	got := PrecipNumbers(hours, 16, func(pct float64) lipgloss.Style {
		if pct > 50 {
			return wet
		}
		return dry
	})
	if lipgloss.Width(got) != 16 {
		t.Errorf("colorized width = %d, want 16 (%q)", lipgloss.Width(got), got)
	}
	if !strings.Contains(got, dry.Render("5%")) {
		t.Errorf("the dry figure must wear its own colour:\n%q", got)
	}
	if !strings.Contains(got, wet.Render("90%")) {
		t.Errorf("the soaked figure must wear its own colour:\n%q", got)
	}
}

func TestPrecipAmountsAtHourColumns(t *testing.T) {
	// Dry hours print a dash; amounts sit at their hour's column.
	// Four samples on sixteen columns: one per four columns.
	got := PrecipAmounts([]float64{0, 0.4, 2.5, 12}, 16, nil)
	if got != "-   0.4 2.5 12  " {
		t.Errorf("PrecipAmounts = %q, want %q", got, "-   0.4 2.5 12  ")
	}
}

// Formatting: sub-10 values keep a decimal, bigger ones go whole, and
// effectively-zero rain prints a bare dash.
func TestPrecipAmountsFormatting(t *testing.T) {
	cases := []struct {
		mm   float64
		want string
	}{
		{0.04, "-"},   // rounds away to dry
		{0, "-"},      // dry
		{-2, "-"},     // bad data clamps to dry
		{0.44, "0.4"}, // drizzle keeps a decimal
		{9.8, "9.8"},
		{10.4, "10"}, // heavy goes whole
		{12.6, "13"},
	}
	for _, tc := range cases {
		got := PrecipAmounts([]float64{tc.mm}, 8, nil)
		if !strings.Contains(got, tc.want) {
			t.Errorf("PrecipAmounts(%v) = %q, must carry %q", tc.mm, got, tc.want)
		}
	}
}

// Each amount figure wears the colour of the intensity it states, and
// the row still fills its width exactly.
func TestPrecipAmountsColorizesEachFigure(t *testing.T) {
	drizzle := lipgloss.NewStyle().Foreground(lipgloss.Color("153"))
	torrent := lipgloss.NewStyle().Foreground(lipgloss.Color("57"))
	mms := []float64{0.5, 20}

	got := PrecipAmounts(mms, 16, func(v float64) lipgloss.Style {
		if v >= 10 {
			return torrent
		}
		return drizzle
	})
	if lipgloss.Width(got) != 16 {
		t.Errorf("colorized width = %d, want 16 (%q)", lipgloss.Width(got), got)
	}
	if !strings.Contains(got, drizzle.Render("0.5")) {
		t.Errorf("the drizzle figure must wear its own colour:\n%q", got)
	}
	if !strings.Contains(got, torrent.Render("20")) {
		t.Errorf("the torrential figure must wear its own colour:\n%q", got)
	}
}

func TestPrecipAmountsEmpty(t *testing.T) {
	if got := PrecipAmounts(nil, 4, nil); got != "    " {
		t.Errorf("PrecipAmounts(nil, 4) = %q, want four spaces", got)
	}
	if got := PrecipAmounts([]float64{1}, 0, nil); got != "" {
		t.Errorf("PrecipAmounts(_, 0) = %q, want empty", got)
	}
}
