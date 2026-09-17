package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestEmptyStateOnFirstRun(t *testing.T) {
	m := New(defaultTestConfig(), emptyTestCache())
	m.w, m.h = 100, 30

	out := m.View()
	if !strings.Contains(out, "a") || !strings.Contains(strings.ToLower(out), "add") {
		t.Errorf("the empty state must name the add key:\n%s", out)
	}
	for i, line := range strings.Split(out, "\n") {
		if got := lipgloss.Width(line); got != 100 {
			t.Errorf("line %d width = %d, want %d", i, got, m.w)
		}
	}
}

// A filter that hides every row is NOT a first run, and saying "add a
// city" there would be actively misleading.
func TestFilteredToNothingIsNotTheEmptyState(t *testing.T) {
	m := testModel(t, 5)
	m.filterQuery = "zzzzz"
	m = m.rebuildView()

	out := m.View()
	if strings.Contains(strings.ToLower(out), "add your first") {
		t.Errorf("a filtered-empty grid must not show the first-run message:\n%s", out)
	}
	if !strings.Contains(strings.ToLower(out), "match") {
		t.Errorf("a filtered-empty grid should say nothing matched:\n%s", out)
	}
}

func TestEmptyStateAtSmallSizes(t *testing.T) {
	m := New(defaultTestConfig(), emptyTestCache())
	m.w, m.h = 55, 16

	for i, line := range strings.Split(m.View(), "\n") {
		if got := lipgloss.Width(line); got != 55 {
			t.Errorf("line %d width = %d, want %d", i, got, m.w)
		}
	}
}
