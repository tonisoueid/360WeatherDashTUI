package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/tsoueid/weatherdash/internal/widgets"
)

// mapTestModel is testModel plus real coordinates on every city, since
// testModel's cities default to (0,0) — deliberately unset, so the map
// tests that need a plottable city must set coordinates explicitly,
// the same way cities_test.go does for duplicate detection.
func mapTestModel(t *testing.T, coords ...[2]float64) Model {
	t.Helper()
	m := testModel(t, len(coords))
	for i, c := range coords {
		m.cities[i].Lat, m.cities[i].Lon = c[0], c[1]
		m.cfg.Cities[i].Lat, m.cfg.Cities[i].Lon = c[0], c[1]
	}
	return m
}

var (
	parisCoord  = [2]float64{48.8566, 2.3522}
	beirutCoord = [2]float64{33.8938, 35.5018}
	sydneyCoord = [2]float64{-33.8688, 151.2093}
	nullIsland  = [2]float64{0, 0}
	northOfCrop = [2]float64{89, 0}
)

// --- glyph safety -----------------------------------------------------

// TestMapMarkerGlyphsAreOneColumn guards Splice's column arithmetic:
// every marker glyph must be exactly one terminal column, or a marker
// insertion shifts everything to its right on that raster line.
func TestMapMarkerGlyphsAreOneColumn(t *testing.T) {
	for _, g := range []string{"◉", "●", "○"} {
		if w := lipgloss.Width(g); w != 1 {
			t.Errorf("glyph %q is %d columns wide, want 1", g, w)
		}
	}
}

func TestMapMarkerGlyphSelection(t *testing.T) {
	th := newTheme()
	cases := []struct {
		name     string
		mk       mapMarker
		wantChar string
	}{
		{"normal with data", mapMarker{haveTemp: true, tempC: 18}, "●"},
		{"normal no data", mapMarker{haveTemp: false}, "○"},
		{"cursored with data", mapMarker{cursored: true, haveTemp: true, tempC: 18}, "◉"},
		{"cursored no data", mapMarker{cursored: true, haveTemp: false}, "◉"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			glyph, _ := mapMarkerGlyph(c.mk, th)
			if glyph != c.wantChar {
				t.Errorf("glyph = %q, want %q", glyph, c.wantChar)
			}
		})
	}
}

// --- mapMarkers ---------------------------------------------------------

func TestMapMarkersSkipsZeroCoordinates(t *testing.T) {
	m := mapTestModel(t, nullIsland, parisCoord)
	markers, skipped := m.mapMarkers(80, 32)
	if len(markers) != 1 {
		t.Fatalf("got %d markers, want 1 (the null-island city should be skipped)", len(markers))
	}
	if skipped != 1 {
		t.Errorf("skipped = %d, want 1", skipped)
	}
}

func TestMapMarkersSkipsOutsideCrop(t *testing.T) {
	m := mapTestModel(t, northOfCrop, parisCoord)
	markers, skipped := m.mapMarkers(80, 32)
	if len(markers) != 1 {
		t.Fatalf("got %d markers, want 1 (the out-of-crop city should be skipped)", len(markers))
	}
	if skipped != 1 {
		t.Errorf("skipped = %d, want 1", skipped)
	}
}

// TestMapMarkersUsesCurrentOnDay0AndHighAfter mirrors the detail
// screen's rule that a "now" reading only exists for today: day 0
// shows the live current temperature, every later day shows that
// day's forecast high.
func TestMapMarkersUsesCurrentOnDay0AndHighAfter(t *testing.T) {
	m := mapTestModel(t, parisCoord)
	f := m.data[m.cities[0].ID]
	f.Current.TempC = 99
	f.Days[2].HiC = 42
	m.data[m.cities[0].ID] = f

	m.cursorDay = 0
	markers, _ := m.mapMarkers(80, 32)
	if len(markers) != 1 || markers[0].tempC != 99 {
		t.Fatalf("day 0: markers = %+v, want tempC 99 (Current)", markers)
	}

	m.cursorDay = 2
	markers, _ = m.mapMarkers(80, 32)
	if len(markers) != 1 || markers[0].tempC != 42 {
		t.Fatalf("day 2: markers = %+v, want tempC 42 (Days[2].HiC)", markers)
	}
}

