package widgets

import (
	"strings"
	"time"
)

// SunArc renders the daylight span as a track exactly width columns
// wide, marking the sun's current position when it is up.
//
// The marker is omitted rather than clamped outside daylight: pinning
// it to an end would imply the sun is just rising or just setting,
// which is wrong at midnight. An absent marker reads correctly as
// "not up".
func SunArc(rise, set, now time.Time, width int) string {
	if width <= 0 {
		return ""
	}
	if rise.IsZero() || set.IsZero() || !set.After(rise) {
		return strings.Repeat("─", width)
	}

	track := []rune(strings.Repeat("━", width))

	span := set.Sub(rise)
	elapsed := now.Sub(rise)
	if elapsed >= 0 && elapsed <= span {
		pos := int(float64(elapsed) / float64(span) * float64(width-1))
		if pos < 0 {
			pos = 0
		}
		if pos >= width {
			pos = width - 1
		}
		track[pos] = '☉'
	}
	return string(track)
}
