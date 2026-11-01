package ui

import (
	"path/filepath"
	"testing"

	"github.com/tsoueid/360WeatherDashTUI/internal/units"
)

func TestUnitsToggle(t *testing.T) {
	m := testModel(t, 3)
	m.configPath = filepath.Join(t.TempDir(), "config.toml")

	m = send(m, "u")
	if m.units != units.Imperial {
		t.Errorf("units = %v, want Imperial", m.units)
	}
	if m.cfg.Units != "imperial" {
		t.Errorf("cfg.Units = %q, want imperial", m.cfg.Units)
	}

	m = send(m, "u")
	if m.units != units.Metric {
		t.Errorf("units = %v, want Metric", m.units)
	}
}

// Toggling units must not discard cached data: values are stored
// canonical-metric and converted at render time.
func TestUnitsToggleKeepsData(t *testing.T) {
	m := testModel(t, 3)
	before := len(m.data)
	m = send(m, "u")

	if len(m.data) != before {
		t.Errorf("data has %d entries after toggling, want %d", len(m.data), before)
	}
}

func TestDensityToggle(t *testing.T) {
	m := testModel(t, 3)
	m.configPath = filepath.Join(t.TempDir(), "config.toml")

	m = send(m, "z")
	if m.density != densityExpanded {
		t.Errorf("density = %v, want expanded", m.density)
	}
	if m.cfg.Density != "expanded" {
		t.Errorf("cfg.Density = %q, want expanded", m.cfg.Density)
	}
}

func TestReorderInManualMode(t *testing.T) {
	m := testModel(t, 4)
	m.configPath = filepath.Join(t.TempDir(), "config.toml")
	m.cursorCity = 2
	moved := m.cities[2].ID

	m = send(m, "K")

	if m.cities[1].ID != moved {
		t.Errorf("city is at index %d, want 1", indexOf(m, moved))
	}
	if m.cursorCity != 1 {
		t.Errorf("cursorCity = %d, want 1 — the cursor must follow the city", m.cursorCity)
	}
	if m.cfg.Cities[1].ID != moved {
		t.Error("a manual reorder MUST write through to the saved order")
	}
}

func TestReorderClampsAtEnds(t *testing.T) {
	m := testModel(t, 3)
	m.configPath = filepath.Join(t.TempDir(), "config.toml")
	m.cursorCity = 0
	first := m.cities[0].ID

	m = send(m, "K")
	if m.cities[0].ID != first || m.cursorCity != 0 {
		t.Error("moving the top city up must be a no-op")
	}
}

func TestReorderRefusedWhileSorted(t *testing.T) {
	m := testModel(t, 4)
	m.sortMode = sortWarmest
	m = m.rebuildView()
	before := m.cfg.Cities[0].ID

	m = send(m, "K")

	if m.cfg.Cities[0].ID != before {
		t.Error("reordering while sorted must not change the saved order")
	}
}

func indexOf(m Model, id string) int {
	for i, c := range m.cities {
		if c.ID == id {
			return i
		}
	}
	return -1
}
