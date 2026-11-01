package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/tonisoueid/360WeatherDashTUI/internal/weather"
	"github.com/tonisoueid/360WeatherDashTUI/internal/widgets"
)

// mapSeaColor and mapLandColor are the world map's basemap: dim and
// deliberately colourless-adjacent, chosen to sit BELOW the
// temperature ramp (theme.go's tempRamp) rather than compete with it.
// Sea is a neutral grey rather than blue on purpose — half the ramp's
// cold end is blue, and a blue ocean would make cold cities vanish
// into it.
var (
	mapSeaColor  = lipgloss.Color("237")
	mapLandColor = lipgloss.Color("22")
)

// mapNoDataColor is the hollow marker's colour: a city with no
// forecast yet gets a dim outline rather than a temperature-ramp
// colour it hasn't earned.
var mapNoDataColor = lipgloss.Color("240")

// mapChipBG is the explicit background every marker glyph and
// temperature chip renders on.
//
// Without it, ANSI terminals leave a style's background as whatever
// was last set — and every raster glyph sets its own background
// explicitly (halfBlockStyle), so a marker or chip with only a
// foreground colour silently inherits the LAND or SEA colour sitting
// under it. tempColor's ramp includes greens ("42" at 12°C) and
// yellows that nearly vanish against mapLandColor's dark green; giving
// every marker and chip its own dark, colour-neutral tile makes the
// ramp's full range legible regardless of what terrain it happens to
// land on.
var mapChipBG = lipgloss.Color("234")

func mapTerrainColor(land bool) lipgloss.Color {
	if land {
		return mapLandColor
	}
	return mapSeaColor
}

// renderMap draws the world map screen: saved cities plotted at their
// real coordinates, each coloured and labelled by the selected day's
// temperature.
//
// It shares the grid's {cursorCity, cursorDay} cursor rather than
// keeping its own: ↑↓ moves between cities and ←→ scrubs the forecast
// window, exactly as they do everywhere else, so the map is another
// view of the same position rather than a separate place with its own
// rules.
func (m Model) renderMap() string {
	// No rows to plot is either a first run or a filter matching
	// nothing — renderEmpty already tells those two apart.
	if len(m.cities) == 0 {
		return m.renderEmpty()
	}

	availLines := m.h - 2 // one status line, one footer line
	if availLines < 2 {
		availLines = 2
	}
	w, px := widgets.MapSize(m.w, availLines)
	left := (m.w - w) / 2
	if left < 0 {
		left = 0
	}

	lines := mapRasterLines(widgets.LandRaster(w, px))
	margin := strings.Repeat(" ", left)
	for i := range lines {
		lines[i] = margin + lines[i] // centre BEFORE stamping, so marker
		// and chip columns (measured from the frame's left edge) land
		// where the projection actually put them.
	}

	markers, skipped := m.mapMarkers(w, px)
	stampMarkers(lines, left, markers)
	m.stampChips(lines, left, left+w, markers)

	frame := make([]string, 0, m.h)
	frame = append(frame, m.mapStatusLine(skipped))
	for _, l := range lines {
		frame = append(frame, widgets.Pad(l, m.w))
	}

	for len(frame) < m.h-1 {
		frame = append(frame, strings.Repeat(" ", m.w))
	}
	frame = append(frame, m.mapFooter(m.w))
	return strings.Join(frame[:m.h], "\n")
}

// mapRasterGlyph picks the block character for one terminal cell from
// its two pixels' land/sea state, independent of colour: full block
// when both halves are land, an upper or lower half-block when only
// one is, and a plain space for open sea.
//
// This is deliberate, not decorative: flagLines (flags.go) always
// renders "▀" and leans entirely on its two ANSI colours to tell one
// half from the other, which is fine for a flag's many hues but fails
// completely on a monochrome terminal, where every style lipgloss
// would apply gets stripped and the whole map would collapse into one
// undifferentiated run of the same glyph. Choosing the glyph by shape
// means the coastline survives that stripping; colour becomes a
// second, richer signal layered on top for terminals that have it,
// rather than the only one.
func mapRasterGlyph(top, bot bool) string {
	switch {
	case top && bot:
		return "█"
	case top:
		return "▀"
	case bot:
		return "▄"
	default:
		return " "
	}
}

