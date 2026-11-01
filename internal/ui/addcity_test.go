package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/tsoueid/360WeatherDashTUI/internal/geocode"
)

func parisResults() []geocode.Place {
	return []geocode.Place{
		{ID: 1, Name: "Paris", Admin1: "Île-de-France", Country: "France",
			CC: "FR", Latitude: 48.85, Longitude: 2.35, Timezone: "Europe/Paris"},
		{ID: 2, Name: "Paris", Admin1: "Texas", Country: "United States",
			CC: "US", Latitude: 33.66, Longitude: -95.55, Timezone: "America/Chicago"},
	}
}

func TestAOpensSearch(t *testing.T) {
	m := send(testModel(t, 3), "a")
	if m.screen != screenAddCity {
		t.Fatalf("screen = %v, want screenAddCity", m.screen)
	}
	if !m.search.Focused() {
		t.Error("the search input must be focused on open")
	}
}

func TestEscClosesSearchWithoutAdding(t *testing.T) {
	m := send(testModel(t, 3), "a", "esc")
	if m.screen != screenGrid {
		t.Errorf("screen = %v, want screenGrid", m.screen)
	}
	if len(m.cities) != 3 {
		t.Errorf("got %d cities, want the original 3", len(m.cities))
	}
}

func TestRenderAddCityShowsDisambiguatedResults(t *testing.T) {
	m := send(testModel(t, 2), "a")
	m.searchResults = parisResults()

	out := m.renderAddCity()
	if !strings.Contains(out, "Île-de-France") || !strings.Contains(out, "Texas") {
		t.Errorf("results must be disambiguated by region:\n%s", out)
	}
	for i, line := range strings.Split(out, "\n") {
		if got := lipgloss.Width(line); got != m.w {
			t.Errorf("line %d width = %d, want %d", i, got, m.w)
		}
	}
}

func TestSearchResultSelectionMoves(t *testing.T) {
	m := send(testModel(t, 2), "a")
	m.searchResults = parisResults()

	m = send(m, "down")
	if m.searchSel != 1 {
		t.Errorf("searchSel = %d, want 1", m.searchSel)
	}
	m = send(m, "down", "down")
	if m.searchSel != 1 {
		t.Errorf("searchSel = %d, want it clamped to 1", m.searchSel)
	}
	m = send(m, "up", "up")
	if m.searchSel != 0 {
		t.Errorf("searchSel = %d, want 0", m.searchSel)
	}
}

func TestTypingSchedulesDebounce(t *testing.T) {
	m := send(testModel(t, 2), "a")
	before := m.searchSeq

	next, cmd := m.Update(keyMsg("P"))
	m = next.(Model)

	if m.searchSeq <= before {
		t.Errorf("searchSeq = %d, want greater than %d", m.searchSeq, before)
	}
	if cmd == nil {
		t.Error("typing must schedule a debounce command")
	}
}

// Only the newest debounce may fire a request.
func TestStaleDebounceIsIgnored(t *testing.T) {
	m := send(testModel(t, 2), "a")
	m.searchSeq = 5

	_, cmd := m.Update(debounceMsg{seq: 2, query: "Par"})
	if cmd != nil {
		t.Error("a superseded debounce must not trigger a search")
	}
}

func TestStaleGeocodeResultIsIgnored(t *testing.T) {
	m := send(testModel(t, 2), "a")
	m.searchSeq = 5
	m.searchResults = parisResults()

	next, _ := m.Update(geocodeMsg{seq: 2, results: nil})
	m = next.(Model)

	if len(m.searchResults) != 2 {
		t.Error("a superseded geocode response must not clear current results")
	}
}

func TestGeocodeErrorShownInline(t *testing.T) {
	m := send(testModel(t, 2), "a")
	m.searchSeq = 3

	next, _ := m.Update(geocodeMsg{seq: 3, err: errTest})
	m = next.(Model)

	if m.screen != screenAddCity {
		t.Error("a search error must leave the modal open")
	}
	if m.searchErr == nil {
		t.Error("the error must be recorded for inline display")
	}
}
