package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/tsoueid/weatherdash/internal/weather"
	"github.com/tsoueid/weatherdash/internal/widgets"
)

// renderGrid draws the matrix screen: a pinned day header above a
// scrolling band of city rows.
//
// The header renders outside the scroll region so the day labels stay
// visible at any scroll position — with twenty cities, a header that
// scrolls away makes the grid unreadable.
func (m Model) renderGrid() string {
	// No rows to show is either a first run or a filter that matched
	// nothing — two situations that deserve different words.
	if len(m.cities) == 0 {
		return m.renderEmpty()
	}

	l := m.layout()

	lines := make([]string, 0, m.h)
	lines = append(lines, m.dayHeader(l))
	lines = append(lines, m.fieldHeader(l))
	lines = append(lines, widgets.Pad(strings.Repeat("─", m.w), m.w))
	if m.filtering || m.filterQuery != "" {
		lines[2] = widgets.Pad(m.filter.View(), m.w)
	}

	body := 0
	for i := m.scroll; i < len(m.cities) && body < l.rows; i++ {
		lines = append(lines, m.cityRow(i, l)...)
		body++
	}

	// Pad the body so the footer always sits on the same line.
	used := len(lines)
	for used < m.h-1 {
		lines = append(lines, strings.Repeat(" ", m.w))
		used++
	}

	lines = append(lines, m.footer(m.w))
	frame := strings.Join(lines[:m.h], "\n")
	if m.confirmDelete {
		frame = m.confirmDeleteModal(frame)
	}
	return frame
}

// confirmDeleteModal pastes the delete confirmation over the middle of
// the frame: a bordered box on the banner grey — the way the rest of
// the UI raises surfaces — with the city named and the keys spelled
// out. Nothing else on screen is redrawn; the grid stays behind the
// modal as context for the decision.
func (m Model) confirmDeleteModal(frame string) string {
	c, ok := m.selectedCity()
	if !ok {
		return frame
	}
	th := newTheme()
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("214")).
		Background(flagPanelColor).
		Padding(0, 3).
		Render(th.warn.Bold(true).Render("Delete "+c.Name+"?") + "\n" +
			whiteOn(flagPanelColor).Render("enter or y delete · esc cancels"))

	rows := strings.Split(box, "\n")
	base := strings.Split(frame, "\n")
	left := (m.w - lipglossWidth(widestLine(rows))) / 2
	if left < 0 {
		left = 0
	}
	top := (m.h - len(rows)) / 2
	for i, r := range rows {
		y := top + i
		if y < 0 || y >= len(base) {
			continue
		}
		base[y] = widgets.Splice(base[y], left, r)
	}
	return strings.Join(base, "\n")
}

// dayHeader renders the pinned column labels.
//
// Labels come from the viewer's local clock. No single weekday label
// can be literally correct for every row at once, because each city's
// day 0 is its own local today; the viewer's frame is the one that
// matches how people plan, and the detail screen shows the selected
// city's exact local date where precision matters.
func (m Model) dayHeader(l layout) string {
	th := newTheme()
	now := m.now()

	var b strings.Builder
	b.WriteString(strings.Repeat(" ", l.labelW))
	for d := 0; d < l.days; d++ {
		b.WriteString(widgets.Pad(dayLabel(now, d), l.cellW))
	}
	return th.header.Render(widgets.Pad(b.String(), m.w))
}

// fieldHeader labels the pieces packed into each day cell — condition,
// hi/lo temperature, rain probability — so "☀  24/12  45%" doesn't
// make a new user guess which number is which. It repeats per day
// column at that column's own tier, via widgets.CellHeader, so the
// labels never promise more than the cells beneath them actually show.
func (m Model) fieldHeader(l layout) string {
	th := newTheme()
	var b strings.Builder
	b.WriteString(strings.Repeat(" ", l.labelW))
	for d := 0; d < l.days; d++ {
		b.WriteString(widgets.CellHeader(l.cellW, l.tier))
	}
	return th.dim.Render(widgets.Pad(b.String(), m.w))
}

