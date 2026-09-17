package ui

import "github.com/charmbracelet/lipgloss"

// theme holds the styles used across screens. Styles live here rather
// than inline so the palette can be changed in one place.
type theme struct {
	header   lipgloss.Style
	label    lipgloss.Style
	labelSel lipgloss.Style
	cursor   lipgloss.Style
	dim      lipgloss.Style
	warn     lipgloss.Style
}

func newTheme() theme {
	return theme{
		header:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("252")),
		label:    lipgloss.NewStyle().Foreground(lipgloss.Color("250")),
		labelSel: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")),
		cursor:   lipgloss.NewStyle().Reverse(true),
		dim:      lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		warn:     lipgloss.NewStyle().Foreground(lipgloss.Color("214")),
	}
}

// tempStop is one anchor of the temperature colour ramp.
type tempStop struct {
	c     float64
	color lipgloss.Color
}

// tempRamp runs cold blue through to hot red. The stops are spaced by
// perceived temperature rather than evenly, so the interesting band
// (roughly 0-30°C, where most weather people care about lives) gets
// most of the resolution.
var tempRamp = []tempStop{
	{-20, lipgloss.Color("57")}, // deep blue
	{-5, lipgloss.Color("39")},  // blue
	{5, lipgloss.Color("51")},   // cyan
	{12, lipgloss.Color("42")},  // green
	{18, lipgloss.Color("227")}, // yellow
	{25, lipgloss.Color("214")}, // orange
	{32, lipgloss.Color("202")}, // deep orange
	{40, lipgloss.Color("196")}, // red
}

// tempColor maps a Celsius value to a ramp colour.
//
// Colour is what makes the matrix scannable before it is readable: the
// eye finds the warm row without parsing any digits. It is applied to
// the temperature text only, never as a cell background, so the grid
// stays legible in terminals with unusual palettes.
func tempColor(c float64) lipgloss.Color {
	if c <= tempRamp[0].c {
		return tempRamp[0].color
	}
	for i := 1; i < len(tempRamp); i++ {
		if c <= tempRamp[i].c {
			return tempRamp[i].color
		}
	}
	return tempRamp[len(tempRamp)-1].color
}

// lipglossWidth is a local alias so callers do not each import lipgloss
// just to measure a string.
func lipglossWidth(s string) int { return lipgloss.Width(s) }
