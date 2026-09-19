package widgets

import "testing"

// TestLandMaskWellFormed guards the generated data table itself: a
// mis-run of tools/mapgen (wrong dimensions, stray characters) must
// fail here rather than surface as a silently wrong map.
func TestLandMaskWellFormed(t *testing.T) {
	if len(landMask) != MaskRows {
		t.Fatalf("landMask has %d rows, want %d", len(landMask), MaskRows)
	}
	land, total := 0, 0
	for i, row := range landMask {
		if len(row) != MaskCols {
			t.Fatalf("row %d has %d columns, want %d", i, len(row), MaskCols)
		}
		for _, c := range row {
			if c != '#' && c != '.' {
				t.Fatalf("row %d contains %q, want only '#' or '.'", i, c)
			}
			if c == '#' {
				land++
			}
			total++
		}
	}
	if frac := float64(land) / float64(total); frac < 0.20 || frac > 0.40 {
		t.Errorf("land fraction = %.3f, want roughly 0.20-0.40 (Earth is ~29%% land)", frac)
	}
}

// landAt samples the raw 1° mask at the given coordinates, independent
// of any downsampling, for exact geographic assertions.
func landAt(t *testing.T, lat, lon float64) bool {
	t.Helper()
	col := int((lon + 180) / 360 * MaskCols)
	if col < 0 || col >= MaskCols {
		t.Fatalf("longitude %v out of range", lon)
	}
	row := int((MaskNorth - lat) / (MaskNorth - MaskSouth) * MaskRows)
	if row < 0 || row >= MaskRows {
		t.Fatalf("latitude %v out of mask crop", lat)
	}
	return landMask[row][col] == '#'
}

// TestLandMaskGeography spot-checks well-known interior points. These
// catch the two failure modes a rasterizer bug produces: an
// inverted mask (land and sea swapped) and a flipped axis (north/south
// or east/west transposed) — both would pass the well-formedness test
// above but fail every check here.
func TestLandMaskGeography(t *testing.T) {
	cases := []struct {
		name     string
		lat, lon float64
		land     bool
	}{
		{"Sahara", 23, 10, true},
		{"Siberia", 60, 100, true},
		{"Amazon basin", -5, -60, true},
		{"central Australia", -25, 134, true},
		{"central USA", 39, -98, true},
		{"Greenland interior", 72, -40, true},

		{"mid-Pacific", 0, -140, false},
		{"mid-Atlantic", 30, -40, false},
		{"south Indian Ocean", -30, 80, false},
		{"Southern Ocean", -55, 100, false},

		// Orientation guards: catch a mask that is well-formed and even
		// roughly plausible in land fraction, but transposed on an axis.
		{"Mongolia (north-of-equator control)", 50, 100, true},
		{"ocean south of Mongolia's longitude", -50, 100, false},
		{"Sahara (east-of-prime-meridian control)", 20, 20, true},
		{"Atlantic west of that latitude", 20, -20, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := landAt(t, c.lat, c.lon); got != c.land {
				t.Errorf("landAt(%v, %v) = %v, want %v", c.lat, c.lon, got, c.land)
			}
		})
	}
}

func TestMapSize(t *testing.T) {
	cases := []struct {
		availW, availLines int
	}{
		{50, 13}, {60, 18}, {80, 21}, {120, 37}, {200, 60},
	}
	for _, c := range cases {
		w, px := MapSize(c.availW, c.availLines)
		if w <= 0 || px <= 0 {
			t.Fatalf("MapSize(%d,%d) = %d,%d, want positive", c.availW, c.availLines, w, px)
		}
		if w > c.availW {
			t.Errorf("MapSize(%d,%d): w=%d exceeds available width", c.availW, c.availLines, w)
		}
		if px > c.availLines*2 {
			t.Errorf("MapSize(%d,%d): px=%d exceeds available pixel rows %d", c.availW, c.availLines, px, c.availLines*2)
		}
		if px%2 != 0 {
			t.Errorf("MapSize(%d,%d): px=%d is odd, want even", c.availW, c.availLines, px)
		}
		// Aspect ratio should track the mask's 360:145 within a
		// generous tolerance — terminal cells are only approximately
		// half-block-square, so this is not exact.
		got := float64(w) / float64(px)
		want := float64(MaskCols) / float64(MaskRows)
		if got < want*0.7 || got > want*1.3 {
			t.Errorf("MapSize(%d,%d): aspect %.2f, want near %.2f", c.availW, c.availLines, got, want)
		}
	}
}