func TestMapMarkersNoForecastYet(t *testing.T) {
	m := mapTestModel(t, parisCoord)
	delete(m.data, m.cities[0].ID)

	markers, _ := m.mapMarkers(80, 32)
	if len(markers) != 1 || markers[0].haveTemp {
		t.Fatalf("markers = %+v, want one marker with haveTemp=false", markers)
	}
}

// --- stampMarkers ---------------------------------------------------------

// TestStampMarkersCursoredWinsCollision: when two cities project onto
// the exact same cell, the cursored one is drawn last and its glyph is
// what ends up visible — never silently hidden under a plain city's
// dot.
func TestStampMarkersCursoredWinsCollision(t *testing.T) {
	lines := []string{strings.Repeat(" ", 10)}
	markers := []mapMarker{
		{col: 3, line: 0, haveTemp: true, tempC: 5, cursored: false},
		{col: 3, line: 0, haveTemp: true, tempC: 20, cursored: true},
	}
	stampMarkers(lines, 0, markers)

	if got := widgets.CellAt(lines[0], 3); got != '◉' {
		t.Errorf("cell at collision = %q, want the cursored glyph ◉", got)
	}
}

// --- stampChips ---------------------------------------------------------

// plainCells reconstructs a rendered line's VISIBLE content, one rune
// per display column, via widgets.CellAt. A raw string.Contains check
// on styled output is unsafe here: an SGR sequence like
// "\x1b[1;38;5;42m" contains the literal digit '8' (from "38") with no
// '8' actually rendered, and this test's golden-shared color profile
// (internal/ui/golden_test.go pins it process-wide without restoring
// it) means whether that styling is present depends on test order.
// Reading back through CellAt is immune to both problems.
func plainCells(line string, w int) string {
	b := make([]rune, w)
	for i := range b {
		b[i] = widgets.CellAt(line, i)
	}
	return string(b)
}

// TestStampChipsSkipsWhenSandwiched sets up three markers two columns
// apart — cursored, plain, plain — so the middle marker has no free
// column on either side once chipGap is honoured: the cursored city's
// chip (placed first, to its left) and the third marker's own dot both
// claim its neighbours. Its temperature chip must be dropped rather
// than overwrite either neighbour; its marker dot must still be there.
func TestStampChipsSkipsWhenSandwiched(t *testing.T) {
	lines := []string{strings.Repeat(" ", 14)}
	markers := []mapMarker{
		{col: 5, line: 0, haveTemp: true, tempC: 9, cursored: true},  // "9"
		{col: 7, line: 0, haveTemp: true, tempC: 8, cursored: false}, // "8" — sandwiched
		{col: 9, line: 0, haveTemp: true, tempC: 7, cursored: false}, // "7"
	}
	m := mapTestModel(t) // no cities needed; only m.units is read

	m.stampChips(lines, 0, 14, markers)
	plain := plainCells(lines[0], 14)

	if !strings.Contains(plain, "9") {
		t.Errorf("plain = %q, want the cursored city's chip always placed", plain)
	}
	if !strings.Contains(plain, "7") {
		t.Errorf("plain = %q, want the third city's chip to fit and be placed", plain)
	}
	if strings.Contains(plain, "8") {
		t.Errorf("plain = %q, want the sandwiched city's chip dropped, not drawn over a neighbour", plain)
	}
}

func TestStampChipsSkipsCityWithNoData(t *testing.T) {
	lines := []string{strings.Repeat(" ", 10)}
	markers := []mapMarker{{col: 3, line: 0, haveTemp: false}}
	m := mapTestModel(t)

	m.stampChips(lines, 0, 10, markers)
	if lines[0] != strings.Repeat(" ", 10) {
		t.Errorf("line = %q, want unchanged: no chip for a city with no forecast", lines[0])
	}
}

