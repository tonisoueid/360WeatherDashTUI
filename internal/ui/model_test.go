package ui

import (
	"errors"
	"testing"
	"time"

	"github.com/tsoueid/weatherdash/internal/store"
	"github.com/tsoueid/weatherdash/internal/weather"
)

var errTest = errors.New("search failed")

// testModel builds a model with n cities, each with 7 days of data.
func testModel(t *testing.T, n int) Model {
	t.Helper()

	cfg := store.DefaultConfig()
	cache := store.NewCache()
	names := []string{"Paris", "Beirut", "Tokyo", "Oslo", "Lisbon", "Berlin", "Cairo", "Lima"}

	for i := 0; i < n; i++ {
		name := names[i%len(names)]
		id := name + string(rune('0'+i))
		cfg.Cities = append(cfg.Cities, weather.City{
			ID: id, Name: name, Country: "X", TZ: "UTC",
		})

		days := make([]weather.Day, 7)
		for d := range days {
			days[d] = weather.Day{
				Date: time.Date(2026, 9, 17+d, 0, 0, 0, 0, time.UTC),
				HiC:  float64(10 + i + d),
				LoC:  float64(2 + i),
				Code: 0,
			}
		}
		cache.Forecasts[id] = weather.Forecast{CityID: id, Days: days}
	}

	m := New(cfg, cache)
	m.w, m.h = 100, 30
	return m
}

func defaultTestConfig() store.Config { return store.DefaultConfig() }
func emptyTestCache() store.Cache     { return store.NewCache() }

func TestNewLoadsCitiesAndCache(t *testing.T) {
	m := testModel(t, 3)
	if len(m.cities) != 3 {
		t.Fatalf("got %d cities, want 3", len(m.cities))
	}
	if len(m.data) != 3 {
		t.Fatalf("got %d forecasts, want 3", len(m.data))
	}
	if m.screen != screenGrid {
		t.Errorf("screen = %v, want screenGrid", m.screen)
	}
	if m.cursorCity != 0 || m.cursorDay != 0 {
		t.Errorf("cursor = %d,%d; want 0,0", m.cursorCity, m.cursorDay)
	}
}

func TestMoveCityClampsAtBothEnds(t *testing.T) {
	m := testModel(t, 4)

	m.moveCity(-1)
	if m.cursorCity != 0 {
		t.Errorf("moving up from the top gave %d, want 0", m.cursorCity)
	}

	m.moveCity(99)
	if m.cursorCity != 3 {
		t.Errorf("moving past the bottom gave %d, want 3", m.cursorCity)
	}
}

func TestMoveDayClampsAtBothEnds(t *testing.T) {
	m := testModel(t, 2)

	m.moveDay(-5)
	if m.cursorDay != 0 {
		t.Errorf("moving left from day 0 gave %d, want 0", m.cursorDay)
	}

	m.moveDay(99)
	if m.cursorDay != 6 {
		t.Errorf("moving past the last day gave %d, want 6", m.cursorDay)
	}
}

func TestCursorSurvivesEmptyCityList(t *testing.T) {
	m := New(store.DefaultConfig(), store.NewCache())
	m.w, m.h = 100, 30

	m.moveCity(1)
	m.moveDay(1)
	m.clampCursor()

	if m.cursorCity != 0 || m.cursorDay != 0 {
		t.Errorf("cursor on an empty list = %d,%d; want 0,0", m.cursorCity, m.cursorDay)
	}
	if _, ok := m.selectedCity(); ok {
		t.Error("selectedCity must report false with no cities")
	}
}

// Deleting the last city while the cursor is on it must not leave the
// cursor pointing past the end.
func TestClampAfterCityListShrinks(t *testing.T) {
	m := testModel(t, 4)
	m.cursorCity = 3

	m.cities = m.cities[:2]
	m.clampCursor()

	if m.cursorCity != 1 {
		t.Errorf("cursor = %d, want 1 after shrinking to 2 cities", m.cursorCity)
	}
}

func TestSelectedForecastAndDay(t *testing.T) {
	m := testModel(t, 3)
	m.cursorCity, m.cursorDay = 1, 4

	c, ok := m.selectedCity()
	if !ok || c.Name != "Beirut" {
		t.Fatalf("selectedCity = %+v, %v", c, ok)
	}

	f, ok := m.selectedForecast()
	if !ok {
		t.Fatal("selectedForecast reported false")
	}
	if f.CityID != c.ID {
		t.Errorf("forecast CityID = %q, want %q", f.CityID, c.ID)
	}

	d, ok := m.dayAt(1, 4)
	if !ok {
		t.Fatal("dayAt reported false")
	}
	if want := float64(10 + 1 + 4); d.HiC != want {
		t.Errorf("dayAt HiC = %v, want %v", d.HiC, want)
	}
}

// A city with no cached forecast must report cleanly, not panic.
func TestSelectedForecastMissingData(t *testing.T) {
	m := testModel(t, 2)
	m.data = map[string]weather.Forecast{}

	if _, ok := m.selectedForecast(); ok {
		t.Error("selectedForecast must report false when the city has no data")
	}
	if _, ok := m.dayAt(0, 0); ok {
		t.Error("dayAt must report false when the city has no data")
	}
}

func TestDensityRowHeight(t *testing.T) {
	if got := densityCompact.rowHeight(); got != 1 {
		t.Errorf("compact rowHeight = %d, want 1", got)
	}
	if got := densityExpanded.rowHeight(); got != 3 {
		t.Errorf("expanded rowHeight = %d, want 3", got)
	}
}