func TestMapSizeDegenerate(t *testing.T) {
	for _, c := range [][2]int{{0, 10}, {10, 0}, {-5, 10}, {10, -5}} {
		if w, px := MapSize(c[0], c[1]); w != 0 || px != 0 {
			t.Errorf("MapSize(%d,%d) = %d,%d, want 0,0", c[0], c[1], w, px)
		}
	}
}

func TestLandRasterDimensions(t *testing.T) {
	for _, c := range []struct{ w, px int }{{40, 16}, {80, 32}, {200, 80}, {1, 2}} {
		out := LandRaster(c.w, c.px)
		if len(out) != c.px {
			t.Fatalf("LandRaster(%d,%d): %d rows, want %d", c.w, c.px, len(out), c.px)
		}
		for i, row := range out {
			if len(row) != c.w {
				t.Fatalf("LandRaster(%d,%d): row %d has %d cols, want %d", c.w, c.px, i, len(row), c.w)
			}
		}
	}
	if out := LandRaster(0, 10); out != nil {
		t.Errorf("LandRaster(0,10) = %v, want nil", out)
	}
}

// TestLandRasterKeepsIslands guards the OR-not-majority downsample
// rule: at a coarse pixel grid, a landmass no bigger than a handful of
// source cells (Iceland) must not be averaged away.
func TestLandRasterKeepsIslands(t *testing.T) {
	w, px := 60, 24 // roughly 6 degrees per pixel
	raster := LandRaster(w, px)
	col, row, ok := Project(64.9, -19.0, w, px) // interior Iceland
	if !ok {
		t.Fatal("Project placed Iceland outside the mask crop")
	}
	if !raster[row][col] {
		t.Errorf("Iceland's pixel (%d,%d) is sea at %dx%d — island lost to downsampling", col, row, w, px)
	}
}

func TestProjectKnownCities(t *testing.T) {
	const w, px = 360, 145 // 1:1 with the source mask, for exact arithmetic

	cases := []struct {
		name     string
		lat, lon float64
	}{
		{"Paris", 48.8566, 2.3522},
		{"Sydney", -33.8688, 151.2093},
		{"Reykjavik", 64.1466, -21.9426},
		{"null island", 0, 0},
	}
	for _, c := range cases {
		col, row, ok := Project(c.lat, c.lon, w, px)
		if !ok {
			t.Fatalf("%s: Project returned ok=false", c.name)
		}
		wantCol := int((c.lon + 180) / 360 * w)
		wantRow := int((MaskNorth - c.lat) / (MaskNorth - MaskSouth) * px)
		if col != wantCol || row != wantRow {
			t.Errorf("%s: Project = (%d,%d), want (%d,%d)", c.name, col, row, wantCol, wantRow)
		}
	}
}

func TestProjectLongitudeWraps(t *testing.T) {
	const w, px = 360, 145
	col181, _, ok := Project(0, 181, w, px)
	if !ok {
		t.Fatal("Project(0,181) returned ok=false")
	}
	colNeg179, _, _ := Project(0, -179, w, px)
	if col181 != colNeg179 {
		t.Errorf("Project(0,181) col=%d, Project(0,-179) col=%d, want equal (wraparound)", col181, colNeg179)
	}
	if col181 < 0 || col181 >= w {
		t.Errorf("wrapped column %d out of range [0,%d)", col181, w)
	}
}

func TestProjectOutsideCropIsNotOK(t *testing.T) {
	const w, px = 360, 145
	if _, _, ok := Project(90, 0, w, px); ok {
		t.Error("Project(90,0) should be outside the 85N..60S crop")
	}
	if _, _, ok := Project(-75, 0, w, px); ok {
		t.Error("Project(-75,0) should be outside the 85N..60S crop")
	}
	if _, _, ok := Project(MaskNorth, 0, w, px); !ok {
		t.Error("Project at the exact north edge should be in range")
	}
	if _, _, ok := Project(MaskSouth, 0, w, px); !ok {
		t.Error("Project at the exact south edge should be in range")
	}
}
