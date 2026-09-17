package widgets

import (
	"fmt"
	"strings"

	"github.com/tsoueid/weatherdash/internal/units"
	"github.com/tsoueid/weatherdash/internal/weather"
)

// Tier is how much detail a grid cell shows. Narrow terminals drop to
// lower tiers rather than truncating, because a clipped "24/1" is worse
// than an honest "24".
type Tier int

const (
	// TierNone is too narrow to render anything.
	TierNone Tier = iota
	// TierMinimal is icon plus the daily high: "O24".
	TierMinimal
	// TierMedium adds the low: "O 24/12".
	TierMedium
	// TierRich adds a precipitation glyph: "O 24/12 █".
	TierRich
)

// MinWidth is the narrowest cell that can render this tier.
func (t Tier) MinWidth() int {
	switch t {
	case TierRich:
		return 10
	case TierMedium:
		return 7
	case TierMinimal:
		return 5
	default:
		return 0
	}
}

// TierFor picks the richest tier that fits in cellW columns.
//
// The model calls this once per render and passes the result to every
// cell, so a whole row shares one tier. Deriving it per cell would let
// a column with a three-digit temperature silently drop a tier and make
// the grid ragged.
func TierFor(cellW int) Tier {
	switch {
	case cellW >= TierRich.MinWidth():
		return TierRich
	case cellW >= TierMedium.MinWidth():
		return TierMedium
	case cellW >= TierMinimal.MinWidth():
		return TierMinimal
	default:
		return TierNone
	}
}

// Cell renders one day for one city, padded to exactly width columns.
//
// A zero-valued Day means "no data yet" and renders blank rather than
// 0°, which would be indistinguishable from a genuine freezing day.
func Cell(d weather.Day, width int, tier Tier, set weather.IconSet, u units.System) string {
	if width <= 0 {
		return ""
	}
	if tier == TierNone || d.Date.IsZero() {
		return strings.Repeat(" ", width)
	}

	icon := weather.Icon(d.Code, set)
	hi := u.TempNum(d.HiC)

	var body string
	switch tier {
	case TierMinimal:
		body = fmt.Sprintf("%s%d", icon, hi)
	case TierMedium:
		body = fmt.Sprintf("%s %d/%d", icon, hi, u.TempNum(d.LoC))
	default:
		body = fmt.Sprintf("%s %d/%d %s",
			icon, hi, u.TempNum(d.LoC), string(precipLevel(float64(d.PrecipProb))))
	}
	return Pad(body, width)
}
