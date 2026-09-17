package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func keyMsg(s string) tea.KeyMsg {
	switch s {
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

// send applies a sequence of keys and returns the resulting model.
func send(m Model, keys ...string) Model {
	for _, k := range keys {
		next, _ := m.Update(keyMsg(k))
		m = next.(Model)
	}
	return m
}

func TestUpdateMovesCursor(t *testing.T) {
	m := send(testModel(t, 5), "down", "down", "right")

	if m.cursorCity != 2 {
		t.Errorf("cursorCity = %d, want 2", m.cursorCity)
	}
	if m.cursorDay != 1 {
		t.Errorf("cursorDay = %d, want 1", m.cursorDay)
	}
}

func TestUpdateVimKeysMatchArrows(t *testing.T) {
	arrows := send(testModel(t, 5), "down", "down", "right")
	vim := send(testModel(t, 5), "j", "j", "l")

	if arrows.cursorCity != vim.cursorCity || arrows.cursorDay != vim.cursorDay {
		t.Errorf("vim gave %d,%d; arrows gave %d,%d",
			vim.cursorCity, vim.cursorDay, arrows.cursorCity, arrows.cursorDay)
	}
}

func TestUpdateClampsAtEdges(t *testing.T) {
	m := send(testModel(t, 3), "up", "up", "left", "left")
	if m.cursorCity != 0 || m.cursorDay != 0 {
		t.Errorf("cursor = %d,%d; want 0,0", m.cursorCity, m.cursorDay)
	}
}

func TestUpdateWindowSizeUpdatesLayout(t *testing.T) {
	m := testModel(t, 3)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 50})
	m = next.(Model)

	if m.w != 140 || m.h != 50 {
		t.Errorf("size = %dx%d, want 140x50", m.w, m.h)
	}
}

// Shrinking the window must not strand the cursor outside the viewport.
func TestUpdateResizeKeepsCursorVisible(t *testing.T) {
	m := testModel(t, 20)
	m.w, m.h = 100, 40
	m.cursorCity = 19
	m.ensureCursorVisible()

	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 16})
	m = next.(Model)

	rows := m.layout().rows
	if m.cursorCity < m.scroll || m.cursorCity >= m.scroll+rows {
		t.Errorf("after resize cursor %d is outside [%d,%d)",
			m.cursorCity, m.scroll, m.scroll+rows)
	}
}

func TestUpdateQuitReturnsQuitCmd(t *testing.T) {
	m := testModel(t, 3)
	_, cmd := m.Update(keyMsg("q"))
	if cmd == nil {
		t.Fatal("q must return a command")
	}
	if msg := cmd(); msg == nil {
		t.Fatal("q must produce a quit message")
	}
}

func TestViewTooSmall(t *testing.T) {
	m := testModel(t, 3)
	m.w, m.h = 30, 10

	out := m.View()
	if !strings.Contains(strings.ToLower(out), "too small") {
		t.Errorf("small terminal view = %q, want a 'too small' message", out)
	}
}

func TestViewRendersGridAtNormalSize(t *testing.T) {
	m := testModel(t, 4)
	if !strings.Contains(m.View(), "Paris") {
		t.Error("View must render the grid at a normal size")
	}
}

// View must never panic before the first WindowSizeMsg arrives, when
// width and height are still zero.
func TestViewBeforeFirstWindowSize(t *testing.T) {
	m := New(defaultTestConfig(), emptyTestCache())
	if out := m.View(); out == "" {
		t.Error("View at zero size must render something, not empty")
	}
}

func TestDemoModelHasData(t *testing.T) {
	m := DemoModel()
	if len(m.cities) < 5 {
		t.Errorf("demo has %d cities, want at least 5", len(m.cities))
	}
	for _, c := range m.cities {
		if _, ok := m.data[c.ID]; !ok {
			t.Errorf("demo city %q has no forecast", c.Name)
		}
	}
}