// dayLabel names one column of the forecast window: "Today" for the
// first day, else its weekday and date. Every screen that labels a day
// axis — the grid header and the world map's status line — shares this
// so "Fri 19" cannot mean two different days depending which screen
// you're looking at.
func dayLabel(now time.Time, d int) string {
	if d == 0 {
		return "Today"
	}
	return now.AddDate(0, 0, d).Format("Mon 2")
}

// cityLabel renders a city's name in its column: the full name when
// it fits, else cut one column short with an ellipsis — so the cut
// says itself, and the name never runs flush against the day cells.
func cityLabel(name string, w int) string {
	if lipglossWidth(name) <= w {
		return widgets.Pad(name, w)
	}
	cut := strings.TrimRight(widgets.Truncate(name, w-1), " ")
	return widgets.Pad(cut+"…", w)
}

// cityRow renders one city as one line (compact) or three (expanded).
func (m Model) cityRow(idx int, l layout) []string {
	th := newTheme()
	city := m.cities[idx]
	f, hasData := m.data[city.ID]

	labelStyle := th.label
	if idx == m.cursorCity {
		labelStyle = th.labelSel
	}
	label := labelStyle.Render(cityLabel(city.Name, l.labelW))

	if m.density == densityCompact {
		return []string{widgets.Pad(label+m.cellsFor(idx, f, hasData, l), m.w)}
	}

	// Expanded: icons, temperatures, precipitation band.
	var icons, temps, precip strings.Builder
	icons.WriteString(cityLabel(city.Name, l.labelW))
	temps.WriteString(widgets.Pad(strings.ToUpper(city.CC), l.labelW))
	precip.WriteString(strings.Repeat(" ", l.labelW))

	for d := 0; d < l.days; d++ {
		day, ok := m.dayAt(idx, d)
		if !ok {
			icons.WriteString(strings.Repeat(" ", l.cellW))
			temps.WriteString(strings.Repeat(" ", l.cellW))
			precip.WriteString(strings.Repeat(" ", l.cellW))
			continue
		}
		icons.WriteString(widgets.Pad(weather.Icon(day.Code, m.icons), l.cellW))
		temps.WriteString(widgets.Pad(
			fmt.Sprintf("%d/%d", m.units.TempNum(day.HiC), m.units.TempNum(day.LoC)),
			l.cellW))
		badge := precipBadgeStyle(float64(day.PrecipProb)).Render(fmt.Sprintf("%d%%", day.PrecipProb))
		precip.WriteString(widgets.Pad(badge, l.cellW))
	}

	return []string{
		widgets.Pad(labelStyle.Render(icons.String()), m.w),
		widgets.Pad(temps.String(), m.w),
		widgets.Pad(th.dim.Render(precip.String()), m.w),
	}
}

// cellsFor renders the day cells of one compact row, highlighting the
// cell under the cursor.
func (m Model) cellsFor(idx int, f weather.Forecast, hasData bool, l layout) string {
	th := newTheme()
	var b strings.Builder

	for d := 0; d < l.days; d++ {
		var cell string
		if !hasData || d >= len(f.Days) {
			cell = strings.Repeat(" ", l.cellW)
		} else {
			day := f.Days[d]
			cell = widgets.Cell(day, l.cellW, l.tier, m.icons, m.units, precipBadgeStyle)
			cell = lipglossColorTemp(cell, day.HiC)
		}
		if idx == m.cursorCity && d == m.cursorDay {
			cell = th.cursor.Render(cell)
		}
		b.WriteString(cell)
	}
	return b.String()
}

// lipglossColorTemp tints a rendered cell by its daily high. Colour is
// applied to the whole cell string rather than to the digits alone
// because splitting a styled string by character would break the width
// accounting the grid depends on.
func lipglossColorTemp(cell string, hiC float64) string {
	return lipgloss.NewStyle().Foreground(tempColor(hiC)).Render(cell)
}
