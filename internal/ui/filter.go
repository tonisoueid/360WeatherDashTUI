package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/tsoueid/weatherdash/internal/weather"
)

// matchesFilter reports whether a city matches a lowercased query.
//
// Name, region, country, and country code are all searched, because at
// twenty cities "show me everything in France" is as common a query as
// "jump to Oslo". Matching the name alone would make the second work
// and the first fail with no explanation.
func matchesFilter(c weather.City, q string) bool {
	if q == "" {
		return true
	}
	for _, field := range []string{c.Name, c.Admin, c.Country, c.CC} {
		if strings.Contains(strings.ToLower(field), q) {
			return true
		}
	}
	return false
}

// handleFilterKey routes keys while the filter input is active.
func (m Model) handleFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Back):
		// Escape clears the filter as well as leaving the mode. A
		// lingering invisible filter is how users end up convinced the
		// app lost their cities.
		m.filtering = false
		m.filter.Blur()
		m.filter.SetValue("")
		m.filterQuery = ""
		return m.rebuildView(), nil

	case msg.Type == tea.KeyEnter:
		// Keep the filter applied but return the keyboard to the grid.
		m.filtering = false
		m.filter.Blur()
		return m, nil
	}

	var cmd tea.Cmd
	m.filter, cmd = m.filter.Update(msg)
	m.filterQuery = m.filter.Value()
	return m.rebuildView(), cmd
}
