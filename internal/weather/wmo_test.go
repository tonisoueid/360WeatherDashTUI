package weather

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// allCodes is every WMO code Open-Meteo can return.
var allCodes = []int{
	0, 1, 2, 3, 45, 48, 51, 53, 55, 56, 57,
	61, 63, 65, 66, 67, 71, 73, 75, 77,
	80, 81, 82, 85, 86, 95, 96, 99,
}

// The grid budgets a fixed number of columns per cell and the icon is
// part of that budget. A double-width glyph in any tier breaks every
// row containing that condition.
func TestIconWidthInvariant(t *testing.T) {
	for _, set := range []IconSet{IconsASCII, IconsUnicode, IconsNerd} {
		want := IconWidth(set)
		for _, code := range allCodes {
			got := lipgloss.Width(Icon(code, set))
			if got != want {
				t.Errorf("Icon(%d, %v) width = %d, want %d (glyph %q)",
					code, set, got, want, Icon(code, set))
			}
		}
	}
}

func TestIconUnknownCodeFallsBack(t *testing.T) {
	for _, set := range []IconSet{IconsASCII, IconsUnicode, IconsNerd} {
		if got := Icon(12345, set); got == "" {
			t.Errorf("Icon(12345, %v) must return a fallback glyph, got empty", set)
		}
		if got, want := lipgloss.Width(Icon(12345, set)), IconWidth(set); got != want {
			t.Errorf("fallback width = %d, want %d", got, want)
		}
	}
}

func TestLabels(t *testing.T) {
	tests := map[int]string{
		0:  "Clear sky",
		3:  "Overcast",
		65: "Heavy rain",
		95: "Thunderstorm",
	}
	for code, want := range tests {
		if got := Label(code); got != want {
			t.Errorf("Label(%d) = %q, want %q", code, got, want)
		}
	}
}

func TestLabelUnknownCode(t *testing.T) {
	if got := Label(12345); got != "Unknown" {
		t.Errorf("Label(12345) = %q, want Unknown", got)
	}
}

func TestIconLegend(t *testing.T) {
	// One representative code per condition bucket, in bucket order.
	repCodes := []int{0, 1, 3, 45, 51, 61, 71, 80, 95, 12345}

	for _, set := range []IconSet{IconsASCII, IconsUnicode, IconsNerd} {
		legend := IconLegend(set)
		if len(legend) != len(repCodes) {
			t.Fatalf("IconLegend(%v) has %d entries, want %d", set, len(legend), len(repCodes))
		}
		seen := map[string]bool{}
		for i, e := range legend {
			if want := Icon(repCodes[i], set); e.Glyph != want {
				t.Errorf("IconLegend(%v)[%d].Glyph = %q, want Icon(%d) = %q",
					set, i, e.Glyph, repCodes[i], want)
			}
			if want := classify(repCodes[i]); e.Cond != want {
				t.Errorf("IconLegend(%v)[%d].Cond = %v, want %v (code %d)",
					set, i, e.Cond, want, repCodes[i])
			}
			if words := strings.Count(e.Meaning, " ") + 1; words > 2 {
				t.Errorf("meaning %q is %d words, want one or two", e.Meaning, words)
			}
			if seen[e.Meaning] {
				t.Errorf("duplicate meaning %q in %v legend", e.Meaning, set)
			}
			seen[e.Meaning] = true
		}
	}
}

func TestParseIconSetRoundTrip(t *testing.T) {
	for _, s := range []IconSet{IconsASCII, IconsUnicode, IconsNerd} {
		got, err := ParseIconSet(s.String())
		if err != nil {
			t.Fatalf("ParseIconSet(%q): %v", s.String(), err)
		}
		if got != s {
			t.Errorf("ParseIconSet(%q) = %v, want %v", s.String(), got, s)
		}
	}
	if _, err := ParseIconSet("wingdings"); err == nil {
		t.Error("unknown icon set must error")
	}
}