// --- mapStatusLine ---------------------------------------------------------

func TestMapStatusLineNamesOnlyCursoredCity(t *testing.T) {
	m := mapTestModel(t, parisCoord, beirutCoord)
	m.cursorCity = 0 // Paris

	got := m.mapStatusLine(0)
	if !strings.Contains(got, "Paris") {
		t.Errorf("status line = %q, want the cursored city's name", got)
	}
	if strings.Contains(got, "Beirut") {
		t.Errorf("status line = %q, want ONLY the cursored city named", got)
	}
}

func TestMapStatusLineReportsSkippedCities(t *testing.T) {
	m := mapTestModel(t, parisCoord)

	if got := m.mapStatusLine(0); strings.Contains(got, "no coordinates") {
		t.Errorf("status line = %q, want no mention when nothing was skipped", got)
	}
	if got := m.mapStatusLine(1); !strings.Contains(got, "1 city has no coordinates") {
		t.Errorf("status line = %q, want the singular skipped-city note", got)
	}
	if got := m.mapStatusLine(2); !strings.Contains(got, "2 cities have no coordinates") {
		t.Errorf("status line = %q, want the plural skipped-city note", got)
	}
}

func TestMapStatusLineWidthIsExact(t *testing.T) {
	m := mapTestModel(t, parisCoord)
	for _, w := range []int{50, 80, 120} {
		m.w = w
		if got := lipgloss.Width(m.mapStatusLine(0)); got != w {
			t.Errorf("mapStatusLine width = %d, want %d", got, w)
		}
	}
}

// --- full renderMap geometry and behaviour ---------------------------------

func TestRenderMapFrameGeometry(t *testing.T) {
	sizes := []struct{ w, h int }{
		{50, 15}, {60, 20}, {80, 24}, {120, 40}, {200, 60},
	}
	for _, sz := range sizes {
		m := mapTestModel(t, parisCoord, beirutCoord, sydneyCoord)
		m.w, m.h = sz.w, sz.h
		m.screen = screenMap
		m.nowFunc = func() time.Time { return goldenClock }

		frame := m.View()
		lines := strings.Split(frame, "\n")
		if len(lines) != m.h {
			t.Errorf("%dx%d: got %d lines, want %d", sz.w, sz.h, len(lines), m.h)
			continue
		}
		for i, l := range lines {
			if got := lipgloss.Width(l); got != m.w {
				t.Errorf("%dx%d: line %d width = %d, want %d", sz.w, sz.h, i, got, m.w)
			}
		}
	}
}

func TestRenderMapEmptyCityListFallsBackToEmptyScreen(t *testing.T) {
	m := New(defaultTestConfig(), emptyTestCache())
	m.w, m.h = 80, 24
	m.screen = screenMap

	if got := m.View(); !strings.Contains(got, "Add your first city") {
		t.Errorf("empty map = %q, want the same empty state as the grid", got)
	}
}

func TestRenderMapMarkerAtProjectedCell(t *testing.T) {
	m := mapTestModel(t, parisCoord)
	m.w, m.h = 100, 30
	m.screen = screenMap
	m.nowFunc = func() time.Time { return goldenClock }

	w, px := widgets.MapSize(m.w, m.h-2)
	left := (m.w - w) / 2
	col, row, ok := widgets.Project(parisCoord[0], parisCoord[1], w, px)
	if !ok {
		t.Fatal("Project placed Paris outside the mask crop")
	}

	lines := strings.Split(m.View(), "\n")
	line := lines[1+row/2] // line 0 is the status line
	got := widgets.CellAt(line, left+col)
	if got != '◉' && got != '●' {
		t.Errorf("cell at Paris's projected position = %q, want a marker glyph", got)
	}
}