// mapRasterLines turns a land/sea pixel grid into terminal lines, one
// per pair of pixel rows (2k, 2k+1) — the same row-pairing flagLines
// uses to rasterize a flag, but with the glyph itself carrying the
// shape (see mapRasterGlyph) and colour applied on top of it.
func mapRasterLines(raster [][]bool) []string {
	px := len(raster)
	if px == 0 {
		return nil
	}
	w := len(raster[0])

	lines := make([]string, px/2)
	for k := range lines {
		top, bot := raster[2*k], raster[2*k+1]
		var b strings.Builder
		for x := 0; x < w; x++ {
			st := halfBlockStyle(mapTerrainColor(top[x]), mapTerrainColor(bot[x]))
			b.WriteString(st.Render(mapRasterGlyph(top[x], bot[x])))
		}
		lines[k] = b.String()
	}
	return lines
}

// mapMarker is one city's plotted position, resolved once per frame so
// the drawing and labelling passes below don't repeat the lookups.
type mapMarker struct {
	col, line int // terminal column and line WITHIN THE RASTER (before centring)
	haveTemp  bool
	tempC     float64
	cursored  bool
}

// mapMarkers projects every displayed city onto a w x px pixel grid
// and resolves the temperature each should show for the selected day.
//
// Day 0 shows the live "now" reading (Forecast.Current), matching the
// detail screen's rule that a "now" figure only exists for today;
// every later day shows that day's forecast high. Cities with unset
// (0,0) coordinates — an unresolved geocode, not a real place in the
// Gulf of Guinea — and cities the projection places outside the mask's
// 85°N..60°S crop are counted as skipped rather than misplotted.
func (m Model) mapMarkers(w, px int) (markers []mapMarker, skipped int) {
	for i, c := range m.cities {
		if c.Lat == 0 && c.Lon == 0 {
			skipped++
			continue
		}
		col, row, ok := widgets.Project(c.Lat, c.Lon, w, px)
		if !ok {
			skipped++
			continue
		}

		mk := mapMarker{col: col, line: row / 2, cursored: i == m.cursorCity}
		if f, has := m.data[c.ID]; has {
			switch {
			case m.cursorDay == 0:
				mk.tempC, mk.haveTemp = f.Current.TempC, true
			case m.cursorDay < len(f.Days):
				mk.tempC, mk.haveTemp = f.Days[m.cursorDay].HiC, true
			}
		}
		markers = append(markers, mk)
	}
	return markers, skipped
}

// mapMarkerGlyph picks one city's glyph and style: a hollow ring with
// no forecast yet, a filled dot coloured by the temperature ramp once
// it has one, and a bold ring-in-a-dot for whichever city the cursor
// is on — drawn last (see stampMarkers) so it is never buried under
// another city sharing its pixel.
func mapMarkerGlyph(mk mapMarker, th theme) (string, lipglossStyle) {
	switch {
	case mk.cursored && mk.haveTemp:
		return "◉", whiteOn(tempColor(mk.tempC)).Bold(true)
	case mk.cursored:
		return "◉", whiteOn(mapNoDataColor).Bold(true)
	case mk.haveTemp:
		// An explicit background, not just a foreground colour: see
		// mapChipBG. Without it this glyph silently inherits whatever
		// terrain colour sits under it.
		return "●", styleFor(tempColor(mk.tempC)).Bold(true).Background(mapChipBG)
	default:
		return "○", th.dim.Background(mapChipBG)
	}
}

// stampMarkers overlays every city's marker glyph onto the raster
// lines, in place. widgets.Splice replays the raster's own colour for
// everything to the right of the glyph, so one marker never bleeds
// into its neighbours (measure.go).
//
// Non-cursored markers are drawn first and the cursored one last, so
// two cities sharing a pixel never hide the one the cursor is on.
func stampMarkers(lines []string, left int, markers []mapMarker) {
	th := newTheme()
	draw := func(mk mapMarker) {
		if mk.line < 0 || mk.line >= len(lines) {
			return
		}
		glyph, style := mapMarkerGlyph(mk, th)
		lines[mk.line] = widgets.Splice(lines[mk.line], left+mk.col, style.Render(glyph))
	}
	for _, mk := range markers {
		if !mk.cursored {
			draw(mk)
		}
	}
	for _, mk := range markers {
		if mk.cursored {
			draw(mk)
		}
	}
}

// mapSpan is a half-open range of terminal columns already spoken for
// on one raster line, used to keep temperature chips off each other
// and off every marker dot.
type mapSpan struct{ start, end int }

func spansOverlap(occupied []mapSpan, s mapSpan) bool {
	for _, o := range occupied {
		if s.start < o.end && o.start < s.end {
			return true
		}
	}
	return false
}

// chipGap is the blank column left between a marker dot and its
// temperature chip, so the number reads as a label beside the dot
// rather than a second half of it.
const chipGap = 1

