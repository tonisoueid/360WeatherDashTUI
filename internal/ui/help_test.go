package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestHelpOpensAndCloses(t *testing.T) {
	m := send(testModel(t, 3), "?")
	if m.screen != screenHelp {
		t.Fatalf("screen = %v, want screenHelp", m.screen)
	}

	m = send(m, "esc")
	if m.screen != screenGrid {
		t.Errorf("screen = %v, want screenGrid", m.screen)
	}
}

func TestHelpReturnsToTheScreenItWasOpenedFrom(t *testing.T) {
	m := send(testModel(t, 3), "enter", "?", "esc")
	if m.screen != screenDetail {
		t.Errorf("screen = %v, want to return to screenDetail", m.screen)
	}
}

func TestHelpListsEveryBinding(t *testing.T) {
	m := send(testModel(t, 3), "?")
	out := m.renderHelp()

	for _, want := range []string{"add city", "filter", "sort", "units", "quit"} {
		if !strings.Contains(out, want) {
			t.Errorf("help is missing %q:\n%s", want, out)
		}
	}
}

func TestHelpLineWidths(t *testing.T) {
	m := send(testModel(t, 3), "?")
	for i, line := range strings.Split(m.renderHelp(), "\n") {
		if got := lipgloss.Width(line); got != m.w {
			t.Errorf("line %d width = %d, want %d", i, got, m.w)
		}
	}
}
