package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/tsoueid/weatherdash/internal/widgets"
)

// renderEmpty draws the first-run and no-matches states.
//
// The two are deliberately distinct. "Add your first city" is correct
// on a fresh install and actively misleading when the user has twenty
// cities and a filter that happens to match none of them.
func (m Model) renderEmpty() string {
	th := newTheme()

	var body string
	if m.filterQuery != "" {
		body = lipgloss.JoinVertical(lipgloss.Center,
			th.label.Render("No cities match \""+m.filterQuery+"\""),
			"",
			th.dim.Render("esc  clear the filter"),
		)
	} else {
		body = lipgloss.JoinVertical(lipgloss.Center,
			th.header.Render("weatherdash"),
			"",
			th.label.Render("Add your first city to get started."),
			"",
			th.dim.Render("a   search for a city"),
			th.dim.Render("?   show all keys"),
			th.dim.Render("q   quit"),
		)
	}

	lines := strings.Split(
		lipgloss.Place(m.w, m.h-1, lipgloss.Center, lipgloss.Center, body), "\n")

	out := make([]string, 0, m.h)
	for _, l := range lines {
		out = append(out, widgets.Pad(l, m.w))
	}
	for len(out) < m.h-1 {
		out = append(out, strings.Repeat(" ", m.w))
	}
	out = append(out, m.footer(m.w))
	return strings.Join(out[:m.h], "\n")
}
