package widgets

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/tonisoueid/360WeatherDashTUI/internal/units"
	"github.com/tonisoueid/360WeatherDashTUI/internal/weather"
)

// sampleDay sets Date deliberately: a zero Date is Cell's signal for
// "no data yet" (see TestCellZeroDayRendersPlaceholder), so a day with
// real values must not leave it zero or every assertion below would
// silently see a blank placeholder instead of the rendered content.
func sampleDay() weather.Day {
	return weather.Day{
		Date: time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC),
		Code: 61, HiC: 24, LoC: 12, PrecipProb: 85,
	}
}

// Every cell in a row must be exactly the budgeted width, in every
// tier, in every icon set. A single wrong cell shifts the whole row.
func TestCellWidthIsExact(t *testing.T) {
	sets := []weather.IconSet{weather.IconsASCII, weather.IconsUnicode, weather.IconsNerd}
	for _, set := range sets {
		for _, w := range []int{5, 7, 9, 10, 13, 15, 16, 20} {
			tier := TierFor(w)
			got := Cell(sampleDay(), w, tier, set, units.Metric, nil)
			if lipgloss.Width(got) != w {
				t.Errorf("Cell(w=%d, set=%v) width = %d, want %d (got %q)",
					w, set, lipgloss.Width(got), w, got)
			}
		}
	}
}

func TestTierForThresholds(t *testing.T) {
	tests := []struct {
		w    int
		want Tier
	}{
		{0, TierNone}, {4, TierNone},
		{5, TierMinimal}, {9, TierMinimal},
		{10, TierMedium}, {15, TierMedium},
		{16, TierRich}, {20, TierRich},
	}
	for _, tc := range tests {
		if got := TierFor(tc.w); got != tc.want {
			t.Errorf("TierFor(%d) = %v, want %v", tc.w, got, tc.want)
		}
	}
}

func TestTierMinWidthMatchesTierFor(t *testing.T) {
	for _, tier := range []Tier{TierMinimal, TierMedium, TierRich} {
		if got := TierFor(tier.MinWidth()); got != tier {
			t.Errorf("TierFor(%v.MinWidth()=%d) = %v, want %v",
				tier, tier.MinWidth(), got, tier)
		}
	}
}

func TestCellMinimalShowsHighOnly(t *testing.T) {
	got := Cell(sampleDay(), 5, TierMinimal, weather.IconsASCII, units.Metric, nil)
	if !strings.Contains(got, "24") {
		t.Errorf("minimal cell %q must show the high", got)
	}
	if strings.Contains(got, "12") {
		t.Errorf("minimal cell %q must not show the low", got)
	}
}

func TestCellMediumShowsHighAndLow(t *testing.T) {
	got := Cell(sampleDay(), TierMedium.MinWidth(), TierMedium, weather.IconsASCII, units.Metric, nil)
	if !strings.Contains(got, "24") || !strings.Contains(got, "12") {
		t.Errorf("medium cell %q must show both 24 and 12", got)
	}
}

// TestCellFixedWidthTempAligns is the regression test for the bug a
// screenshot of the real grid surfaced: temperatures were formatted
// with a plain "%d", so a two-digit day and a three-character day
// (negative double digits) put the slash — and everything after it —
// in different columns depending on the row. "%3d/%3d" must always
// produce the same 7-column shape regardless of digit count, so a
// column of dates stays a straight column instead of a zig-zag.
func TestCellFixedWidthTempAligns(t *testing.T) {
	warm := sampleDay() // HiC 24, LoC 12: two digits each
	cold := sampleDay()
	cold.HiC, cold.LoC = -5, -12 // one and two digits, with a sign

	w := TierMedium.MinWidth()
	warmCell := Cell(warm, w, TierMedium, weather.IconsASCII, units.Metric, nil)
	coldCell := Cell(cold, w, TierMedium, weather.IconsASCII, units.Metric, nil)

	if !strings.Contains(warmCell, " 24/ 12") {
		t.Errorf("warm cell %q must right-align to \" 24/ 12\"", warmCell)
	}
	if !strings.Contains(coldCell, " -5/-12") {
		t.Errorf("cold cell %q must right-align to \" -5/-12\"", coldCell)
	}
	// Both must place the slash in the SAME column: the whole point of
	// fixed-width formatting.
	if strings.Index(warmCell, "/") != strings.Index(coldCell, "/") {
		t.Errorf("slash column differs: warm %q, cold %q", warmCell, coldCell)
	}
}

// TestCellMediumHasMoreRoomThanTheDigitsAlone guards the "more space"
// request: TierMedium must not glue the icon straight onto the
// temperatures the way TierMinimal does.
func TestCellMediumHasMoreRoomThanTheDigitsAlone(t *testing.T) {
	got := Cell(sampleDay(), TierMedium.MinWidth(), TierMedium, weather.IconsASCII, units.Metric, nil)
	if !strings.Contains(got, "   24/ 12") {
		t.Errorf("medium cell %q must have breathing room before the temperatures", got)
	}
}

func TestCellRichShowsRainPercent(t *testing.T) {
	got := Cell(sampleDay(), TierRich.MinWidth(), TierRich, weather.IconsASCII, units.Metric, nil)
	if !strings.Contains(got, "85%") {
		t.Errorf("rich cell %q must show the rain probability as 85%%, not a glyph", got)
	}
}

