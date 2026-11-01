package ui

import (
	"testing"
	"time"

	// The timezone sorts must resolve IANA zones; embedding tzdata
	// keeps the test binary hermetic on systems without /usr/share/zoneinfo.
	_ "time/tzdata"

	"github.com/tonisoueid/360WeatherDashTUI/internal/store"
	"github.com/tonisoueid/360WeatherDashTUI/internal/weather"
)

func TestSortModeCycles(t *testing.T) {
	const modes = 9
	seen := map[sortMode]bool{}
	s := sortManual
	for i := 0; i < modes; i++ {
		seen[s] = true
		s = s.Next()
	}
	if len(seen) != modes {
		t.Errorf("cycling visited %d modes, want %d", len(seen), modes)
	}
	if s != sortManual {
		t.Errorf("cycling %d times ended on %v, want back at sortManual", modes, s)
	}
}

func TestSortWarmestOrdersByMeanHigh(t *testing.T) {
	// testModel gives city i a mean high that rises with i.
	m := testModel(t, 4)
	m.sortMode = sortWarmest
	m = m.rebuildView()

	first, last := m.cities[0].ID, m.cities[len(m.cities)-1].ID
	if first != m.cfg.Cities[3].ID {
		t.Errorf("warmest first = %q, want the highest-index city", first)
	}
	if last != m.cfg.Cities[0].ID {
		t.Errorf("warmest last = %q, want the lowest-index city", last)
	}
}

func TestSortColdestIsTheReverse(t *testing.T) {
	warm := testModel(t, 4)
	warm.sortMode = sortWarmest
	warm = warm.rebuildView()

	cold := testModel(t, 4)
	cold.sortMode = sortColdest
	cold = cold.rebuildView()

	if warm.cities[0].ID != cold.cities[3].ID {
		t.Error("coldest must be the reverse of warmest")
	}
}

// Sorting must never rewrite the saved order.
func TestSortDoesNotMutateConfig(t *testing.T) {
	m := testModel(t, 4)
	before := make([]string, len(m.cfg.Cities))
	for i, c := range m.cfg.Cities {
		before[i] = c.ID
	}

	m.sortMode = sortWarmest
	m = m.rebuildView()

	for i, c := range m.cfg.Cities {
		if c.ID != before[i] {
			t.Fatalf("config order changed at %d: %q, want %q", i, c.ID, before[i])
		}
	}
}

func TestSortReturnsToManualOrder(t *testing.T) {
	m := testModel(t, 4)
	m.sortMode = sortWarmest
	m = m.rebuildView()
	m.sortMode = sortManual
	m = m.rebuildView()

	for i, c := range m.cities {
		if c.ID != m.cfg.Cities[i].ID {
			t.Errorf("manual order not restored at %d: %q, want %q", i, c.ID, m.cfg.Cities[i].ID)
		}
	}
}

// Pressing s must keep the cursor on the same CITY, not the same row.
func TestSortPreservesCursorCity(t *testing.T) {
	m := testModel(t, 5)
	m.cursorCity = 1
	want := m.cities[1].ID

	m.sortMode = sortWarmest
	m = m.rebuildView()

	if got := m.cities[m.cursorCity].ID; got != want {
		t.Errorf("cursor landed on %q, want it to follow %q", got, want)
	}
}

func TestSortPutsCitiesWithoutDataLast(t *testing.T) {
	m := testModel(t, 3)
	orphan := m.cities[0].ID
	delete(m.data, orphan)

	m.sortMode = sortWarmest
	m = m.rebuildView()

	if m.cities[len(m.cities)-1].ID != orphan {
		t.Errorf("city without data is at %v, want last", m.cities)
	}
}

func TestParseSortModeRoundTrip(t *testing.T) {
	for _, s := range []sortMode{sortManual, sortWarmest, sortColdest, sortDriest,
		sortAZ, sortZA, sortEast, sortWest, sortGMT} {
		if got := parseSortMode(s.String()); got != s {
			t.Errorf("parseSortMode(%q) = %v, want %v", s.String(), got, s)
		}
	}
	if got := parseSortMode("nonsense"); got != sortManual {
		t.Errorf("unknown sort = %v, want sortManual", got)
	}
}

