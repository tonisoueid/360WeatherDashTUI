package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/tsoueid/weatherdash/internal/weather"
	"github.com/tsoueid/weatherdash/internal/widgets"
)

// detailGutter is the left margin for labelled rows.
const detailGutter = 9

// isToday reports whether a forecast day is the current local day in
// that city's timezone.
//
// The comparison happens in the city's zone, not the viewer's: at 23:00
// in Paris it is already tomorrow in Tokyo, and showing Tokyo's live
// conditions under yesterday's date would be wrong.
func (m Model) isToday(d weather.Day, city weather.City) bool {
	if d.Date.IsZero() {
		return false
	}
	loc, err := time.LoadLocation(city.TZ)
	if err != nil {
		loc = time.Local
	}
	now := time.Now().In(loc)
	return d.Date.Year() == now.Year() && d.Date.YearDay() == now.YearDay()
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
		return m.fill([]string{
			th.header.Render(city.Full()),
			"",
			th.dim.Render("No forecast data for this day yet. Press r to refresh."),
		})
	}

	// Header: who, when, and where in the window.
	title := fmt.Sprintf("%s — %s — day %d of %d",
		city.Full(), day.Date.Format("Mon 2 Jan"), m.cursorDay+1, m.days)
	lines = append(lines, th.header.Render(widgets.Pad(" "+title, m.w)))
	lines = append(lines, widgets.Pad(strings.Repeat("─", m.w), m.w))

	// Hero: condition, range, and the day's headline numbers.
	icon := weather.Icon(day.Code, m.icons)
	lines = append(lines, widgets.Pad(fmt.Sprintf("  %s  %s      %s / %s",
		icon, weather.Label(day.Code),
		m.units.Temp(day.HiC), m.units.Temp(day.LoC)), m.w))
	lines = append(lines, th.dim.Render(widgets.Pad(fmt.Sprintf(
		"       %s total · wind to %s · UV %.0f",
		m.units.Precip(day.PrecipMM), m.units.Speed(day.WindMaxKPH), day.UVMax), m.w)))

	// The current-conditions line exists only for today. A future day
	// has no "now", and showing today's live reading under Friday's
	// date would be wrong in a way that looks right.
	if m.isToday(day, city) {
		if f, ok := m.selectedForecast(); ok {
			lines = append(lines, widgets.Pad(fmt.Sprintf(
				"       now %s · feels %s · %s %s",
				m.units.Temp(f.Current.TempC), m.units.Temp(f.Current.ApparentC),
				widgets.WindArrow(f.Current.WindDir), m.units.Speed(f.Current.WindKPH)), m.w))
		}
	}
	lines = append(lines, strings.Repeat(" ", m.w))

	chartW := m.w - detailGutter - 2
	if chartW < 8 {
		chartW = 8
	}

	temps := make([]float64, 0, len(day.Hours))
	probs := make([]int, 0, len(day.Hours))
	mms := make([]float64, 0, len(day.Hours))
	for _, h := range day.Hours {
		temps = append(temps, h.TempC)
		probs = append(probs, h.PrecipProb)
		mms = append(mms, h.PrecipMM)
	}

	row := func(label, body string) string {
		return widgets.Pad("  "+widgets.Pad(label, detailGutter-2)+body, m.w)
	}

	lines = append(lines, row("Temp",
		widgets.Sparkline(temps, chartW, func(v float64) lipglossStyle {
			return styleFor(tempColor(v))
		})))
	lines = append(lines, row("Rain", widgets.PrecipBand(probs, chartW)))
	lines = append(lines, row("", th.dim.Render(widgets.Sparkline(mms, chartW, nil))))

	arrows, speeds := widgets.WindRow(day.Hours, chartW, m.units)
	lines = append(lines, row("Wind", arrows))
	lines = append(lines, row("", th.dim.Render(speeds)))
	lines = append(lines, row("", th.dim.Render(hourAxis(chartW))))
	lines = append(lines, strings.Repeat(" ", m.w))

	lines = append(lines, row("Sun", fmt.Sprintf("%s %s %s",
		day.Sunrise.Format("15:04"),
		widgets.SunArc(day.Sunrise, day.Sunset, time.Now(), chartW-14),
		day.Sunset.Format("15:04"))))

	return m.fillWithFooter(lines,
		" ←→ day · ↑↓ city · esc back · u units · ? help")
}

// hourAxis labels a 24-hour chart every three hours.
func hourAxis(width int) string {
	axis := []rune(strings.Repeat(" ", width))
	for h := 0; h < 24; h += 3 {
		col := h * width / 24
		label := fmt.Sprintf("%02d", h)
		for i, r := range label {
			if col+i < width {
				axis[col+i] = r
			}
		}
	}
	return string(axis)
}
