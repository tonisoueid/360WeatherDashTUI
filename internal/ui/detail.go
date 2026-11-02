package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/tonisoueid/360WeatherDashTUI/internal/units"
	"github.com/tonisoueid/360WeatherDashTUI/internal/weather"
	"github.com/tonisoueid/360WeatherDashTUI/internal/widgets"
)

// detailGutter is the left margin for labelled rows.
const detailGutter = 9

// isToday reports whether a forecast day is the current local day in
// that city's timezone.
//
// The comparison happens in the city's zone, not the viewer's: at 23:00
// in Paris it is already tomorrow in Tokyo, and showing Tokyo's live
// conditions under yesterday's date would be wrong. The model's
// injectable clock is used so tests can pin it.
func (m Model) isToday(d weather.Day, city weather.City) bool {
	if d.Date.IsZero() {
		return false
	}
	loc, err := time.LoadLocation(city.TZ)
	if err != nil {
		loc = time.Local
	}
	now := m.now().In(loc)
	return d.Date.Year() == now.Year() && d.Date.YearDay() == now.YearDay()
}

// headerBlock lays out the detail header as one light-grey banner,
// matching the flag's mount: the flag pinned flush to the upper
// right, the temperature scale before it with grey room to breathe,
// and the title on the flag's vertically centred line. With no flag
// for the country the block is the title line alone.
func (m Model) headerBlock(th theme, title, cc string) []string {
	flag := flagLines(cc)
	if flag == nil {
		return []string{widgets.Pad(th.header.Render(" "+title), m.w)}
	}

	// Grey breathing room between the scale and the flag's colours.
	const scaleGap = 3

	flagW := lipglossWidth(flag[0])
	legend := m.tempLegend()
	legendW := lipglossWidth(widestLine(legend))

	leftW := m.w - flagW - legendW - scaleGap
	if leftW < 2 {
		leftW = 2
	}

	// The title rides the upper of the flag's two middle lines: an
	// even-height flag has no exact centre row, and sitting high reads
	// as centred.
	titleRow := (len(flag) - 1) / 2
	// The title is bold orange on the grey banner: the app's orange
	// reads loudly there, where grey-on-grey would sink.
	titleSt := th.warn.Bold(true).Background(flagPanelColor)

	rows := make([]string, len(flag))
	for i := range flag {
		var left string
		if i == titleRow {
			left = titleSt.Render(widgets.Truncate(" "+title, leftW))
			if pad := leftW - lipglossWidth(left); pad > 0 {
				left += flagPanel.Render(strings.Repeat(" ", pad))
			}
		} else {
			left = flagPanel.Render(strings.Repeat(" ", leftW))
		}
		var mid string
		if i < len(legend) {
			mid = legend[i]
		}
		rows[i] = widgets.Pad(left+mid+
			flagPanel.Render(strings.Repeat(" ", scaleGap))+flag[i], m.w)
	}
	return rows
}

// tempLegend explains the temperature colour scale as a vertical
// bar: hottest at the top, coldest at the bottom, one ramp stop per
// half-row (the same fg-top/bg-bottom trick the flags use), with the
// ends labelled in the viewer's units. It shares the header banner's
// rows, just before the flag.
func (m Model) tempLegend() []string {
	stops := len(tempRamp)
	lo, hi := tempRamp[0], tempRamp[stops-1]

	loTxt, hiTxt := m.units.Temp(lo.c), m.units.Temp(hi.c)
	w := len([]rune(loTxt))
	if hw := len([]rune(hiTxt)); hw > w {
		w = hw
	}

	rows := make([]string, stops/2)
	for r := range rows {
		top := tempRamp[stops-1-2*r] // the hotter half of the line
		bot := tempRamp[stops-2-2*r]
		bar := halfBlockStyle(top.color, bot.color).Render("▀")

		var label string
		lr := 0
		labelSt := whiteOn(flagPanelColor)
		switch r {
		case 0:
			label = labelSt.Render(hiTxt)
			lr = len([]rune(hiTxt))
		case len(rows) - 1:
			label = labelSt.Render(loTxt)
			lr = len([]rune(loTxt))
		}
		rows[r] = flagPanel.Render(strings.Repeat(" ", w+1-lr)) + label + bar
	}
	return rows
}

