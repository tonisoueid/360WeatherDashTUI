package widgets

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/tsoueid/weatherdash/internal/units"
	"github.com/tsoueid/weatherdash/internal/weather"
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
		for _, w := range []int{5, 6, 7, 8, 10, 13, 14, 16, 20} {
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
		{5, TierMinimal}, {7, TierMinimal},
		{8, TierMedium}, {13, TierMedium},
		{14, TierRich}, {20, TierRich},
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
	got := Cell(sampleDay(), 9, TierMedium, weather.IconsASCII, units.Metric, nil)
	if !strings.Contains(got, "24") || !strings.Contains(got, "12") {
		t.Errorf("medium cell %q must show both 24 and 12", got)
	}
}

// TestCellMediumHasMoreRoomThanTheDigitsAlone guards the "more space"
// request: TierMedium must not glue the icon straight onto the
// temperatures the way TierMinimal does.
func TestCellMediumHasMoreRoomThanTheDigitsAlone(t *testing.T) {
	got := Cell(sampleDay(), 14, TierMedium, weather.IconsASCII, units.Metric, nil)
	if !strings.Contains(got, "  24/12") {
		t.Errorf("medium cell %q must have breathing room before the temperatures", got)
	}
}

func TestCellRichShowsRainPercent(t *testing.T) {
	got := Cell(sampleDay(), 14, TierRich, weather.IconsASCII, units.Metric, nil)
	if !strings.Contains(got, "85%") {
		t.Errorf("rich cell %q must show the rain probability as 85%%, not a glyph", got)
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
	got := Cell(sampleDay(), 14, TierRich, weather.IconsASCII, units.Metric, style)
	if !strings.Contains(got, "\x1b[48;5;27m") {
		t.Errorf("rich cell %q must carry the rainStyle background", got)
	}
}

func TestCellRespectsUnits(t *testing.T) {
	got := Cell(sampleDay(), 14, TierRich, weather.IconsASCII, units.Imperial, nil)
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
	got := Cell(weather.Day{}, 9, TierMedium, weather.IconsASCII, units.Metric, nil)
	if lipgloss.Width(got) != 9 {
		t.Errorf("width = %d, want 9", lipgloss.Width(got))
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
		{5, TierMinimal}, {8, TierMedium}, {14, TierRich}, {20, TierRich}, {3, TierNone},
	} {
		got := CellHeader(tc.w, tc.tier)
		if lipgloss.Width(got) != tc.w {
			t.Errorf("CellHeader(%d, %v) width = %d, want %d", tc.w, tc.tier, lipgloss.Width(got), tc.w)
		}
	}
}

func TestCellHeaderNamesOnlyWhatTheTierShows(t *testing.T) {
	minimal := CellHeader(5, TierMinimal)
	if !strings.Contains(minimal, "cond") {
		t.Errorf("minimal header %q must name the condition column", minimal)
	}
	if strings.Contains(minimal, "temp") || strings.Contains(minimal, "rain") {
		t.Errorf("minimal header %q must not claim columns TierMinimal doesn't show", minimal)
	}

	medium := CellHeader(13, TierMedium)
	if !strings.Contains(medium, "tmp") {
		t.Errorf("medium header %q must name the temperature column", medium)
	}
	if strings.Contains(medium, "rain") {
		t.Errorf("medium header %q must not claim the rain column TierMedium doesn't show", medium)
	}

	rich := CellHeader(20, TierRich)
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
