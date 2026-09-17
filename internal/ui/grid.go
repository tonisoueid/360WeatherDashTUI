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
	l := m.layout()

	lines := make([]string, 0, m.h)
	lines = append(lines, m.dayHeader(l))
	lines = append(lines, widgets.Pad(strings.Repeat("─", m.w), m.w))

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
	return strings.Join(lines[:m.h], "\n")
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
	now := time.Now()

	var b strings.Builder
	b.WriteString(strings.Repeat(" ", l.labelW))
	for d := 0; d < l.days; d++ {
		label := "Today"
		if d > 0 {
			label = now.AddDate(0, 0, d).Format("Mon 2")
		}
		b.WriteString(widgets.Pad(label, l.cellW))
	}
	return th.header.Render(widgets.Pad(b.String(), m.w))
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
	label := labelStyle.Render(widgets.Pad(city.Name, l.labelW))

	if m.density == densityCompact {
		return []string{widgets.Pad(label+m.cellsFor(idx, f, hasData, l), m.w)}
	}

	// Expanded: icons, temperatures, precipitation band.
	var icons, temps, precip strings.Builder
	icons.WriteString(widgets.Pad(city.Name, l.labelW))
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
		precip.WriteString(widgets.Pad(
			widgets.PrecipBand([]int{day.PrecipProb}, 2), l.cellW))
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
			cell = widgets.Cell(day, l.cellW, l.tier, m.icons, m.units)
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
