package ui

import (
	"fmt"
	"strings"

	"github.com/tsoueid/weatherdash/internal/weather"
	"github.com/tsoueid/weatherdash/internal/widgets"
)

// helpGroups names each row returned by keyMap.FullHelp.
var helpGroups = []string{"Navigation", "Cities", "Display"}

// renderHelp draws the full key reference beside a legend of the
// weather icons the grid prints to the left of each cell.
//
// The keys half is generated from the keyMap rather than written out by
// hand, so a new binding documents itself; a hand-maintained help screen
// drifts the moment a key changes, and it drifts silently. The legend
// half comes from the weather package for the configured icon set, so
// it teaches the glyphs the user is actually seeing.
func (m Model) renderHelp() string {
	th := newTheme()
	lines := []string{
		th.header.Render(widgets.Pad(" Keys", m.w)),
		widgets.Pad(strings.Repeat("─", m.w), m.w),
	}

	keys := m.helpKeys(th)
	legend := m.helpIconLegend(th)

	// Column geometry: the keys column is as wide as its widest row,
	// and the legend sits beside it. The gap breathes on wider
	// terminals and narrows to one space at the minimum width rather
	// than clipping a meaning mid-word.
	keysW := lipglossWidth(widestLine(keys))
	gap := 4
	if keysW+gap+lipglossWidth(widestLine(legend)) > m.w {
		gap = 1
	}

	// Zip the columns from the top so the legend tops out beside the
	// first key group, then spill whichever column runs longer.
	n := min(len(keys), len(legend))
	for i := 0; i < n; i++ {
		lines = append(lines, widgets.Pad(
			widgets.Pad(keys[i], keysW)+strings.Repeat(" ", gap)+legend[i], m.w))
	}
	for _, l := range keys[n:] {
		lines = append(lines, widgets.Pad(widgets.Pad(l, keysW+gap), m.w))
	}
	for _, l := range legend[n:] {
		lines = append(lines, widgets.Pad(widgets.Pad(l, keysW+gap), m.w))
	}

	return m.fillWithFooter(lines, " esc or ? to close")
}

// helpKeys lays out the key groups, one binding per row, unpadded: the
// caller measures the result to size the keys column.
func (m Model) helpKeys(th theme) []string {
	var lines []string
	for i, group := range m.keys.FullHelp() {
		name := "Other"
		if i < len(helpGroups) {
			name = helpGroups[i]
		}
		lines = append(lines, th.label.Render("  "+name))
		for _, b := range group {
			h := b.Help()
			lines = append(lines, fmt.Sprintf("    %s  %s",
				widgets.Pad(h.Key, 10), th.dim.Render(h.Desc)))
		}
		lines = append(lines, " ")
	}
	return lines
}

// helpIconLegend lays out the condition glyphs with their meanings, in
// the style of the key groups so the two columns read as one screen.
// Glyph and meaning share the colour people expect the condition to
// wear, so the legend doubles as a colour key.
func (m Model) helpIconLegend(th theme) []string {
	lines := []string{th.label.Render("  Weather")}
	for _, e := range weather.IconLegend(m.icons) {
		st := condStyle(e.Cond)
		lines = append(lines, fmt.Sprintf("  %s  %s", st.Render(e.Glyph), st.Render(e.Meaning)))
	}
	return lines
}

// widestLine returns the widest line in the set, ANSI-aware, for
// column sizing.
func widestLine(lines []string) string {
	widest := ""
	w := -1
	for _, l := range lines {
		if lw := lipglossWidth(l); lw > w {
			w, widest = lw, l
		}
	}
	return widest
}
