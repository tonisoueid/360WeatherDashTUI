package ui

import "testing"

func TestEnterZoomsInAndEscReturns(t *testing.T) {
	m := send(testModel(t, 4), "down", "right", "right", "enter")

	if m.screen != screenDetail {
		t.Fatalf("screen = %v, want screenDetail", m.screen)
	}
	if m.cursorCity != 1 || m.cursorDay != 2 {
		t.Errorf("zooming changed the cursor to %d,%d; want 1,2",
			m.cursorCity, m.cursorDay)
	}

	m = send(m, "esc")
	if m.screen != screenGrid {
		t.Errorf("screen = %v, want screenGrid", m.screen)
	}
	if m.cursorCity != 1 || m.cursorDay != 2 {
		t.Errorf("escaping changed the cursor to %d,%d; want 1,2",
			m.cursorCity, m.cursorDay)
	}
}

// The detail view is a magnifying glass over the matrix: moving inside
// it moves the same cursor, without returning to the grid.
func TestDetailNavigationMovesTheSharedCursor(t *testing.T) {
	m := send(testModel(t, 4), "enter", "right", "right", "down")

	if m.screen != screenDetail {
		t.Fatalf("screen = %v, want to stay in screenDetail", m.screen)
	}
	if m.cursorDay != 2 {
		t.Errorf("cursorDay = %d, want 2", m.cursorDay)
	}
	if m.cursorCity != 1 {
		t.Errorf("cursorCity = %d, want 1", m.cursorCity)
	}
}

func TestDetailNavigationClamps(t *testing.T) {
	m := send(testModel(t, 2), "enter", "left", "left", "up", "up")

	if m.cursorCity != 0 || m.cursorDay != 0 {
		t.Errorf("cursor = %d,%d; want 0,0", m.cursorCity, m.cursorDay)
	}
	if m.screen != screenDetail {
		t.Errorf("clamping must not leave the detail screen")
	}
}

func TestEnterWithNoCitiesStaysOnGrid(t *testing.T) {
	m := New(defaultTestConfig(), emptyTestCache())
	m.w, m.h = 100, 30
	m = send(m, "enter")

	if m.screen != screenGrid {
		t.Errorf("screen = %v, want screenGrid with no cities to zoom into", m.screen)
	}
}

func TestViewRendersDetailWhenZoomed(t *testing.T) {
	m := send(testModel(t, 3), "enter")
	if got := m.View(); got == "" {
		t.Error("View must render the detail screen")
	}
}
