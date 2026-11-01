package widgets

import (
	"fmt"
	"strings"

	"github.com/tonisoueid/360WeatherDashTUI/internal/units"
	"github.com/tonisoueid/360WeatherDashTUI/internal/weather"
)

// Tier is how much detail a grid cell shows. Narrow terminals drop to
// lower tiers rather than truncating, because a clipped "24/1" is worse
// than an honest "24".
type Tier int

const (
	// TierNone is too narrow to render anything.
	TierNone Tier = iota
	// TierMinimal is icon plus the daily high: "O  24".
	TierMinimal
	// TierMedium adds the low, with room to breathe: "O   24/ 12".
	TierMedium
	// TierRich adds the rain probability as a number, not a glyph:
	// "O   24/ 12   45%".
	TierRich
)

// Sub-field widths shared by Cell and CellHeader, so a header word and
// the data beneath it start in exactly the same column instead of
// merely approximating it.
//
// Every number is fixed-width and right-aligned (iconW is always 1:
// weather.IconWidth), not sized to whatever that particular value
// happens to need. A day with "9/3" and a day with "-12/-30" must
// still start their rain figure at the same column, or scrolling down
// a column of dates turns into a zig-zag — the exact defect a
// screenshot of this grid surfaced: two-digit and three-character
// temperatures shifted the rain percentage sideways row by row.
const (
	iconW = 1
	gapW  = 2 // breathing room between sections
	tempW = 7 // "%3d/%3d": "-12/-30" .. "  9/  3", always 7 columns
	rainW = 4 // "%3d%%":   "  0%"    .. " 100%",  always 4 columns
)

// MinWidth is the narrowest cell that can render this tier.
//
// Each value is the tier's fixed content width exactly — iconW, gapW,
// tempW and rainW leave no row-to-row slack to undercount, unlike a
// format string sized off one example value. A tier whose own content
// no longer fits is the "clipped '24/1'" failure this package's own
// doc comment warns against.
func (t Tier) MinWidth() int {
	switch t {
	case TierRich:
		return iconW + gapW + tempW + gapW + rainW
	case TierMedium:
		return iconW + gapW + tempW
	case TierMinimal:
		return iconW + 1 + 3 // icon, one space, a right-aligned "%3d" high
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
		body = fmt.Sprintf("%s %3d", icon, hi)
	case TierMedium:
		body = fmt.Sprintf("%s  %3d/%3d", icon, hi, u.TempNum(d.LoC))
	default:
		rain := fmt.Sprintf("%3d%%", d.PrecipProb)
		if rainStyle != nil {
			rain = rainStyle(float64(d.PrecipProb)).Render(rain)
		}
		body = fmt.Sprintf("%s  %3d/%3d  %s", icon, hi, u.TempNum(d.LoC), rain)
	}
	return Pad(body, width)
}

// CellHeader labels the sections a Cell at this tier packs together —
// condition, hi/lo temperature, rain probability — so the column
// header reads as a legend for the data beneath it. It never claims a
// section a cell at this tier doesn't actually render.
//
// It does not try to line up letter-for-letter with Cell's own
// icon/gap/temp/gap/rain columns: the icon's section is only iconW+gapW
// (3) columns wide, too narrow to hold any legible abbreviation of
// "condition" AND a separating space, and an early version that forced
// the attempt produced "cndtemp" — the two words running together are
// worse than not aligning at all. Each tier's MinWidth is now sized to
// Cell's own fixed-width content (see the constants above), which is
// wider than these plain-word headers ever need, so simple single-space
// wording always has room to spare and never merges into the next day
// column the way a tighter, exactly-sized string could.
func CellHeader(width int, tier Tier) string {
	if width <= 0 || tier == TierNone {
		return strings.Repeat(" ", width)
	}

	var body string
	switch tier {
	case TierMinimal:
		body = "cond"
	case TierMedium:
		body = "cond temp"
	default:
		body = "cond temp rain"
	}
	return Pad(body, width)
}
