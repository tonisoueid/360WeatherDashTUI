package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/tsoueid/weatherdash/internal/weather"
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

func TestHelpShowsIconLegendBesideKeys(t *testing.T) {
	prev := lipgloss.ColorProfile()
	defer lipgloss.SetColorProfile(prev)
	lipgloss.SetColorProfile(termenv.ANSI256)

	m := send(testModel(t, 3), "?")
	out := m.renderHelp()

	// Every glyph the grid can draw appears next to its meaning, so
	// the legend teaches the full vocabulary, and the keys half is
	// still there beside it.
	for _, e := range weather.IconLegend(m.icons) {
		if !strings.Contains(out, e.Glyph) {
			t.Errorf("help is missing glyph %q:\n%s", e.Glyph, out)
		}
		if !strings.Contains(out, e.Meaning) {
			t.Errorf("help is missing meaning %q:\n%s", e.Meaning, out)
		}
	}
	if !strings.Contains(out, "Navigation") {
		t.Errorf("help is missing the key groups:\n%s", out)
	}

	// The legend must track the configured icon set: it teaches the
	// glyphs the grid is actually drawing.
	m.icons = weather.IconsASCII
	if ascii := m.renderHelp(); strings.Contains(ascii, "☀") {
		t.Errorf("ascii legend must not show unicode glyphs:\n%s", ascii)
	}
	m.icons = weather.IconsNerd
	if nerd := m.renderHelp(); !strings.Contains(nerd, weather.Icon(3, weather.IconsNerd)) {
		t.Errorf("nerdfont legend must show nerdfont glyphs:\n%s", nerd)
	}
}

func TestHelpLegendColorCodedByCondition(t *testing.T) {
	prev := lipgloss.ColorProfile()
	defer lipgloss.SetColorProfile(prev)
	lipgloss.SetColorProfile(termenv.ANSI256)

	// Every condition in the legend wears a colour, and no two share
	// one: a shared colour would make two meanings indistinguishable
	// and the legend wrong as a key.
	seen := map[weather.Condition]lipgloss.Color{}
	for _, e := range weather.IconLegend(weather.IconsUnicode) {
		c, ok := condColors[e.Cond]
		if !ok {
			t.Errorf("condition %v has no colour", e.Cond)
			continue
		}
		for other, oc := range seen {
			if oc == c {
				t.Errorf("conditions %v and %v share colour %v", other, e.Cond, c)
			}
		}
		seen[e.Cond] = c
	}

	// The rendered legend styles glyph and meaning alike in the
	// condition's colour.
	m := send(testModel(t, 3), "?")
	out := m.renderHelp()
	for _, e := range weather.IconLegend(m.icons) {
		styled := condStyle(e.Cond)
		if !strings.Contains(out, styled.Render(e.Glyph)) {
			t.Errorf("glyph %q not rendered in its condition colour:\n%s", e.Glyph, out)
		}
		if !strings.Contains(out, styled.Render(e.Meaning)) {
			t.Errorf("meaning %q not rendered in its condition colour:\n%s", e.Meaning, out)
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
