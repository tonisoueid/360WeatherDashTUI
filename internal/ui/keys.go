// Package ui holds the Bubble Tea model. A single shared {city, day}
// cursor drives both the grid and the detail screen, so zooming in and
// out never loses position.
package ui

import "github.com/charmbracelet/bubbles/key"

// keyMap declares every binding once, so the Update switch and the help
// footer read from the same source. Matching raw key strings inside
// Update instead would let the displayed help drift from the actual
// behaviour.
type keyMap struct {
	Up       key.Binding
	Down     key.Binding
	Left     key.Binding
	Right    key.Binding
	Enter    key.Binding
	Back     key.Binding
	Add      key.Binding
	Delete   key.Binding
	Filter   key.Binding
	Sort     key.Binding
	Density  key.Binding
	Units    key.Binding
	Map      key.Binding
	Refresh  key.Binding
	MoveUp   key.Binding
	MoveDown key.Binding
	Help     key.Binding
	Quit     key.Binding
}

func defaultKeyMap() keyMap {
	return keyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "city up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "city down"),
		),
		Left: key.NewBinding(
			key.WithKeys("left", "h"),
			key.WithHelp("←/h", "previous day"),
		),
		Right: key.NewBinding(
			key.WithKeys("right", "l"),
			key.WithHelp("→/l", "next day"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("⏎", "zoom in"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),
		// Add and Filter are deliberately separate: searching the world
		// for a new city and narrowing the rows you already have are
		// different operations, and merging them creates mode confusion.
		Add: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "add city"),
		),
		Delete: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "delete city"),
		),
		Filter: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "filter"),
		),
		Sort: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "sort"),
		),
		Density: key.NewBinding(
			key.WithKeys("z"),
			key.WithHelp("z", "density"),
		),
		Units: key.NewBinding(
			key.WithKeys("u"),
			key.WithHelp("u", "units"),
		),
		// Map toggles the home view, grid to world map and back. It is
		// not a persisted preference like Density or Units — it always
		// opens on the grid — so it lives with the other display keys
		// without a config field alongside it.
		Map: key.NewBinding(
			key.WithKeys("m"),
			key.WithHelp("m", "world map"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		MoveUp: key.NewBinding(
			key.WithKeys("K"),
			key.WithHelp("K", "move city up"),
		),
		MoveDown: key.NewBinding(
			key.WithKeys("J"),
			key.WithHelp("J", "move city down"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
	}
}

// ShortHelp is the one-line footer hint.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Right, k.Enter, k.Add, k.Filter, k.Sort, k.Help, k.Quit}
}

// FullHelp is the ? overlay, grouped by purpose.
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Left, k.Right, k.Enter, k.Back},
		{k.Add, k.Delete, k.Filter, k.Sort, k.MoveUp, k.MoveDown},
		{k.Density, k.Units, k.Map, k.Refresh, k.Help, k.Quit},
	}
}