// --- key handling: toggle, cursor, and the detail round trip ---------------

func TestMapKeyTogglesWithoutPersisting(t *testing.T) {
	m := testModel(t, 3)
	next, cmd := m.Update(keyMsg("m"))
	m = next.(Model)

	if m.screen != screenMap {
		t.Fatalf("screen = %v, want screenMap", m.screen)
	}
	if cmd != nil {
		t.Error("toggling the map must not return a save command — the home view is not persisted")
	}
}

func TestMapKeyAndEscBothReturnToGrid(t *testing.T) {
	m := send(testModel(t, 3), "m", "m")
	if m.screen != screenGrid {
		t.Errorf("m twice: screen = %v, want screenGrid", m.screen)
	}

	m = send(testModel(t, 3), "m", "esc")
	if m.screen != screenGrid {
		t.Errorf("m then esc: screen = %v, want screenGrid", m.screen)
	}
}

func TestMapArrowsMoveTheSharedCursor(t *testing.T) {
	m := send(testModel(t, 5), "m", "down", "down", "right")
	if m.cursorCity != 2 {
		t.Errorf("cursorCity = %d, want 2", m.cursorCity)
	}
	if m.cursorDay != 1 {
		t.Errorf("cursorDay = %d, want 1", m.cursorDay)
	}
}

func TestMapCursorClampsAtEdges(t *testing.T) {
	m := send(testModel(t, 2), "m", "left", "left", "up", "up")
	if m.cursorCity != 0 || m.cursorDay != 0 {
		t.Errorf("cursor = %d,%d; want 0,0", m.cursorCity, m.cursorDay)
	}
}

// TestMapEnterReturnsToMapNotGrid is the regression this feature could
// easily introduce: zooming into detail from the map and backing out
// must surface back to the map, not unconditionally to the grid.
func TestMapEnterReturnsToMapNotGrid(t *testing.T) {
	m := send(testModel(t, 3), "m", "enter")
	if m.screen != screenDetail {
		t.Fatalf("screen = %v, want screenDetail", m.screen)
	}

	m = send(m, "esc")
	if m.screen != screenMap {
		t.Errorf("screen = %v, want screenMap (zoomed in from the map)", m.screen)
	}
}

// TestGridEnterStillReturnsToGrid guards the ordinary path against the
// detailReturn change above: zooming in from the grid must still
// surface back to the grid.
func TestGridEnterStillReturnsToGrid(t *testing.T) {
	m := send(testModel(t, 3), "enter")
	if m.screen != screenDetail {
		t.Fatalf("screen = %v, want screenDetail", m.screen)
	}
	m = send(m, "esc")
	if m.screen != screenGrid {
		t.Errorf("screen = %v, want screenGrid", m.screen)
	}
}

func TestMapEnterWithNoCitiesStaysOnMap(t *testing.T) {
	m := New(defaultTestConfig(), emptyTestCache())
	m.w, m.h = 100, 30
	m = send(m, "m", "enter")
	if m.screen != screenMap {
		t.Errorf("screen = %v, want screenMap with no cities to zoom into", m.screen)
	}
}

func TestMapHelpRoundTrip(t *testing.T) {
	m := send(testModel(t, 3), "m", "?")
	if m.screen != screenHelp {
		t.Fatalf("screen = %v, want screenHelp", m.screen)
	}
	if m.helpReturn != screenMap {
		t.Errorf("helpReturn = %v, want screenMap", m.helpReturn)
	}

	m = send(m, "?")
	if m.screen != screenMap {
		t.Errorf("screen = %v, want screenMap after closing help", m.screen)
	}
}

func TestMapUnitsToggleUpdatesConfig(t *testing.T) {
	m := send(testModel(t, 3), "m", "u")
	if m.cfg.Units != m.units.String() {
		t.Errorf("cfg.Units = %q, want it to track the toggled units %q", m.cfg.Units, m.units.String())
	}
}