func TestSortAlphabeticalAZ(t *testing.T) {
	m := testModel(t, 5) // Paris, Beirut, Tokyo, Oslo, Lisbon
	m.sortMode = sortAZ
	m = m.rebuildView()

	want := []string{"Beirut", "Lisbon", "Oslo", "Paris", "Tokyo"}
	for i, name := range want {
		if m.cities[i].Name != name {
			t.Fatalf("A-Z order = %v, want %v", cityNames(m.cities), want)
		}
	}
}

func TestSortAlphabeticalZA(t *testing.T) {
	m := testModel(t, 5)
	m.sortMode = sortZA
	m = m.rebuildView()

	want := []string{"Tokyo", "Paris", "Oslo", "Lisbon", "Beirut"}
	for i, name := range want {
		if m.cities[i].Name != name {
			t.Fatalf("Z-A order = %v, want %v", cityNames(m.cities), want)
		}
	}
}

// Alphabetical order must not exile a city without forecast data: the
// name ordering does not depend on what has fetched.
func TestSortAlphabeticalIgnoresDataAvailability(t *testing.T) {
	m := testModel(t, 3)
	delete(m.data, m.cities[1].ID) // Beirut has no data

	m.sortMode = sortAZ
	m = m.rebuildView()

	if m.cities[0].Name != "Beirut" {
		t.Errorf("Beirut is at index 0's place = %q; data-less cities must sort by name, not last",
			m.cities[0].Name)
	}
}

// tzModel spans the globe with the clock pinned, so every zone's DST
// state is fixed regardless of when the test runs.
func tzModel(t *testing.T) Model {
	t.Helper()
	zones := []struct{ name, tz string }{
		{"Tokyo", "Asia/Tokyo"},               // +9
		{"Paris", "Europe/Paris"},             // +2 (CEST in September)
		{"Accra", "Africa/Accra"},             // 0, no DST
		{"NewYork", "America/New_York"},       // -4 (EDT)
		{"LosAngeles", "America/Los_Angeles"}, // -7 (PDT)
	}

	cfg := store.DefaultConfig()
	cache := store.NewCache()
	for _, z := range zones {
		cfg.Cities = append(cfg.Cities, weather.City{
			ID: z.name, Name: z.name, TZ: z.tz, Country: "X",
		})
		cache.Forecasts[z.name] = weather.Forecast{
			CityID: z.name, Days: []weather.Day{{HiC: 10}},
		}
	}

	m := New(cfg, cache)
	m.nowFunc = func() time.Time {
		return time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	}
	return m
}

func TestSortTimezoneEastmostFirst(t *testing.T) {
	m := tzModel(t)
	m.sortMode = sortEast
	m = m.rebuildView()

	want := []string{"Tokyo", "Paris", "Accra", "NewYork", "LosAngeles"}
	for i, name := range want {
		if m.cities[i].Name != name {
			t.Fatalf("eastmost-first order = %v, want %v", cityNames(m.cities), want)
		}
	}
}

func TestSortTimezoneWestmostFirst(t *testing.T) {
	m := tzModel(t)
	m.sortMode = sortWest
	m = m.rebuildView()

	want := []string{"LosAngeles", "NewYork", "Accra", "Paris", "Tokyo"}
	for i, name := range want {
		if m.cities[i].Name != name {
			t.Fatalf("westmost-first order = %v, want %v", cityNames(m.cities), want)
		}
	}
}

// GMT first means closest to UTC+0: 0, then ±2, ±4, … by distance.
func TestSortTimezoneGMTFirst(t *testing.T) {
	m := tzModel(t)
	m.sortMode = sortGMT
	m = m.rebuildView()

	want := []string{"Accra", "Paris", "NewYork", "LosAngeles", "Tokyo"}
	for i, name := range want {
		if m.cities[i].Name != name {
			t.Fatalf("GMT-first order = %v, want %v", cityNames(m.cities), want)
		}
	}
}

func cityNames(cities []weather.City) []string {
	names := make([]string, len(cities))
	for i, c := range cities {
		names[i] = c.Name
	}
	return names
}
