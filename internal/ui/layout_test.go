package ui

import (
	"strings"
	"testing"

	"github.com/tsoueid/weatherdash/internal/widgets"
)

func TestLayoutTiersByWidth(t *testing.T) {
	// TierRich got wider (see widgets.Tier.MinWidth) to fit the rain
	// percentage and its own breathing room, so it now takes a wider
	// terminal to reach at the default 7-day window than it used to —
	// 120 still gets there, 100 no longer does.
	tests := []struct {
		w        int
		wantTier widgets.Tier
	}{
		{120, widgets.TierRich},
		{100, widgets.TierMedium},
		{80, widgets.TierMedium},
		{60, widgets.TierMinimal},
	}
	for _, tc := range tests {
		m := testModel(t, 3)
		m.w = tc.w
		if got := m.layout().tier; got != tc.wantTier {
			t.Errorf("width %d: tier = %v, want %v (cellW %d)",
				tc.w, got, tc.wantTier, m.layout().cellW)
		}
	}
}

// With a long forecast window there is not enough width for every day,
// so columns are dropped rather than truncated into nonsense.
func TestLayoutDropsDaysWhenTooNarrow(t *testing.T) {
	m := testModel(t, 3)
	m.days = 16
	m.w = 80

	l := m.layout()
	if l.days >= 16 {
		t.Errorf("days = %d, want fewer than 16 at width 80", l.days)
	}
	if l.cellW < widgets.TierMinimal.MinWidth() {
		t.Errorf("cellW = %d, want at least %d after dropping days",
			l.cellW, widgets.TierMinimal.MinWidth())
	}
	if l.days < 3 {
		t.Errorf("days = %d, want at least the 3-day floor", l.days)
	}
}

func TestLayoutRowsRespectDensity(t *testing.T) {
	m := testModel(t, 20)
	m.h = 24

	compact := m.layout().rows
	m.density = densityExpanded
	expanded := m.layout().rows

	if expanded >= compact {
		t.Errorf("expanded rows = %d, compact = %d; expanded must fit fewer",
			expanded, compact)
	}
	if expanded < 1 {
		t.Errorf("expanded rows = %d, want at least 1", expanded)
	}
}

// The city-name column widens to fit the longest name in view, so a
// name is never cut when the width is there to give it.
func TestLabelColumnFitsLongestName(t *testing.T) {
	m := testModel(t, 2)
	m.cities[0].Name = "Salt Lake City" // 14 columns

	l := m.layout()
	if l.labelW != 14 {
		t.Errorf("labelW = %d, want 14 to fit the longest name", l.labelW)
	}
	if out := m.renderGrid(); !strings.Contains(out, "Salt Lake City") {
		t.Errorf("the full name must render uncut:\n%s", out)
	}
}

// The floor holds for short names, and the ceiling keeps one long
// name from eating the day columns: even at the narrowest usable
// terminal with the widest label, cells keep their minimum tier.
func TestLabelColumnFloorAndCeiling(t *testing.T) {
	m := testModel(t, 2)

	if got := m.labelColumnWidth(); got != minLabelWidth {
		t.Errorf("short names: labelW = %d, want the %d floor", got, minLabelWidth)
	}

	m.cities[0].Name = "San Cristóbal de La Laguna" // 26 columns
	m.w = minTermWidth

	l := m.layout()
	if l.labelW != maxLabelWidth {
		t.Errorf("long name: labelW = %d, want the %d ceiling", l.labelW, maxLabelWidth)
	}
	if out := m.renderGrid(); !strings.Contains(out, "San Cristóbal de…") {
		t.Errorf("a name past the ceiling must be cut visibly, with an ellipsis:\n%s", out)
	}
	if l.cellW < widgets.TierMinimal.MinWidth() {
		t.Errorf("cellW = %d, want at least %d beside the widest label",
			l.cellW, widgets.TierMinimal.MinWidth())
	}
	if l.days < minDayColumns {
		t.Errorf("days = %d, want at least the %d-day floor", l.days, minDayColumns)
	}
}

func TestTooSmall(t *testing.T) {
	m := testModel(t, 3)

	m.w, m.h = 100, 30
	if m.tooSmall() {
		t.Error("100x30 must not be too small")
	}
	m.w, m.h = 40, 30
	if !m.tooSmall() {
		t.Error("width 40 must be too small")
	}
	m.w, m.h = 100, 8
	if !m.tooSmall() {
		t.Error("height 8 must be too small")
	}
}

func TestEnsureCursorVisibleScrollsDown(t *testing.T) {
	m := testModel(t, 20)
	m.w, m.h = 100, 14 // a small viewport so scrolling is forced

	rows := m.layout().rows
	m.cursorCity = 19
	m.ensureCursorVisible()

	if m.cursorCity < m.scroll || m.cursorCity >= m.scroll+rows {
		t.Errorf("cursor %d outside window [%d,%d)",
			m.cursorCity, m.scroll, m.scroll+rows)
	}
}

func TestEnsureCursorVisibleScrollsUp(t *testing.T) {
	m := testModel(t, 20)
	m.w, m.h = 100, 14
	m.scroll = 10
	m.cursorCity = 0
	m.ensureCursorVisible()

	if m.scroll != 0 {
		t.Errorf("scroll = %d, want 0 when the cursor is at the top", m.scroll)
	}
}

func TestScrollNeverExceedsContent(t *testing.T) {
	m := testModel(t, 4)
	m.w, m.h = 100, 30
	m.scroll = 99
	m.cursorCity = 0
	m.ensureCursorVisible()

	if m.scroll != 0 {
		t.Errorf("scroll = %d, want 0 when all rows fit", m.scroll)
	}
}

func TestEnsureCursorVisibleEmptyList(t *testing.T) {
	m := New(defaultTestConfig(), emptyTestCache())
	m.w, m.h = 100, 30
	m.ensureCursorVisible() // must not panic or divide by zero

	if m.scroll != 0 {
		t.Errorf("scroll = %d, want 0", m.scroll)
	}
}
