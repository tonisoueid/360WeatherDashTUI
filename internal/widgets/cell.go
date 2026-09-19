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
	// TierMinimal is icon plus the daily high: "O 24".
	TierMinimal
	// TierMedium adds the low, with room to breathe: "O  24/12".
	TierMedium
	// TierRich adds the rain probability as a number, not a glyph:
	// "O  24/12  45%".
	TierRich
)

// MinWidth is the narrowest cell that can render this tier.
//
// TierMedium and TierRich both got wider along with their content: the
// double space between sections is deliberate breathing room, not
// padding left over from a narrower design, and TierRich's rain figure
// can run to three digits ("100%"). Undercounting either would let
// TierFor pick a tier whose own content it then has to truncate — the
// exact "clipped '24/1'" failure this package's own doc comment warns
// against.
func (t Tier) MinWidth() int {
	switch t {
	case TierRich:
		return 14
	case TierMedium:
		return 8
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
//
// rainStyle colours the TierRich rain-probability figure — its own
// background, not just a foreground, so the number reads as a small
// tile of its own rather than inheriting whatever colour the rest of
// the cell wears (see internal/ui/theme.go's precipBadgeStyle for why
// that distinction matters). Passing nil renders the figure as plain
// text, which is what golden tests do so they compare characters
// rather than ANSI escapes — the same convention Sparkline and
// PrecipBand already use.
func Cell(d weather.Day, width int, tier Tier, set weather.IconSet, u units.System, rainStyle Colorize) string {
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
		body = fmt.Sprintf("%s %d", icon, hi)
	case TierMedium:
		body = fmt.Sprintf("%s  %d/%d", icon, hi, u.TempNum(d.LoC))
	default:
		rain := fmt.Sprintf("%d%%", d.PrecipProb)
		if rainStyle != nil {
			rain = rainStyle(float64(d.PrecipProb)).Render(rain)
		}
		body = fmt.Sprintf("%s  %d/%d  %s", icon, hi, u.TempNum(d.LoC), rain)
	}
	return Pad(body, width)
}

// CellHeader labels the sections a Cell at this tier packs together —
// condition, hi/lo temperature, rain probability — so the column
// header reads as a legend for the data beneath it. It never claims a
// section a cell at this tier doesn't actually render.
//
// Its wording is chosen to fit TierRich's own minimum width (14) with
// no truncation at all — "cond temp rain" is exactly 14 columns — so
// the single most informative header does not itself become the thing
// that gets clipped mid-word the moment a terminal sits right at that
// boundary, the way "cond  hi/lo  rain%" (Cell's own double-spaced,
// unabbreviated wording) did at the very common 120-column/7-day case.
func CellHeader(width int, tier Tier) string {
	if width <= 0 || tier == TierNone {
		return strings.Repeat(" ", width)
	}

	// One column short of each tier's own minimum width, deliberately:
	// a label that exactly fills its cell leaves Pad nothing to add, so
	// two day columns run together with no gap between them ("cond
	// tempcond temp") at whichever width happens to make cellW equal
	// the label's own length — 80 columns, very commonly, at
	// TierMedium. Reserving one trailing column guarantees a separator
	// survives at that tier's tightest common width.
	var body string
	switch tier {
	case TierMinimal:
		body = "cond"
	case TierMedium:
		body = "cond tmp"
	default:
		body = "cond temp rain"
	}
	return Pad(body, width)
}
