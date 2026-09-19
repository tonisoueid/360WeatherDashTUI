package ui

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/tsoueid/weatherdash/internal/weather"
)

// theme holds the styles used across screens. Styles live here rather
// than inline so the palette can be changed in one place.
type theme struct {
	header     lipgloss.Style
	label      lipgloss.Style
	labelSel   lipgloss.Style
	cursor     lipgloss.Style
	dim        lipgloss.Style
	warn       lipgloss.Style
	hintSorted lipgloss.Style
	hintCity   lipgloss.Style
	hintDay    lipgloss.Style
	hintZoom   lipgloss.Style
	hintAdd    lipgloss.Style
	hintFilter lipgloss.Style
	hintSort   lipgloss.Style
	hintHelp   lipgloss.Style
	hintBack   lipgloss.Style
	hintUnits  lipgloss.Style
	hintMap    lipgloss.Style
	footerText lipgloss.Style
}

func newTheme() theme {
	return theme{
		header:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("252")),
		label:    lipgloss.NewStyle().Foreground(lipgloss.Color("250")),
		labelSel: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")),
		cursor:   lipgloss.NewStyle().Reverse(true),
		dim:      lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		warn:     lipgloss.NewStyle().Foreground(lipgloss.Color("214")),

		// Footer hints are bold with one colour per function, so each
		// command separates from its neighbours before the line is
		// actually read. Separators between them stay dim. The grid
		// and detail footers never share the screen, so the detail's
		// back and units colours reuse hues from the grid's palette.
		hintSorted: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("141")), // purple
		hintCity:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39")),  // blue
		hintDay:    lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("51")),  // cyan
		hintZoom:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("42")),  // green
		hintAdd:    lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("213")), // pink
		hintFilter: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("227")), // yellow
		hintSort:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("208")), // orange
		hintHelp:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")),  // white
		hintBack:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("213")), // pink
		hintUnits:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("227")), // yellow
		hintMap:    lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("203")), // salmon
		footerText: lipgloss.NewStyle().Bold(true),
	}
}

// tempStop is one anchor of the temperature colour ramp.
type tempStop struct {
	c     float64
	color lipgloss.Color
}

// tempRamp runs cold blue through to hot red. The stops are spaced by
// perceived temperature rather than evenly, so the interesting band
// (roughly 0-30°C, where most weather people care about lives) gets
// most of the resolution.
var tempRamp = []tempStop{
	{-20, lipgloss.Color("57")}, // deep blue
	{-5, lipgloss.Color("39")},  // blue
	{5, lipgloss.Color("51")},   // cyan
	{12, lipgloss.Color("42")},  // green
	{18, lipgloss.Color("227")}, // yellow
	{25, lipgloss.Color("214")}, // orange
	{32, lipgloss.Color("202")}, // deep orange
	{40, lipgloss.Color("196")}, // red
}

// tempColor maps a Celsius value to a ramp colour.
//
// Colour is what makes the matrix scannable before it is readable: the
// eye finds the warm row without parsing any digits. It is applied to
// the temperature text only, never as a cell background, so the grid
// stays legible in terminals with unusual palettes.
func tempColor(c float64) lipgloss.Color {
	if c <= tempRamp[0].c {
		return tempRamp[0].color
	}
	for i := 1; i < len(tempRamp); i++ {
		if c <= tempRamp[i].c {
			return tempRamp[i].color
		}
	}
	return tempRamp[len(tempRamp)-1].color
}

// precipColor maps a rain probability to the colour its figure wears:
// white when rain is barely a possibility, then the theme's own blues
// deepening — pale, rain, showers — as the chance rises. The breaks
// are the same decision boundaries the grid's shading uses, so a
// figure and its glyph always agree on where "might rain" begins, and
// the hues are the ones conditions already wear, so rain reads in one
// family everywhere on screen.
func precipColor(pct float64) lipgloss.Color {
	switch {
	case pct < 10:
		return lipgloss.Color("15") // white
	case pct < 30:
		return lipgloss.Color("153") // pale blue
	case pct < 60:
		return lipgloss.Color("39") // rain blue
	default:
		return lipgloss.Color("27") // deep blue
	}
}

