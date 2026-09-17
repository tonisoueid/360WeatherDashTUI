package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// Every line of the grid must be exactly the terminal width, or the
// right-hand border tears.
func TestRenderGridLineWidths(t *testing.T) {
	for _, size := range []struct{ w, h int }{
		{80, 24}, {120, 40}, {60, 20}, {100, 16},
	} {
		m := testModel(t, 8)
		m.w, m.h = size.w, size.h

		out := m.renderGrid()
		for i, line := range strings.Split(out, "\n") {
			if got := lipgloss.Width(line); got != size.w {
				t.Errorf("%dx%d line %d width = %d, want %d: %q",
					size.w, size.h, i, got, size.w, line)
			}
		}
	}
}

func TestRenderGridShowsCityNames(t *testing.T) {
	m := testModel(t, 3)
	out := m.renderGrid()

	for _, want := range []string{"Paris", "Beirut", "Tokyo"} {
		if !strings.Contains(out, want) {
			t.Errorf("grid is missing %q:\n%s", want, out)
		}
	}
}

func TestRenderGridHeaderLabelsToday(t *testing.T) {
	m := testModel(t, 2)
	if !strings.Contains(m.renderGrid(), "Today") {
		t.Errorf("grid header must label the first column Today:\n%s", m.renderGrid())
	}
}

func TestRenderGridScrollsToWindow(t *testing.T) {
	m := testModel(t, 20)
	m.w, m.h = 100, 14
	m.cursorCity = 19
	m.ensureCursorVisible()

	out := m.renderGrid()
	if strings.Contains(out, "Paris0") {
		t.Error("scrolled grid must not show the first city")
	}
}

func TestExpandedDensityAddsRows(t *testing.T) {
	m := testModel(t, 3)
	m.w, m.h = 120, 40

	// renderGrid always emits a full-height frame, so density is
	// measured in content lines rather than total lines.
	contentLines := func(s string) int {
		n := 0
		for _, line := range strings.Split(s, "\n") {
			if strings.TrimSpace(line) != "" {
				n++
			}
		}
		return n
	}

	compact := contentLines(m.renderGrid())
	m.density = densityExpanded
	expanded := contentLines(m.renderGrid())

	if expanded <= compact {
		t.Errorf("expanded grid has %d content lines, compact %d; expanded must be taller",
			expanded, compact)
	}
}

func TestRenderGridEmptyCityList(t *testing.T) {
	m := New(defaultTestConfig(), emptyTestCache())
	m.w, m.h = 100, 30

	out := m.renderGrid()
	if strings.TrimSpace(out) == "" {
		t.Error("an empty city list must still render chrome, not nothing")
	}
	for i, line := range strings.Split(out, "\n") {
		if got := lipgloss.Width(line); got != 100 {
			t.Errorf("line %d width = %d, want 100", i, got)
		}
	}
}

func TestTempColorRampIsMonotonic(t *testing.T) {
	// Distinct temperatures across the range must not all collapse to
	// one colour, or the grid stops being scannable.
	seen := map[lipgloss.Color]bool{}
	for _, c := range []float64{-20, -5, 5, 15, 25, 35, 45} {
		seen[tempColor(c)] = true
	}
	if len(seen) < 5 {
		t.Errorf("temp ramp produced only %d distinct colours, want at least 5", len(seen))
	}
}
