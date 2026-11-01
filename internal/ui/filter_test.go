package ui

import (
	"testing"

	"github.com/tsoueid/360WeatherDashTUI/internal/weather"
)

func TestMatchesFilter(t *testing.T) {
	c := weather.City{Name: "Paris", Admin: "Île-de-France", Country: "France", CC: "FR"}
	for _, q := range []string{"par", "paris", "fran", "fr"} {
		if !matchesFilter(c, q) {
			t.Errorf("%q should match %+v", q, c)
		}
	}
	for _, q := range []string{"tokyo", "zzz"} {
		if matchesFilter(c, q) {
			t.Errorf("%q should not match %+v", q, c)
		}
	}
}

func TestFilterNarrowsTheGrid(t *testing.T) {
	m := testModel(t, 5)
	m.filterQuery = "tok"
	m = m.rebuildView()

	if len(m.cities) != 1 {
		t.Fatalf("got %d cities, want 1", len(m.cities))
	}
	if m.cities[0].Name != "Tokyo" {
		t.Errorf("filtered to %q, want Tokyo", m.cities[0].Name)
	}
}

// Filtering hides rows; it must never remove them from the saved list.
func TestFilterDoesNotTouchConfig(t *testing.T) {
	m := testModel(t, 5)
	m.filterQuery = "tok"
	m = m.rebuildView()

	if len(m.cfg.Cities) != 5 {
		t.Errorf("config holds %d cities, want all 5", len(m.cfg.Cities))
	}
}

func TestClearingTheFilterRestoresEveryRow(t *testing.T) {
	m := testModel(t, 5)
	m.filterQuery = "tok"
	m = m.rebuildView()
	m.filterQuery = ""
	m = m.rebuildView()

	if len(m.cities) != 5 {
		t.Errorf("got %d cities, want 5", len(m.cities))
	}
}

func TestFilterMatchingNothingIsSafe(t *testing.T) {
	m := testModel(t, 5)
	m.filterQuery = "zzzzz"
	m = m.rebuildView()

	if len(m.cities) != 0 {
		t.Fatalf("got %d cities, want 0", len(m.cities))
	}
	if m.cursorCity != 0 {
		t.Errorf("cursorCity = %d, want 0", m.cursorCity)
	}
	if out := m.renderGrid(); out == "" {
		t.Error("an empty filtered grid must still render chrome")
	}
}

func TestSlashOpensFilterAndEscClears(t *testing.T) {
	m := send(testModel(t, 5), "/")
	if !m.filtering {
		t.Fatal("/ must enter filter mode")
	}

	m.filterQuery = "tok"
	m = m.rebuildView()
	m = send(m, "esc")

	if m.filtering {
		t.Error("esc must leave filter mode")
	}
	if m.filterQuery != "" {
		t.Errorf("filterQuery = %q, want cleared", m.filterQuery)
	}
	if len(m.cities) != 5 {
		t.Errorf("got %d cities, want all 5 restored", len(m.cities))
	}
}
