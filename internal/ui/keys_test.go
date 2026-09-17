package ui

import (
	"testing"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

func TestKeyMapMatchesVimAndArrows(t *testing.T) {
	k := defaultKeyMap()
	tests := []struct {
		name    string
		binding key.Binding
		keys    []string
	}{
		{"up", k.Up, []string{"up", "k"}},
		{"down", k.Down, []string{"down", "j"}},
		{"left", k.Left, []string{"left", "h"}},
		{"right", k.Right, []string{"right", "l"}},
	}
	for _, tc := range tests {
		for _, s := range tc.keys {
			msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
			if len(s) > 1 {
				msg = tea.KeyMsg{Type: keyTypeFor(s)}
			}
			if !key.Matches(msg, tc.binding) {
				t.Errorf("%s binding does not match %q", tc.name, s)
			}
		}
	}
}

func TestDistinctBindingsForAddAndFilter(t *testing.T) {
	k := defaultKeyMap()
	add := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")}
	filter := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")}

	if !key.Matches(add, k.Add) {
		t.Error("a must trigger Add")
	}
	if key.Matches(add, k.Filter) {
		t.Error("a must NOT also trigger Filter — searching and filtering are different operations")
	}
	if !key.Matches(filter, k.Filter) {
		t.Error("/ must trigger Filter")
	}
}

func TestEveryBindingHasHelpText(t *testing.T) {
	k := defaultKeyMap()
	for _, row := range k.FullHelp() {
		for _, b := range row {
			h := b.Help()
			if h.Key == "" || h.Desc == "" {
				t.Errorf("binding %v has incomplete help text %+v", b.Keys(), h)
			}
		}
	}
}

func TestShortHelpIsNonEmpty(t *testing.T) {
	if len(defaultKeyMap().ShortHelp()) == 0 {
		t.Error("ShortHelp must list the footer bindings")
	}
}

// keyTypeFor maps the named keys used in the table to their tea types.
func keyTypeFor(s string) tea.KeyType {
	switch s {
	case "up":
		return tea.KeyUp
	case "down":
		return tea.KeyDown
	case "left":
		return tea.KeyLeft
	case "right":
		return tea.KeyRight
	default:
		return tea.KeyRunes
	}
}
