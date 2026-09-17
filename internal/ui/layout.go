package ui

import "github.com/tsoueid/weatherdash/internal/widgets"

const (
	// minTermWidth is the narrowest terminal the matrix is usable in.
	// Below this the grid is replaced by a message rather than drawn
	// as garbage.
	minTermWidth  = 50
	minTermHeight = 15

	// labelWidth is the city-name column.
	labelWidth = 12

	// gridChromeLines is the day header plus the footer.
	gridChromeLines = 3

	// minDayColumns is the fewest day columns worth showing. Below
	// three the matrix stops being a comparison view.
	minDayColumns = 3

	// scrollOff keeps this many rows visible past the cursor, so the
	// selected city is never flush against the edge.
	scrollOff = 1
)

// layout is the geometry for one render, derived from the window size.
// It is computed once per frame and passed down, so every cell in the
// grid shares one tier and one width.
type layout struct {
	labelW int
	days   int
	cellW  int
	tier   widgets.Tier
	rows   int
}

// tooSmall reports whether the terminal is below the usable minimum.
func (m Model) tooSmall() bool {
	return m.w < minTermWidth || m.h < minTermHeight
}

// layout computes the grid geometry for the current window size.
//
// When the configured forecast window does not fit, day columns are
// dropped from the right rather than narrowed past TierMinimal: a cell
// too small for its content truncates "24/12" into "24/1", which is
// worse than not showing the day at all.
func (m Model) layout() layout {
	days := m.days
	if days < 1 {
		days = 1
	}

	avail := m.w - labelWidth
	if avail < 0 {
		avail = 0
	}

	cellW := 0
	for days >= minDayColumns {
		cellW = avail / days
		if cellW >= widgets.TierMinimal.MinWidth() {
			break
		}
		days--
	}
	if days < minDayColumns {
		days = minDayColumns
		cellW = avail / days
	}

	rows := (m.h - gridChromeLines) / m.density.rowHeight()
	if rows < 1 {
		rows = 1
	}

	return layout{
		labelW: labelWidth,
		days:   days,
		cellW:  cellW,
		tier:   widgets.TierFor(cellW),
		rows:   rows,
	}
}

// ensureCursorVisible adjusts scroll so the selected city stays in the
// viewport, keeping scrollOff rows of context where possible.
func (m *Model) ensureCursorVisible() {
	if len(m.cities) == 0 {
		m.scroll = 0
		return
	}

	rows := m.layout().rows

	// All rows fit: never scroll.
	if len(m.cities) <= rows {
		m.scroll = 0
		return
	}

	top := m.cursorCity - scrollOff
	if m.scroll > top {
		m.scroll = top
	}
	bottom := m.cursorCity - rows + 1 + scrollOff
	if m.scroll < bottom {
		m.scroll = bottom
	}

	if max := len(m.cities) - rows; m.scroll > max {
		m.scroll = max
	}
	if m.scroll < 0 {
		m.scroll = 0
	}
}
