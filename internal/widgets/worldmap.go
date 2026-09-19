package widgets

import "math"

// MaskCols, MaskRows, MaskNorth, and MaskSouth describe landMask's own
// coordinate system (see worldmask.go): 360 columns of 1° spanning the
// full -180°..180° longitude range, and 145 rows of 1° cropped to
// 85°N..60°S. Antarctica and the high Arctic have no cities to plot,
// and cropping them buys resolution everywhere a city can actually be.
const (
	MaskCols  = 360
	MaskRows  = 145
	MaskNorth = 85.0
	MaskSouth = -60.0
)

// MapSize picks the largest pixel grid that fits inside availW
// terminal columns and availLines terminal lines while preserving the
// mask's MaskCols:MaskRows aspect ratio, so continents are never
// stretched by a terminal whose shape doesn't match the world's.
//
// Pixel rows come two to a terminal line — a half-block glyph pairs
// an upper and a lower pixel into one line — so the returned px is
// always even and availLines*2 is the true pixel budget.
func MapSize(availW, availLines int) (w, px int) {
	if availW <= 0 || availLines <= 0 {
		return 0, 0
	}

	maxPx := availLines * 2

	// Start width-bound: use every available column.
	w = availW
	px = w * MaskRows / MaskCols
	if px > maxPx {
		// Too tall for the width chosen: go height-bound instead,
		// then re-derive the width from that.
		px = maxPx
		w = px * MaskCols / MaskRows
		if w > availW {
			w = availW
		}
	}

	// px must be even to pair into whole terminal lines. Rounding
	// down (never up) is what keeps the box inside its budget.
	if px%2 != 0 {
		px--
	}
	if px < 2 {
		px = 2
	}
	if w < 1 {
		w = 1
	}
	return w, px
}

// LandRaster downsamples the embedded land mask to a w x px pixel
// grid, indexed [row][col] with row 0 at the north edge.
//
// A pixel is land if ANY source cell inside its footprint is land —
// not a majority vote. At the coarse resolutions a terminal renders,
// a majority rule erases Iceland, Ireland, Japan, and every narrow
// peninsula; OR is what keeps them on the map.
func LandRaster(w, px int) [][]bool {
	if w <= 0 || px <= 0 {
		return nil
	}

	out := make([][]bool, px)
	for y := 0; y < px; y++ {
		sy0 := y * MaskRows / px
		sy1 := (y + 1) * MaskRows / px
		if sy1 <= sy0 {
			sy1 = sy0 + 1
		}

		row := make([]bool, w)
		for x := 0; x < w; x++ {
			sx0 := x * MaskCols / w
			sx1 := (x + 1) * MaskCols / w
			if sx1 <= sx0 {
				sx1 = sx0 + 1
			}

			land := false
			for sy := sy0; sy < sy1 && sy < MaskRows && !land; sy++ {
				for sx := sx0; sx < sx1 && sx < MaskCols; sx++ {
					if landMask[sy][sx] == '#' {
						land = true
						break
					}
				}
			}
			row[x] = land
		}
		out[y] = row
	}
	return out
}

// Project maps a latitude/longitude to a (col, row) pixel in a w x px
// grid built by MapSize/LandRaster.
//
// Longitude always wraps rather than being rejected — a provider
// returning 181° or 540° lands on a sane column instead of an
// out-of-range one. Latitude does not wrap: ok is false outside the
// mask's 85°N..60°S crop, and callers skip drawing that city rather
// than plotting it on the wrong pole.
func Project(lat, lon float64, w, px int) (col, row int, ok bool) {
	if w <= 0 || px <= 0 {
		return 0, 0, false
	}

	lon = math.Mod(lon+180, 360)
	if lon < 0 {
		lon += 360
	}
	lon -= 180

	col = int((lon + 180) / 360 * float64(w))
	if col >= w {
		col = w - 1
	}
	if col < 0 {
		col = 0
	}

	if lat > MaskNorth || lat < MaskSouth {
		return col, 0, false
	}

	row = int((MaskNorth - lat) / (MaskNorth - MaskSouth) * float64(px))
	if row >= px {
		row = px - 1
	}
	if row < 0 {
		row = 0
	}
	return col, row, true
}