// renderDetail draws one city on one day: the zoomed view of a single
// matrix cell.
func (m Model) renderDetail() string {
	th := newTheme()
	lines := make([]string, 0, m.h)

	city, hasCity := m.selectedCity()
	if !hasCity {
		return m.fill([]string{th.dim.Render("No cities saved. Press a to add one.")})
	}

	day, hasDay := m.dayAt(m.cursorCity, m.cursorDay)
	if !hasDay {
		lines := m.headerBlock(th, city.Full(), city.CC)
		lines = append(lines, "", th.dim.Render("No forecast data for this day yet. Press r to refresh."))
		return m.fill(lines)
	}

	// Header: who, when, and where in the window. The day counter sits
	// in parentheses off the date; the flag art is pinned to the upper
	// right with the title vertically centred beside it — a one-line
	// glyph budget cannot render a cross or a canton honestly.
	title := fmt.Sprintf("%s — %s (day %d of %d)",
		city.Full(), day.Date.Format("Mon 2 Jan"), m.cursorDay+1, m.days)
	lines = append(lines, m.headerBlock(th, title, city.CC)...)
	lines = append(lines, widgets.Pad(strings.Repeat("─", m.w), m.w))

	// No hero block above the ruler: the now line is the overview. The
	// live condition rides the top of the line, the current conditions
	// ride its column, and the charts carry the day's shape — a second
	// summary in the upper left would say the same thing twice.
	chartW := m.w - detailGutter - 2
	if chartW < 8 {
		chartW = 8
	}

	row := func(label, body string) string {
		return widgets.Pad("  "+widgets.Pad(label, detailGutter-2)+body, m.w)
	}

	// The now line exists only when the day on screen is the city's
	// today — a "now" on Friday is a lie. Its column is the city's
	// current time on the same midnight-to-midnight scale the charts
	// use, read from the city's own clock, so it moves horizontally
	// with the city's offset from the viewer's.
	var now *nowMark
	if m.isToday(day, city) {
		if f, ok := m.selectedForecast(); ok {
			loc, err := time.LoadLocation(city.TZ)
			if err != nil {
				loc = time.Local
			}
			n := m.now().In(loc)
			now = &nowMark{t: n, cond: weather.Cond(f.Current.Code)}

			// The whole current read wears the theme's boldest white —
			// bold, colour 15 — because it is the one line the eye
			// should land on first. It rides the line's column,
			// hanging left when the line sits too close to the right
			// edge for the text.
			overview := th.labelSel.Render(fmt.Sprintf(
				"Now : %s - %s",
				weather.Label(f.Current.Code), m.units.Temp(f.Current.TempC)))
			x := detailGutter + dayColumn(n, chartW)
			if x+lipglossWidth(overview) > m.w {
				x = m.w - lipglossWidth(overview)
			}
			if x < 0 {
				x = 0
			}
			lines = append(lines, widgets.Pad(strings.Repeat(" ", x)+overview, m.w))
		}
	}
	lines = append(lines, strings.Repeat(" ", m.w))
	lineTop := len(lines) - 1 // the line drops from just under the overview

	// The Time rows lead the sections: the sun stamps ride above the
	// hour ruler they are read against — the same midnight-to-midnight
	// scale — at their true positions, so no hour marker is hidden.
	stamps, ruler := m.timeAxis(th, chartW, day)
	lines = append(lines, row("Time", stamps))
	lines = append(lines, row("", ruler))
	lines = append(lines, strings.Repeat(" ", m.w))

	temps := make([]float64, 0, len(day.Hours))
	probs := make([]int, 0, len(day.Hours))
	mms := make([]float64, 0, len(day.Hours))
	for _, h := range day.Hours {
		temps = append(temps, h.TempC)
		probs = append(probs, h.PrecipProb)
		mms = append(mms, h.PrecipMM)
	}

	// A blank line after every section: four rulers in a row leave the
	// eye nowhere to rest between them.
	blank := strings.Repeat(" ", m.w)

	// The temperature chart rides between its bounds: the day's high
	// drawn as a rule above, the low below, each in the temperature's
	// own ramp colour with its value at the line's end.
	// The band triples in height when the screen has room: everything
	// laid out so far, the band with its two bounds, the blank after
	// it, the rain and wind sections below (7 lines), and the footer.
	bandRows := 1
	if len(lines)+2+3+7+1 <= m.h {
		bandRows = 3
	}
	lines = append(lines, row("Temp", tempBound(day.HiC, chartW, m.units)))
	for _, band := range widgets.SparklineRows(temps, chartW, bandRows,
		func(v float64) lipglossStyle { return styleFor(tempColor(v)) }) {
		lines = append(lines, row("", band))
	}
	lines = append(lines, row("", tempBound(day.LoC, chartW, m.units)))
	lines = append(lines, blank)

	// Rain is two different questions on the same hour axis, so the
	// rows label themselves and never touch: the chance per hour, its
	// value — percent sign and all — printed under each hour mark and
	// wearing the deepening blue of the chance it states, and below
	// it, on its own row, how much falls when it does — the actual mm
	// per hour, in the blues of their intensity, dry hours a bare
	// dash. The blank line between them keeps "will it rain" and
	// "how much" from reading as one chart.
	lines = append(lines, row("Rain", widgets.PrecipNumbers(probs, chartW,
		func(pct float64) lipglossStyle { return styleFor(precipColor(pct)) })))
	lines = append(lines, blank)
	lines = append(lines, row("mm/h", widgets.PrecipAmounts(mms, chartW,
		func(v float64) lipglossStyle { return styleFor(amountColor(v)) })))
	lines = append(lines, blank)

	// Wind: direction above, speed below. Each speed figure wears the
	// colour of its own strength — white to red across the Beaufort
	// breaks — so a dangerous gust stands out before its digits are
	// read.
	arrows, speeds := widgets.WindRow(day.Hours, chartW, m.units,
		func(kph float64) lipglossStyle { return styleFor(windColor(kph)) })
	lines = append(lines, row("Wind", arrows))
	lines = append(lines, row("", speeds))

	// The now line itself: a vertical rule at the city's current
	// column, spliced through the rendered chart area after the fact —
	// that way it wears its own colour, the condition's, without
	// disturbing the styling either side of the cut. Where a value
	// stands in the column — an hour digit, a percentage, a speed —
	// the line yields: a gap in the rule is honest, an overwritten
	// number is not.
	if now != nil {
		x := detailGutter + dayColumn(now.t, chartW)
		st := styleFor(condColors[now.cond])
		for i := lineTop; i < len(lines); i++ {
			if !nowLineCrosses(widgets.CellAt(lines[i], x)) {
				continue
			}
			lines[i] = widgets.Splice(lines[i], x, st.Render("│"))
		}
	}

	return m.fillWithFooter(lines, joinHints(th, detailHints(th)))
}