// TestCellFixedWidthRainAligns is TestCellFixedWidthTempAligns's
// counterpart for the rain figure: "%3d%%" must right-align "8%" and
// "100%" to the same column regardless of digit count.
func TestCellFixedWidthRainAligns(t *testing.T) {
	low := sampleDay()
	low.PrecipProb = 8
	high := sampleDay()
	high.PrecipProb = 100

	w := TierRich.MinWidth()
	lowCell := Cell(low, w, TierRich, weather.IconsASCII, units.Metric, nil)
	highCell := Cell(high, w, TierRich, weather.IconsASCII, units.Metric, nil)

	if !strings.Contains(lowCell, "  8%") {
		t.Errorf("low cell %q must right-align to \"  8%%\"", lowCell)
	}
	if !strings.Contains(highCell, "100%") {
		t.Errorf("high cell %q must show \"100%%\" at full width", highCell)
	}
	if strings.Index(lowCell, "%") != strings.Index(highCell, "%") {
		t.Errorf("%% column differs: low %q, high %q", lowCell, highCell)
	}
}

// TestCellRichColoursTheRainFigure confirms the rainStyle hook is
// actually applied to the percentage, not just accepted and ignored.
// Colour is optional (nil is the plain-text/golden-test path), but
// when a caller supplies one it must show up.
func TestCellRichColoursTheRainFigure(t *testing.T) {
	prev := lipgloss.ColorProfile()
	defer lipgloss.SetColorProfile(prev)
	lipgloss.SetColorProfile(termenv.ANSI256)

	style := func(float64) lipgloss.Style {
		return lipgloss.NewStyle().Background(lipgloss.Color("27"))
	}
	got := Cell(sampleDay(), TierRich.MinWidth(), TierRich, weather.IconsASCII, units.Metric, style)
	if !strings.Contains(got, "\x1b[48;5;27m") {
		t.Errorf("rich cell %q must carry the rainStyle background", got)
	}
}

func TestCellRespectsUnits(t *testing.T) {
	got := Cell(sampleDay(), TierRich.MinWidth(), TierRich, weather.IconsASCII, units.Imperial, nil)
	if !strings.Contains(got, "75") {
		t.Errorf("imperial cell %q must show 75 (24°C)", got)
	}
}

func TestCellTierNoneIsBlank(t *testing.T) {
	got := Cell(sampleDay(), 3, TierNone, weather.IconsASCII, units.Metric, nil)
	if strings.TrimSpace(got) != "" {
		t.Errorf("TierNone cell = %q, want blank", got)
	}
	if lipgloss.Width(got) != 3 {
		t.Errorf("TierNone width = %d, want 3", lipgloss.Width(got))
	}
}

// A city with no data yet must render an empty cell of the right width,
// not a misleading 0°.
func TestCellZeroDayRendersPlaceholder(t *testing.T) {
	got := Cell(weather.Day{}, TierMedium.MinWidth(), TierMedium, weather.IconsASCII, units.Metric, nil)
	if lipgloss.Width(got) != TierMedium.MinWidth() {
		t.Errorf("width = %d, want %d", lipgloss.Width(got), TierMedium.MinWidth())
	}
	if strings.Contains(got, "0°") {
		t.Errorf("empty day rendered %q; a zero day must not show 0°", got)
	}
}

// --- CellHeader -------------------------------------------------------

func TestCellHeaderWidthIsExact(t *testing.T) {
	for _, tc := range []struct {
		w    int
		tier Tier
	}{
		{5, TierMinimal}, {10, TierMedium}, {16, TierRich}, {20, TierRich}, {3, TierNone},
	} {
		got := CellHeader(tc.w, tc.tier)
		if lipgloss.Width(got) != tc.w {
			t.Errorf("CellHeader(%d, %v) width = %d, want %d", tc.w, tc.tier, lipgloss.Width(got), tc.w)
		}
	}
}

func TestCellHeaderNamesOnlyWhatTheTierShows(t *testing.T) {
	minimal := CellHeader(TierMinimal.MinWidth(), TierMinimal)
	if !strings.Contains(minimal, "cond") {
		t.Errorf("minimal header %q must name the condition column", minimal)
	}
	if strings.Contains(minimal, "temp") || strings.Contains(minimal, "rain") {
		t.Errorf("minimal header %q must not claim columns TierMinimal doesn't show", minimal)
	}

	medium := CellHeader(TierMedium.MinWidth(), TierMedium)
	if !strings.Contains(medium, "temp") {
		t.Errorf("medium header %q must name the temperature column", medium)
	}
	if strings.Contains(medium, "rain") {
		t.Errorf("medium header %q must not claim the rain column TierMedium doesn't show", medium)
	}

	rich := CellHeader(TierRich.MinWidth(), TierRich)
	if !strings.Contains(rich, "rain") {
		t.Errorf("rich header %q must name the rain column", rich)
	}
}

// TestCellHeaderFitsTierRichMinWidthExactly guards the reason the
// wording was chosen: it must not truncate at TierRich's own floor.
func TestCellHeaderFitsTierRichMinWidthExactly(t *testing.T) {
	w := TierRich.MinWidth()
	got := CellHeader(w, TierRich)
	if strings.TrimRight(got, " ") != "cond temp rain" {
		t.Errorf("CellHeader(%d, TierRich) = %q, want the full label with no truncation", w, got)
	}
}
