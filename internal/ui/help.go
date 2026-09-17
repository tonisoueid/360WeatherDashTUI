package ui

import (
	"fmt"
	"strings"

	"github.com/tsoueid/weatherdash/internal/widgets"
)

// helpGroups names each row returned by keyMap.FullHelp.
var helpGroups = []string{"Navigation", "Cities", "Display"}

// renderHelp draws the full key reference.
//
// It is generated from the keyMap rather than written out by hand, so a
// new binding documents itself. A hand-maintained help screen drifts
// the moment a key changes, and it drifts silently.
func (m Model) renderHelp() string {
	th := newTheme()
	lines := []string{
		th.header.Render(widgets.Pad(" Keys", m.w)),
		widgets.Pad(strings.Repeat("─", m.w), m.w),
	}

	for i, group := range m.keys.FullHelp() {
		name := "Other"
		if i < len(helpGroups) {
			name = helpGroups[i]
		}
		lines = append(lines, th.label.Render(widgets.Pad("  "+name, m.w)))
		for _, b := range group {
			h := b.Help()
			lines = append(lines, widgets.Pad(
				fmt.Sprintf("    %s  %s",
					widgets.Pad(h.Key, 10), th.dim.Render(h.Desc)), m.w))
		}
		lines = append(lines, strings.Repeat(" ", m.w))
	}

	return m.fillWithFooter(lines, " esc or ? to close")
}
