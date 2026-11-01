package ui

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/tsoueid/weatherdash/internal/widgets"
)

var update = flag.Bool("update", false, "rewrite golden files")

// assertGolden compares a rendered frame against a stored snapshot.
func assertGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", "golden", name+".txt")

	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("missing golden file %s — run: go test ./internal/ui/ -update", path)
	}
	if got != string(want) {
		t.Errorf("frame does not match %s\n--- got ---\n%s\n--- want ---\n%s",
			path, got, want)
	}
}

// goldenClock is pinned so the day labels baked into snapshots never
// drift with the real date.
var goldenClock = time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)

// goldenModel is deterministic: DemoModel's data is generated from a
// fixed formula, the clock is pinned, and every frame is rendered at a
// fixed size, so the only thing that can change a snapshot is a
// rendering change.
func goldenModel(t *testing.T, w, h int) Model {
	t.Helper()
	// Under go test stdout is not a TTY, so lipgloss would strip all
	// styling — including the reverse-video cursor — from the frame.
	// Pinning the profile keeps the snapshot faithful to what a user
	// sees, which is the point of a golden.
	lipgloss.SetColorProfile(termenv.ANSI256)
	m := DemoModel()
	m.w, m.h = w, h
	m.nowFunc = func() time.Time { return goldenClock }
	m.lastOK = goldenClock // "updated just now" stays deterministic
	return m
}

func TestGoldenGrid80x24(t *testing.T) {
	assertGolden(t, "grid_80x24", goldenModel(t, 80, 24).View())
}

func TestGoldenGrid120x40(t *testing.T) {
	assertGolden(t, "grid_120x40", goldenModel(t, 120, 40).View())
}

// TestGoldenGridRich130x30 is the only golden that exercises
// TierRich's compact-density rendering (rain percentage, full
// spacing): TierRich.MinWidth grew to fit its now fixed-width figures,
// so 120 columns — the widest of the other grid goldens — lands on
// TierMedium instead. Without this, no golden frame would render the
// rain-percentage badge at all.
func TestGoldenGridRich130x30(t *testing.T) {
	m := goldenModel(t, 130, 30)
	if got := m.layout().tier; got != widgets.TierRich {
		t.Fatalf("this golden is only useful if it reaches TierRich; got %v (add width)", got)
	}
	assertGolden(t, "grid_rich_130x30", m.View())
}

func TestGoldenGridExpanded(t *testing.T) {
	m := goldenModel(t, 120, 40)
	m.density = densityExpanded
	assertGolden(t, "grid_expanded_120x40", m.View())
}

func TestGoldenGridNarrow(t *testing.T) {
	assertGolden(t, "grid_60x20", goldenModel(t, 60, 20).View())
}

func TestGoldenEmptyState(t *testing.T) {
	m := New(defaultTestConfig(), emptyTestCache())
	m.w, m.h = 80, 24
	m.nowFunc = func() time.Time { return goldenClock }
	assertGolden(t, "empty_80x24", m.View())
}

func TestGoldenTooSmall(t *testing.T) {
	assertGolden(t, "too_small_30x10", goldenModel(t, 30, 10).View())
}

func TestGoldenHelp(t *testing.T) {
	m := goldenModel(t, 80, 24)
	m.helpReturn = screenGrid
	m.screen = screenHelp
	assertGolden(t, "help_80x24", m.View())
}

func TestGoldenAddCityModal(t *testing.T) {
	m := goldenModel(t, 80, 24)
	m.screen = screenAddCity
	m.search.SetValue("Paris")
	m.searchResults = parisResults()
	assertGolden(t, "addcity_80x24", m.View())
}

func TestGoldenWorldMap80x24(t *testing.T) {
	m := goldenModel(t, 80, 24)
	m.screen = screenMap
	assertGolden(t, "map_80x24", m.View())
}

func TestGoldenWorldMap120x40(t *testing.T) {
	m := goldenModel(t, 120, 40)
	m.screen = screenMap
	assertGolden(t, "map_120x40", m.View())
}

// TestGoldenWorldMapDay3 pins the day cursor forward so the snapshot
// proves scrubbing actually repaints every marker, not just the day
// the map happens to open on.
func TestGoldenWorldMapDay3(t *testing.T) {
	m := goldenModel(t, 120, 40)
	m.screen = screenMap
	m.cursorDay = 3
	assertGolden(t, "map_day3_120x40", m.View())
}