// stampChips writes each city's bare-number temperature beside its
// marker, in place.
//
// Every marker's own column is reserved up front on its line, so a
// chip can never bury a different city's dot even on a crowded row.
// The cursored city's chip is placed first — guaranteeing it survives
// — then the rest in display order; a chip that would collide with an
// already-placed one is dropped rather than overwriting it, the same
// rule internal/widgets/sparkline.go's numberRow uses for its own
// value labels. The marker itself always stays, with or without a
// chip.
func (m Model) stampChips(lines []string, left, fullWidth int, markers []mapMarker) {
	if len(lines) == 0 {
		return
	}

	occupied := make(map[int][]mapSpan, len(markers))
	for _, mk := range markers {
		c := left + mk.col
		occupied[mk.line] = append(occupied[mk.line], mapSpan{c, c + 1})
	}

	order := make([]mapMarker, 0, len(markers))
	for _, mk := range markers {
		if mk.cursored {
			order = append(order, mk)
		}
	}
	for _, mk := range markers {
		if !mk.cursored {
			order = append(order, mk)
		}
	}

	for _, mk := range order {
		if !mk.haveTemp || mk.line < 0 || mk.line >= len(lines) {
			continue
		}
		text := strconv.Itoa(m.units.TempNum(mk.tempC))
		w := lipglossWidth(text)
		col := left + mk.col

		// Each span reserves the gap column too, not just the digits,
		// so a later chip can't land flush against this one's dot.
		right := mapSpan{col + 1, col + 1 + chipGap + w}
		leftSpan := mapSpan{col - chipGap - w, col}

		// An explicit background — see mapChipBG — for the same reason
		// the marker glyph needs one: a foreground-only style would
		// silently inherit the land or sea colour under it. The gap
		// gets the same background rather than being left as whatever
		// the raster drew there, so it reads as a deliberate blank
		// beside the dot rather than one more pixel of terrain that
		// happens to separate them.
		style := styleFor(tempColor(mk.tempC)).Bold(true).Background(mapChipBG)
		gap := lipgloss.NewStyle().Background(mapChipBG).Render(strings.Repeat(" ", chipGap))

		var at int
		var reserve mapSpan
		var ins string
		switch {
		case right.end <= fullWidth && !spansOverlap(occupied[mk.line], right):
			at, reserve, ins = right.start, right, gap+style.Render(text)
		case leftSpan.start >= 0 && !spansOverlap(occupied[mk.line], leftSpan):
			at, reserve, ins = leftSpan.start, leftSpan, style.Render(text)+gap
		default:
			continue // no room without overlapping something: keep the marker, drop the chip
		}

		lines[mk.line] = widgets.Splice(lines[mk.line], at, ins)
		// Reserve the gap along with the digits, so the NEXT chip on
		// this line can't land flush against this one either.
		occupied[mk.line] = append(occupied[mk.line], reserve)
	}
}

// mapStatusLine names the cursored city and the day on screen — the
// one piece of information the map's markers can't spell out in a
// single pixel. Every other city already carries its own temperature
// on the map; this line is what tells you WHICH dot you're looking at.
func (m Model) mapStatusLine(skipped int) string {
	th := newTheme()
	day := dayLabel(m.now(), m.cursorDay)

	c, ok := m.selectedCity()
	if !ok {
		return widgets.Pad(th.label.Render(" "+day), m.w)
	}

	var body string
	f, has := m.data[c.ID]
	switch {
	case !has:
		body = fmt.Sprintf("▸ %s   no data yet", c.Full())
	case m.cursorDay == 0:
		body = fmt.Sprintf("▸ %s   %s %s %s", c.Full(),
			weather.Icon(f.Current.Code, m.icons), m.units.Temp(f.Current.TempC), weather.Label(f.Current.Code))
	case m.cursorDay < len(f.Days):
		d := f.Days[m.cursorDay]
		body = fmt.Sprintf("▸ %s   %s %s / %s %s", c.Full(),
			weather.Icon(d.Code, m.icons), m.units.Temp(d.HiC), m.units.Temp(d.LoC), weather.Label(d.Code))
	default:
		body = fmt.Sprintf("▸ %s   no data for %s", c.Full(), day)
	}
	if skipped == 1 {
		body += " · 1 city has no coordinates"
	} else if skipped > 1 {
		body += fmt.Sprintf(" · %d cities have no coordinates", skipped)
	}

	line := th.labelSel.Render(" " + body)
	right := th.dim.Render(day)
	gap := m.w - lipglossWidth(line) - lipglossWidth(right) - 1
	if gap < 1 {
		return widgets.Pad(line, m.w)
	}
	return widgets.Pad(line+strings.Repeat(" ", gap)+right, m.w)
}
