package ui

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/tsoueid/360WeatherDashTUI/internal/store"
	"github.com/tsoueid/360WeatherDashTUI/internal/weather"
)

func TestAddCityAppendsAndSyncsConfig(t *testing.T) {
	m := testModel(t, 2)
	m.configPath = filepath.Join(t.TempDir(), "config.toml")

	m, err := m.addCity(weather.City{
		ID: "om-1", Name: "Lisbon", Lat: 38.72, Lon: -9.14, TZ: "Europe/Lisbon",
	})
	if err != nil {
		t.Fatalf("addCity: %v", err)
	}
	if len(m.cities) != 3 {
		t.Fatalf("got %d cities, want 3", len(m.cities))
	}
	if len(m.cfg.Cities) != 3 {
		t.Errorf("config holds %d cities, want 3", len(m.cfg.Cities))
	}
}

// Two entries for the same place would produce two identical rows and
// two wasted coordinates in every request.
func TestAddCityRejectsNearDuplicate(t *testing.T) {
	m := testModel(t, 1)
	m.cities[0].Lat, m.cities[0].Lon = 48.8566, 2.3522

	_, err := m.addCity(weather.City{
		ID: "different-id", Name: "Paris", Lat: 48.8570, Lon: 2.3525,
	})
	if err == nil {
		t.Fatal("adding a place within the duplicate radius must error")
	}
}

func TestAddCityAllowsDistinctPlacesWithTheSameName(t *testing.T) {
	m := testModel(t, 1)
	m.cities[0].Name = "Paris"
	m.cities[0].Lat, m.cities[0].Lon = 48.8566, 2.3522
	m.configPath = filepath.Join(t.TempDir(), "config.toml")

	if _, err := m.addCity(weather.City{
		ID: "om-2", Name: "Paris", Lat: 33.66, Lon: -95.55,
	}); err != nil {
		t.Fatalf("Paris, Texas must be addable alongside Paris, France: %v", err)
	}
}

func TestDeleteSelectedCityRemovesAndClamps(t *testing.T) {
	m := testModel(t, 3)
	m.configPath = filepath.Join(t.TempDir(), "config.toml")
	m.cursorCity = 2
	removedID := m.cities[2].ID

	m = m.deleteSelectedCity()

	if len(m.cities) != 2 {
		t.Fatalf("got %d cities, want 2", len(m.cities))
	}
	if m.cursorCity != 1 {
		t.Errorf("cursorCity = %d, want 1 after deleting the last row", m.cursorCity)
	}
	if _, ok := m.data[removedID]; ok {
		t.Error("the deleted city's cached forecast must be dropped")
	}
	if len(m.cfg.Cities) != 2 {
		t.Errorf("config holds %d cities, want 2", len(m.cfg.Cities))
	}
}

func TestDeleteOnEmptyListIsSafe(t *testing.T) {
	m := New(defaultTestConfig(), emptyTestCache())
	m.w, m.h = 100, 30
	m = m.deleteSelectedCity() // must not panic

	if len(m.cities) != 0 {
		t.Errorf("got %d cities, want 0", len(m.cities))
	}
}

func TestSaveConfigCmdWritesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	cfg := store.DefaultConfig()
	cfg.Cities = []weather.City{{ID: "a", Name: "Paris", TZ: "Europe/Paris"}}

	if msg := saveConfigCmd(path, cfg)(); msg != nil {
		if e, ok := msg.(configSavedMsg); ok && e.err != nil {
			t.Fatalf("save reported %v", e.err)
		}
	}

	got, _, err := store.LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Cities) != 1 || got.Cities[0].Name != "Paris" {
		t.Errorf("round trip gave %+v", got.Cities)
	}
}

func TestDeleteRequiresConfirmation(t *testing.T) {
	m := send(testModel(t, 3), "d")
	if !m.confirmDelete {
		t.Fatal("d must arm a confirmation rather than deleting immediately")
	}
	if len(m.cities) != 3 {
		t.Errorf("got %d cities; nothing may be deleted before confirming", len(m.cities))
	}

	m.configPath = ""
	m = send(m, "d")
	if len(m.cities) != 2 {
		t.Errorf("got %d cities, want 2 after confirming", len(m.cities))
	}
	if m.confirmDelete {
		t.Error("the confirmation must be disarmed after acting")
	}
}

func TestDeleteConfirmationCancels(t *testing.T) {
	m := send(testModel(t, 3), "d", "esc")
	if m.confirmDelete {
		t.Error("esc must disarm the confirmation")
	}
	if len(m.cities) != 3 {
		t.Errorf("got %d cities, want 3", len(m.cities))
	}
}

// The confirmation is a modal in the middle of the grid: the city is
// named, the keys are spelled out, and the frame's geometry survives
// the overlay.
func TestDeleteModalRendersCentered(t *testing.T) {
	m := send(testModel(t, 3), "d")
	out := m.View()

	if !strings.Contains(out, "Delete Paris?") {
		t.Errorf("the modal must name the city:\n%s", out)
	}
	if !strings.Contains(out, "enter or y delete · esc cancels") {
		t.Errorf("the modal must spell out confirm and cancel:\n%s", out)
	}
	for i, line := range strings.Split(out, "\n") {
		if got := lipgloss.Width(line); got != m.w {
			t.Errorf("line %d width = %d, want %d: %q", i, got, m.w, line)
		}
	}
}

func TestDeleteModalConfirmKeys(t *testing.T) {
	for _, confirm := range []string{"enter", "y", "d"} {
		m := send(testModel(t, 3), "d")
		m.configPath = ""
		m = send(m, confirm)
		if len(m.cities) != 2 {
			t.Errorf("%s must confirm the delete, got %d cities", confirm, len(m.cities))
		}
		if m.confirmDelete {
			t.Errorf("%s must close the modal", confirm)
		}
	}

	// Any other key cancels and deletes nothing.
	m := send(testModel(t, 3), "d", "x")
	if m.confirmDelete || len(m.cities) != 3 {
		t.Errorf("a stray key must cancel the modal, got %d cities, open=%v",
			len(m.cities), m.confirmDelete)
	}
}
