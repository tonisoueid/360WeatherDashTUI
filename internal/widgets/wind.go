package widgets

import (
	"strings"

	"github.com/tsoueid/360WeatherDashTUI/internal/units"
	"github.com/tsoueid/360WeatherDashTUI/internal/weather"
)

// windArrows are indexed by 45° sector, starting at "going north".
var windArrows = []string{"↑", "↗", "→", "↘", "↓", "↙", "←", "↖"}

// WindArrow renders a wind direction as a single-width arrow.
//
// Open-Meteo reports the direction wind blows FROM; the arrow shows
// where it is GOING, which is the convention weather maps use and the
// one people read correctly without a legend. Hence the 180° rotation.
func WindArrow(degrees int) string {
	d := ((degrees+180)%360 + 360) % 360
	idx := int((float64(d)+22.5)/45) % len(windArrows)
	return windArrows[idx]
}

// WindRow renders wind direction and speed as two aligned rows exactly
// width columns wide.
//
// Arrows are sampled rather than averaged: averaging directions across
// a bucket is meaningless when they straddle north (350° and 10° would
// average to 180°, the exact opposite). Speeds are labelled at a
// readable interval instead of on every column; each label can be
// coloured by the speed it states, via the same optional hook
// Sparkline uses. Labels are trimmed to the room they have BEFORE
// styling — cutting a styled string would split its escape sequence.
func WindRow(hours []weather.Hour, width int, u units.System, colorize Colorize) (arrows, speeds string) {
	if width <= 0 {
		return "", ""
	}
	if len(hours) == 0 {
		blank := strings.Repeat(" ", width)
		return blank, blank
	}

	// One arrow every other column keeps the row from looking like a
	// solid bar while still tracking the shape of the day.
	const arrowEvery = 2

	var ab strings.Builder
	for col := 0; col < width; col++ {
		if col%arrowEvery != 0 {
			ab.WriteByte(' ')
			continue
		}
		idx := col * len(hours) / width
		if idx >= len(hours) {
			idx = len(hours) - 1
		}
		ab.WriteString(WindArrow(hours[idx].WindDir))
	}

	// Speed labels every 8 columns, left-anchored to their sample.
	const labelEvery = 8
	var sb strings.Builder
	col := 0
	for pos := 0; pos < width; pos += labelEvery {
		if pos < col {
			// A longer predecessor left no room for this label.
			continue
		}
		idx := pos * len(hours) / width
		if idx >= len(hours) {
			idx = len(hours) - 1
		}
		kph := hours[idx].WindKPH
		runes := []rune(u.Speed(kph))
		if len(runes) > width-pos {
			runes = runes[:width-pos]
		}
		sb.WriteString(strings.Repeat(" ", pos-col))
		text := string(runes)
		if colorize != nil {
			text = colorize(kph).Render(text)
		}
		sb.WriteString(text)
		col = pos + len(runes)
	}
	if col < width {
		sb.WriteString(strings.Repeat(" ", width-col))
	}

	return Pad(ab.String(), width), sb.String()
}
