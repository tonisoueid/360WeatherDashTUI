package widgets

import (
	"strings"

	"github.com/tsoueid/weatherdash/internal/units"
	"github.com/tsoueid/weatherdash/internal/weather"
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
// readable interval instead of on every column.
func WindRow(hours []weather.Hour, width int, u units.System) (arrows, speeds string) {
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
	sb := make([]rune, width)
	for i := range sb {
		sb[i] = ' '
	}
	for col := 0; col < width; col += labelEvery {
		idx := col * len(hours) / width
		if idx >= len(hours) {
			idx = len(hours) - 1
		}
		label := []rune(u.Speed(hours[idx].WindKPH))
		for j, r := range label {
			if col+j >= width {
				break
			}
			sb[col+j] = r
		}
	}

	return Pad(ab.String(), width), Pad(string(sb), width)
}
