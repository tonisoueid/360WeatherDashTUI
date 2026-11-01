package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/tsoueid/360WeatherDashTUI/internal/widgets"
)

// View dispatches to the active screen's renderer.
//
// It renders a message rather than a grid when the terminal is too
// small, because a matrix squeezed below its minimum produces torn
// borders that look like a crash.
func (m Model) View() string {
	if m.w == 0 || m.h == 0 {
		// Before the first WindowSizeMsg there is no geometry to render
		// against. Returning empty would flash a blank frame.
		return "Loading…"
	}
	if m.tooSmall() {
		return lipgloss.Place(m.w, m.h, lipgloss.Center, lipgloss.Center,
			fmt.Sprintf("Terminal too small\nneed at least %d×%d, have %d×%d",
				minTermWidth, minTermHeight, m.w, m.h))
	}

	switch m.screen {
	case screenGrid:
		return m.renderGrid()
	case screenDetail:
		return m.renderDetail()
	case screenAddCity:
		return m.renderAddCity()
	case screenHelp:
		return m.renderHelp()
	case screenMap:
		return m.renderMap()
	}
	return m.renderGrid()
}

// fill pads a block of lines to exactly the window size, so every
// screen returns a full frame and none of them has to repeat the
// arithmetic.
func (m Model) fill(lines []string) string {
	return m.fillWithFooter(lines, "")
}

// fillWithFooter pads lines and pins a hint to the bottom row.
func (m Model) fillWithFooter(lines []string, hint string) string {
	out := make([]string, 0, m.h)
	for _, l := range lines {
		out = append(out, widgets.Pad(l, m.w))
	}
	for len(out) < m.h-1 {
		out = append(out, strings.Repeat(" ", m.w))
	}
	if hint != "" {
		out = append(out, newTheme().dim.Render(widgets.Pad(hint, m.w)))
	} else {
		out = append(out, strings.Repeat(" ", m.w))
	}
	return strings.Join(out[:m.h], "\n")
}
