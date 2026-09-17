package ui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// Init starts the spinner. Phase 5 adds the initial fetch and the
// auto-refresh ticker here.
func (m Model) Init() tea.Cmd {
	return m.spinner.Tick
}

// Update is a pure function of (model, message). No I/O happens here —
// network work is returned as a tea.Cmd and arrives back as a message,
// which is what makes the entire loop testable without mocks.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		m.help.Width = msg.Width
		// A resize changes how many rows fit, which can strand the
		// cursor outside the viewport.
		m.ensureCursorVisible()
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

// handleKey routes a keypress to the active screen. Phases 6 to 8 add
// the other screens' branches.
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keys.Quit) {
		return m, tea.Quit
	}

	switch m.screen {
	case screenGrid:
		return m.handleGridKey(msg)
	}
	return m, nil
}

func (m Model) handleGridKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Up):
		m.moveCity(-1)
		m.ensureCursorVisible()
	case key.Matches(msg, m.keys.Down):
		m.moveCity(1)
		m.ensureCursorVisible()
	case key.Matches(msg, m.keys.Left):
		m.moveDay(-1)
	case key.Matches(msg, m.keys.Right):
		m.moveDay(1)
	}
	return m, nil
}
