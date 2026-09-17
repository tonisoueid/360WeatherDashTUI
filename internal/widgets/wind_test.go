package widgets

import (
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/tsoueid/weatherdash/internal/units"
	"github.com/tsoueid/weatherdash/internal/weather"
)

// Open-Meteo reports the direction the wind blows FROM. The arrow shows
// where it is GOING, matching weather-map convention, so a northerly
// (0°, from the north) points south.
func TestWindArrowDirection(t *testing.T) {
	tests := []struct {
		deg  int
		want string
	}{
		{0, "↓"}, {45, "↙"}, {90, "←"}, {135, "↖"},
		{180, "↑"}, {225, "↗"}, {270, "→"}, {315, "↘"},
		{360, "↓"}, {359, "↓"},
	}
	for _, tc := range tests {
		if got := WindArrow(tc.deg); got != tc.want {
			t.Errorf("WindArrow(%d) = %q, want %q", tc.deg, got, tc.want)
		}
	}
}

func TestWindArrowIsSingleWidth(t *testing.T) {
	for deg := 0; deg < 360; deg += 15 {
		if w := lipgloss.Width(WindArrow(deg)); w != 1 {
			t.Errorf("WindArrow(%d) width = %d, want 1", deg, w)
		}
	}
}

func TestWindArrowHandlesOutOfRange(t *testing.T) {
	for _, deg := range []int{-45, 720, -1} {
		if w := lipgloss.Width(WindArrow(deg)); w != 1 {
			t.Errorf("WindArrow(%d) width = %d, want 1", deg, w)
		}
	}
}

func TestWindRowWidths(t *testing.T) {
	hours := make([]weather.Hour, 24)
	for i := range hours {
		hours[i] = weather.Hour{
			Time:    time.Date(2026, 9, 17, i, 0, 0, 0, time.UTC),
			WindKPH: float64(5 + i),
			WindDir: i * 15,
		}
	}
	for _, w := range []int{20, 40, 60} {
		arrows, speeds := WindRow(hours, w, units.Metric)
		if lipgloss.Width(arrows) != w {
			t.Errorf("arrows width = %d, want %d (%q)", lipgloss.Width(arrows), w, arrows)
		}
		if lipgloss.Width(speeds) != w {
			t.Errorf("speeds width = %d, want %d (%q)", lipgloss.Width(speeds), w, speeds)
		}
	}
}

func TestWindRowEmpty(t *testing.T) {
	arrows, speeds := WindRow(nil, 10, units.Metric)
	if lipgloss.Width(arrows) != 10 || lipgloss.Width(speeds) != 10 {
		t.Errorf("empty wind row widths = %d,%d; want 10,10",
			lipgloss.Width(arrows), lipgloss.Width(speeds))
	}
}
