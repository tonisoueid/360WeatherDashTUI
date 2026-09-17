package ui

import "testing"

func TestSortModeCycles(t *testing.T) {
	seen := map[sortMode]bool{}
	s := sortManual
	for i := 0; i < 4; i++ {
		seen[s] = true
		s = s.Next()
	}
	if len(seen) != 4 {
		t.Errorf("cycling visited %d modes, want 4", len(seen))
	}
	if s != sortManual {
		t.Errorf("cycling four times ended on %v, want back at sortManual", s)
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
	for _, s := range []sortMode{sortManual, sortWarmest, sortColdest, sortDriest} {
		if got := parseSortMode(s.String()); got != s {
			t.Errorf("parseSortMode(%q) = %v, want %v", s.String(), got, s)
		}
	}
	if got := parseSortMode("nonsense"); got != sortManual {
		t.Errorf("unknown sort = %v, want sortManual", got)
	}
}
