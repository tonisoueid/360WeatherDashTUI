package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/tsoueid/weatherdash/internal/weather"
)

func TestRenderDetailLineWidths(t *testing.T) {
	for _, size := range []struct{ w, h int }{{80, 24}, {120, 40}, {60, 20}} {
		m := testModel(t, 4)
		m.w, m.h = size.w, size.h
		m.screen = screenDetail
		m.cursorCity, m.cursorDay = 1, 2

		for i, line := range strings.Split(m.renderDetail(), "\n") {
			if got := lipgloss.Width(line); got != size.w {
				t.Errorf("%dx%d line %d width = %d, want %d: %q",
					size.w, size.h, i, got, size.w, line)
			}
		}
	}
}

func TestRenderDetailShowsCityAndPosition(t *testing.T) {
	m := testModel(t, 4)
	m.cursorCity, m.cursorDay = 2, 3

	out := m.renderDetail()
	if !strings.Contains(out, "Tokyo") {
		t.Errorf("detail must name the city:\n%s", out)
	}
	if !strings.Contains(out, "4 of 7") {
		t.Errorf("detail must show 'day 4 of 7':\n%s", out)
	}
}

func TestRenderDetailShowsConditionLabel(t *testing.T) {
	m := testModel(t, 2)
	f := m.data[m.cities[0].ID]
	f.Days[0].Code = 65 // Heavy rain
	m.data[m.cities[0].ID] = f

	if !strings.Contains(m.renderDetail(), "Heavy rain") {
		t.Errorf("detail must show the condition label:\n%s", m.renderDetail())
	}
}

func TestRenderDetailWithoutData(t *testing.T) {
	m := testModel(t, 2)
	m.data = map[string]weather.Forecast{}

	out := m.renderDetail()
	if strings.TrimSpace(out) == "" {
		t.Error("detail with no data must render a message, not nothing")
	}
	for i, line := range strings.Split(out, "\n") {
		if got := lipgloss.Width(line); got != m.w {
			t.Errorf("line %d width = %d, want %d", i, got, m.w)
		}
	}
}

func TestRenderDetailEmptyCityList(t *testing.T) {
	m := New(defaultTestConfig(), emptyTestCache())
	m.w, m.h = 100, 30
	m.screen = screenDetail

	if out := m.renderDetail(); strings.TrimSpace(out) == "" {
		t.Error("detail with no cities must render something")
	}
}

func TestIsTodayComparesLocalDate(t *testing.T) {
	m := testModel(t, 1)
	city := m.cities[0]

	today := weather.Day{Date: time.Now().Truncate(24 * time.Hour)}
	if !m.isToday(today, city) {
		t.Error("today's date must be recognised as today")
	}

	future := weather.Day{Date: time.Now().AddDate(0, 0, 3).Truncate(24 * time.Hour)}
	if m.isToday(future, city) {
		t.Error("a date three days out must not be today")
	}
}
