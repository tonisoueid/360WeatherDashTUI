package widgets

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
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
		for _, w := range []int{5, 6, 7, 9, 10, 12, 16} {
			tier := TierFor(w)
			got := Cell(sampleDay(), w, tier, set, units.Metric)
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
		{5, TierMinimal}, {6, TierMinimal},
		{7, TierMedium}, {9, TierMedium},
		{10, TierRich}, {20, TierRich},
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
	got := Cell(sampleDay(), 5, TierMinimal, weather.IconsASCII, units.Metric)
	if !strings.Contains(got, "24") {
		t.Errorf("minimal cell %q must show the high", got)
	}
	if strings.Contains(got, "12") {
		t.Errorf("minimal cell %q must not show the low", got)
	}
}

func TestCellMediumShowsHighAndLow(t *testing.T) {
	got := Cell(sampleDay(), 9, TierMedium, weather.IconsASCII, units.Metric)
	if !strings.Contains(got, "24") || !strings.Contains(got, "12") {
		t.Errorf("medium cell %q must show both 24 and 12", got)
	}
}

func TestCellRichAddsPrecipGlyph(t *testing.T) {
	got := Cell(sampleDay(), 12, TierRich, weather.IconsASCII, units.Metric)
	if !strings.ContainsRune(got, '█') {
		t.Errorf("rich cell %q must carry a precip glyph for 85%%", got)
	}
}

func TestCellRespectsUnits(t *testing.T) {
	got := Cell(sampleDay(), 12, TierRich, weather.IconsASCII, units.Imperial)
	if !strings.Contains(got, "75") {
		t.Errorf("imperial cell %q must show 75 (24°C)", got)
	}
}

func TestCellTierNoneIsBlank(t *testing.T) {
	got := Cell(sampleDay(), 3, TierNone, weather.IconsASCII, units.Metric)
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
	got := Cell(weather.Day{}, 9, TierMedium, weather.IconsASCII, units.Metric)
	if lipgloss.Width(got) != 9 {
		t.Errorf("width = %d, want 9", lipgloss.Width(got))
	}
	if strings.Contains(got, "0°") {
		t.Errorf("empty day rendered %q; a zero day must not show 0°", got)
	}
}