// nowMark is the current-time mark on the detail charts: the city's
// clock position and its live condition, which colours the now line.
type nowMark struct {
	t    time.Time // the city's current time, in the city's zone
	cond weather.Condition
}

// dayColumn maps a time of day to its column on the shared
// midnight-to-midnight scale, the same formula the hour ruler and the
// sun stamps use.
func dayColumn(t time.Time, width int) int {
	return (t.Hour()*60 + t.Minute()) * width / 1440
}

// nowLineCrosses reports whether the vertical now line may replace
// the cell c: blanks and chart glyphs yes, anything that reads as a
// value or a word no.
func nowLineCrosses(c rune) bool {
	switch c {
	case ' ', '▁', '▂', '▃', '▄', '▅', '▆', '▇', '█',
		'·', '░', '▒', '▓', '─', '↑', '↓', '←', '→', '↖', '↗', '↘', '↙':
		return true
	}
	return false
}

// tempBound draws one bound of the temperature chart: a horizontal
// rule in the temperature's ramp colour, the value at both ends —
// beside the gutter on the left, and anchoring the line's right edge
// when the width can hold both.
func tempBound(c float64, width int, u units.System) string {
	label := []rune(u.Temp(c))
	rule := []rune(strings.Repeat("─", width))

	put := func(start int) {
		if start < 0 {
			start = 0
		}
		for i, r := range label {
			if start+i < width {
				rule[start+i] = r
			}
		}
	}
	put(0)
	if len(label) < width {
		rule[len(label)] = ' '
	}
	if width >= 2*len(label)+2 {
		put(width - len(label))
	}
	return styleFor(tempColor(c)).Render(string(rule))
}

