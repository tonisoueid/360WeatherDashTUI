package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/tsoueid/360WeatherDashTUI/internal/widgets"
)

// renderAddCity draws the city search modal.
func (m Model) renderAddCity() string {
	th := newTheme()
	lines := []string{
		th.header.Render(widgets.Pad(" Add a city", m.w)),
		widgets.Pad(strings.Repeat("─", m.w), m.w),
		widgets.Pad(m.search.View(), m.w),
		strings.Repeat(" ", m.w),
	}

	switch {
	case m.searchErr != nil:
		lines = append(lines, th.warn.Render(
			widgets.Pad("  "+m.searchErr.Error(), m.w)))
	case m.searching:
		lines = append(lines, th.dim.Render(
			widgets.Pad("  "+m.spinner.View()+" searching…", m.w)))
	case len(m.searchResults) == 0 && m.search.Value() != "":
		lines = append(lines, th.dim.Render(
			widgets.Pad("  no matches", m.w)))
	default:
		for i, p := range m.searchResults {
			// The region is what makes these rows distinguishable; a
			// list of eight identical "Paris" entries is a coin flip,
			// not a picker.
			row := fmt.Sprintf("  %s  (%.2f, %.2f)", p.Label(), p.Latitude, p.Longitude)
			if i == m.searchSel {
				row = th.cursor.Render(widgets.Pad(row, m.w))
			} else {
				row = widgets.Pad(row, m.w)
			}
			lines = append(lines, row)
		}
	}

	return m.fillWithFooter(lines, " type to search · ↑↓ select · ⏎ add · esc cancel")
}

// handleAddCityKey routes keys while the modal is open.
func (m Model) handleAddCityKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Back):
		m.screen = screenGrid
		m.search.Blur()
		m.search.SetValue("")
		m.searchResults = nil
		m.searchErr = nil
		m.searchSel = 0
		return m, nil

	case msg.Type == tea.KeyUp:
		if m.searchSel > 0 {
			m.searchSel--
		}
		return m, nil

	case msg.Type == tea.KeyDown:
		if m.searchSel < len(m.searchResults)-1 {
			m.searchSel++
		}
		return m, nil

	case msg.Type == tea.KeyEnter:
		return m.commitSelectedPlace()
	}

	// Anything else is text. Each edit bumps the sequence and schedules
	// a fresh debounce; the timers left over from earlier keystrokes
	// expire without firing because their seq no longer matches.
	var cmd tea.Cmd
	m.search, cmd = m.search.Update(msg)

	m.searchSeq++
	m.searchErr = nil
	return m, tea.Batch(cmd, debounceCmd(m.search.Value(), m.searchSeq))
}

// commitSelectedPlace adds the highlighted result, persists it, and
// triggers a refresh so the new row fills in immediately.
func (m Model) commitSelectedPlace() (tea.Model, tea.Cmd) {
	if m.searchSel < 0 || m.searchSel >= len(m.searchResults) {
		return m, nil
	}

	next, err := m.addCity(m.searchResults[m.searchSel].ToCity())
	if err != nil {
		// Stay in the modal so the user can pick a different result.
		m.searchErr = err
		return m, nil
	}
	m = next

	m.screen = screenGrid
	m.search.Blur()
	m.search.SetValue("")
	m.searchResults = nil
	m.searchSel = 0
	m.searchErr = nil

	m, fetch := m.refreshCmd()
	return m, tea.Batch(saveConfigCmd(m.configPath, m.cfg), fetch)
}
