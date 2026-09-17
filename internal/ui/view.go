package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
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
	}
	return m.renderGrid()
}