// timeAxis renders the day's ruler and the stamps that ride above it.
// Each hour that fits is labelled at its column with its unit —
// "00h", "01h" — on the same midnight-to-midnight scale the charts
// use; at narrow widths a label that would overrun its neighbour is
// skipped, not written over it. The stamps sit on their own row,
// sunrise in gold and sunset in orange, each centred on its true
// minute-of-day column; a stamp that would overlap one already placed
// is dropped whole rather than interleaved with it.
func (m Model) timeAxis(th theme, width int, day weather.Day) (stamps, ruler string) {
	buf := []rune(strings.Repeat(" ", width))
	next := 0 // first column still free
	for h := 0; h < 24; h++ {
		s := fmt.Sprintf("%02dh", h)
		col := h * width / 24
		if col < next {
			continue
		}
		for i, r := range s {
			if col+i < width {
				buf[col+i] = r
			}
		}
		next = col + len(s)
	}

	type stamp struct {
		t    time.Time
		text string
		st   lipglossStyle
	}
	marks := make([]stamp, 0, 2)
	if !day.Sunrise.IsZero() {
		marks = append(marks, stamp{day.Sunrise, "☀↑" + day.Sunrise.Format("15:04"), condStyle(weather.CondClear)})
	}
	if !day.Sunset.IsZero() {
		marks = append(marks, stamp{day.Sunset, "☀↓" + day.Sunset.Format("15:04"), th.warn.Bold(true)})
	}

	// Each stamp is centred on its minute-of-day column. A stamp that
	// would overlap one already placed is dropped — two stamps sharing
	// columns would read as one garbled word — and the survivors are
	// sorted left to right for the span walk below.
	sbuf := []rune(strings.Repeat(" ", width))
	type span struct {
		start, end int
		st         lipglossStyle
	}
	spans := make([]span, 0, len(marks))
	for _, mk := range marks {
		runes := []rune(mk.text)
		col := (mk.t.Hour()*60 + mk.t.Minute()) * width / 1440
		start := col - len(runes)/2
		if start < 0 {
			start = 0
		}
		end := start + len(runes)
		if end > width {
			end = width
		}
		clash := false
		for _, s := range spans {
			if start < s.end && s.start < end {
				clash = true
				break
			}
		}
		if clash {
			continue
		}
		for i, r := range runes {
			if start+i < width {
				sbuf[start+i] = r
			}
		}
		spans = append(spans, span{start, end, mk.st})
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].start < spans[j].start })

	var b strings.Builder
	pos := 0
	for _, s := range spans {
		if s.start < pos {
			continue
		}
		b.WriteString(strings.Repeat(" ", s.start-pos))
		b.WriteString(s.st.Render(string(sbuf[s.start:s.end])))
		pos = s.end
	}
	return b.String(), th.label.Bold(true).Render(string(buf))
}
