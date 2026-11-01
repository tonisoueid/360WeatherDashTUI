package ui

import (
	"errors"
	"fmt"
	"math"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/tsoueid/360WeatherDashTUI/internal/store"
	"github.com/tsoueid/360WeatherDashTUI/internal/weather"
)

// duplicateRadius is how close two coordinates must be to count as the
// same place, in degrees — roughly a kilometre. Geocoding the same city
// twice can return very slightly different coordinates, so exact
// equality would not catch a genuine duplicate.
const duplicateRadius = 0.01

// configSavedMsg reports the outcome of a config write.
type configSavedMsg struct{ err error }

// nearDuplicate finds an existing city at essentially the same place.
func nearDuplicate(cities []weather.City, c weather.City) (weather.City, bool) {
	for _, existing := range cities {
		if math.Abs(existing.Lat-c.Lat) < duplicateRadius &&
			math.Abs(existing.Lon-c.Lon) < duplicateRadius {
			return existing, true
		}
	}
	return weather.City{}, false
}

// addCity appends a city and syncs the saved order.
//
// Duplicates are refused rather than silently merged: two rows for the
// same place would waste a coordinate slot in every request and give
// the user two identical lines with no way to tell them apart.
func (m Model) addCity(c weather.City) (Model, error) {
	if dup, ok := nearDuplicate(m.cities, c); ok {
		return m, fmt.Errorf("%s is already saved", dup.Name)
	}
	m.cities = append(m.cities, c)
	m = m.syncConfigCities()
	return m.rebuildView(), nil
}

// deleteSelectedCity removes the city under the cursor.
func (m Model) deleteSelectedCity() Model {
	city, ok := m.selectedCity()
	if !ok {
		return m
	}

	out := make([]weather.City, 0, len(m.cities)-1)
	for _, c := range m.cities {
		if c.ID != city.ID {
			out = append(out, c)
		}
	}
	m.cities = out
	delete(m.data, city.ID)

	m.clampCursor()
	m.ensureCursorVisible()
	m = m.syncConfigCities()
	return m.rebuildView()
}

// errReorderWhileSorted explains why K and J are inert under a sort.
var errReorderWhileSorted = errors.New("press s to return to manual order before reordering")

// moveCityInOrder shifts the selected city within the SAVED order.
//
// This is one of only three operations that write to cfg.Cities (the
// others being add and delete). It is refused outside manual sort by
// its caller: reordering a temperature-sorted list is incoherent,
// because the next re-sort would undo it.
func (m Model) moveCityInOrder(delta int) Model {
	city, ok := m.selectedCity()
	if !ok {
		return m
	}

	idx := -1
	for i, c := range m.cfg.Cities {
		if c.ID == city.ID {
			idx = i
			break
		}
	}
	target := idx + delta
	if idx < 0 || target < 0 || target >= len(m.cfg.Cities) {
		return m
	}

	m.cfg.Cities[idx], m.cfg.Cities[target] = m.cfg.Cities[target], m.cfg.Cities[idx]
	return m.rebuildView()
}

// syncConfigCities copies the display order into the saved config.
//
// This is only ever called from add, delete, and manual reorder —
// never from a sort, which is a view transform. Letting a sort reach
// this function would destroy the user's manual order permanently.
func (m Model) syncConfigCities() Model {
	cities := make([]weather.City, len(m.cities))
	copy(cities, m.cities)
	m.cfg.Cities = cities
	return m
}

// saveConfigCmd persists the config.
//
// Unlike the cache, this error is reported: a failed cache write costs
// one slow startup, but a failed config write loses a city the user
// just added, and they would not find out until the next launch.
func saveConfigCmd(path string, cfg store.Config) tea.Cmd {
	if path == "" {
		return nil
	}
	return func() tea.Msg {
		return configSavedMsg{err: store.SaveConfig(path, cfg)}
	}
}