// precipBadgeStyle pairs precipColor's hue with a readable foreground,
// for the rain-probability figure the grid prints inside its own
// coloured tile ("45%") rather than as plain text.
//
// This is deliberately a full style with an explicit BACKGROUND, not
// just precipColor used as a foreground: a foreground-only style on an
// ANSI terminal silently inherits whatever background was last set,
// which for a cell packed next to other coloured text is unpredictable
// and sometimes unreadable — the exact bug the world map's markers had
// (internal/ui/worldmap.go's mapChipBG) before they got their own
// explicit tile. precipColor's own palette runs light-to-dark (white,
// pale blue, rain blue, deep blue), so the badge's text flips from
// dark to white at the same boundary the colour turns from pale to
// saturated, keeping every probability legible on its own background.
func precipBadgeStyle(pct float64) lipgloss.Style {
	bg := precipColor(pct)
	fg := lipgloss.Color("234") // near-black, for the pale end of the ramp
	if pct >= 30 {
		fg = lipgloss.Color("15") // white, for the saturated blues
	}
	return lipgloss.NewStyle().Bold(true).Background(bg).Foreground(fg)
}

// amountColor maps an hourly rainfall in mm to its figure's colour.
// The breaks are the meteorological rain-intensity classes — drizzle,
// moderate, heavy, torrential — and the hues are the same deepening
// blues the chance figures wear, so blue means the same thing on both
// rain rows; the fifth stop goes violet for the torrential hours a
// dashboard exists to warn about, bright enough to stay readable on a
// dark terminal.
func amountColor(mmph float64) lipgloss.Color {
	switch {
	case mmph < 1:
		return lipgloss.Color("153") // drizzle
	case mmph < 4:
		return lipgloss.Color("39") // moderate rain
	case mmph < 10:
		return lipgloss.Color("27") // heavy rain
	default:
		return lipgloss.Color("57") // torrential
	}
}

// lipglossWidth is a local alias so callers do not each import lipgloss
// just to measure a string.
func lipglossWidth(s string) int { return lipgloss.Width(s) }

// windColor maps a wind speed in km/h to the colour its figure wears:
// white for low, yellow for moderate, orange for high, red for
// extreme. The breaks are the Beaufort scale's own — 20 km/h where
// "moderate breeze" begins, 39 where wind turns strong, 62 where gale
// force starts and walking against it becomes hard — so the colour
// says the same thing a forecaster would. It is keyed on the canonical
// km/h value, not the displayed figure, so toggling units does not
// move a gust across a colour boundary.
func windColor(kph float64) lipgloss.Color {
	switch {
	case kph < 20:
		return lipgloss.Color("15") // white
	case kph < 39:
		return lipgloss.Color("227") // yellow
	case kph < 62:
		return lipgloss.Color("214") // orange
	default:
		return lipgloss.Color("196") // red
	}
}

// lipglossStyle aliases the style type so detail.go can name it without
// importing lipgloss directly.
type lipglossStyle = lipgloss.Style

// styleFor builds a foreground style from a colour.
func styleFor(c lipgloss.Color) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(c)
}

// halfBlockStyle builds a style for a half-block glyph: its top half
// in one colour, its bottom half in another.
func halfBlockStyle(top, bot lipgloss.Color) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(top).Background(bot)
}

// whiteOn renders white text over a background colour.
func whiteOn(bg lipgloss.Color) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Background(bg)
}

// condColors maps a weather condition to the colour people expect it
// to wear. The logic is the folk kind: sunshine in gold that pales as
// cloud takes it over, greys for cloud and fog, blues that deepen as
// the water gets heavier, white for snow, and a storm purple loud
// enough to read as a warning.
var condColors = map[weather.Condition]lipgloss.Color{
	weather.CondClear:   lipgloss.Color("227"), // sun gold
	weather.CondPartly:  lipgloss.Color("221"), // paler sun
	weather.CondCloudy:  lipgloss.Color("245"), // light overcast grey
	weather.CondFog:     lipgloss.Color("241"), // murky grey
	weather.CondDrizzle: lipgloss.Color("153"), // pale blue
	weather.CondRain:    lipgloss.Color("39"),  // blue
	weather.CondShowers: lipgloss.Color("27"),  // deep blue
	weather.CondSnow:    lipgloss.Color("15"),  // white
	weather.CondThunder: lipgloss.Color("135"), // storm purple
	weather.CondUnknown: lipgloss.Color("240"), // dim, like th.dim
}

// condStyle renders a condition glyph or its meaning in the colour
// people expect for that weather.
func condStyle(c weather.Condition) lipglossStyle {
	return lipgloss.NewStyle().Bold(true).Foreground(condColors[c])
}
