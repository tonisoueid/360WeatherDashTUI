// Command mapgen rasterizes a Natural Earth land GeoJSON into the 1°
// land mask consumed by internal/widgets/worldmap.go.
//
// It is a build-time tool in spirit only: it is never invoked by `go
// generate`, the Makefile, or CI, and it adds no dependency to the main
// module (stdlib only). Run it by hand — approximately never, since
// coastlines do not move — whenever the mask needs regenerating:
//
//	go run ./tools/mapgen -out internal/widgets/worldmask.go
//
// The default source is the Natural Earth 110m "land" layer (public
// domain, https://www.naturalearthdata.com), mirrored as GeoJSON at:
//
//	https://raw.githubusercontent.com/nvkelso/natural-earth-vector/master/geojson/ne_110m_land.geojson
//
// -src also accepts a local file path, so the mask can be regenerated
// offline from a cached copy of the same GeoJSON.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// Geometry matches the mask's own coordinate system: 1° cells, cropped
// to 85°N..60°S (north polar ice and Antarctica are ocean-or-ice with
// no cities to plot, and cropping them buys resolution everywhere
// else). Columns run -180°..180° west to east; rows run north to south
// so row 0 is the top of the rendered map, matching how a terminal
// paints downward.
const (
	gridWidth  = 360
	gridHeight = 145
	north      = 85.0
	south      = -60.0

	defaultSrc = "https://raw.githubusercontent.com/nvkelso/natural-earth-vector/master/geojson/ne_110m_land.geojson"
)

// point is a (lon, lat) vertex, matching GeoJSON's [x, y] = [lon, lat]
// axis order.
type point struct{ lon, lat float64 }

// ring is a closed sequence of points; a polygon is one exterior ring
// plus zero or more hole rings.
type ring []point

// geoPolygon is one polygon's rings plus its bounding box, precomputed
// once so rasterization can reject most cells with a cheap comparison
// before running the full point-in-polygon test.
type geoPolygon struct {
	rings          []ring
	minLon, maxLon float64
	minLat, maxLat float64
}

func main() {
	src := flag.String("src", defaultSrc, "Natural Earth land GeoJSON: URL or local file path")
	out := flag.String("out", "internal/widgets/worldmask.go", "output Go file")
	flag.Parse()

	data, err := load(*src)
	if err != nil {
		log.Fatalf("mapgen: load %s: %v", *src, err)
	}

	polys, err := parseGeoJSON(data)
	if err != nil {
		log.Fatalf("mapgen: parse %s: %v", *src, err)
	}
	if len(polys) == 0 {
		log.Fatalf("mapgen: %s contained no polygons", *src)
	}

	mask := rasterize(polys)

	if err := writeMask(*out, *src, mask); err != nil {
		log.Fatalf("mapgen: write %s: %v", *out, err)
	}
	fmt.Printf("mapgen: wrote %s (%d polygons, %dx%d cells)\n", *out, len(polys), gridWidth, gridHeight)
}

// load reads src as a URL if it looks like one, otherwise as a local
// file. This is what lets the mask be regenerated offline from a
// cached copy of the same GeoJSON.
func load(src string) ([]byte, error) {
	if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Get(src)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
		}
		return io.ReadAll(resp.Body)
	}
	return os.ReadFile(src)
}

// geoJSON is the minimal shape needed from a FeatureCollection of
// Polygon/MultiPolygon land features. Coordinates are read as raw
// json.RawMessage and decoded per-geometry-type, since GeoJSON nests
// coordinates one level deeper for MultiPolygon than for Polygon.
type geoJSON struct {
	Features []struct {
		Geometry struct {
			Type        string          `json:"type"`
			Coordinates json.RawMessage `json:"coordinates"`
		} `json:"geometry"`
	} `json:"features"`
}

// parseGeoJSON extracts every polygon (each with its rings) from a
// FeatureCollection, computing each polygon's bounding box up front.
func parseGeoJSON(data []byte) ([]geoPolygon, error) {
	var fc geoJSON
	if err := json.Unmarshal(data, &fc); err != nil {
		return nil, err
	}

	var polys []geoPolygon
	for _, f := range fc.Features {
		switch f.Geometry.Type {
		case "Polygon":
			var rings [][][2]float64
			if err := json.Unmarshal(f.Geometry.Coordinates, &rings); err != nil {
				return nil, err
			}
			polys = append(polys, newPolygon(rings))
		case "MultiPolygon":
			var mp [][][][2]float64
			if err := json.Unmarshal(f.Geometry.Coordinates, &mp); err != nil {
				return nil, err
			}
			for _, rings := range mp {
				polys = append(polys, newPolygon(rings))
			}
		}
	}
	return polys, nil
}

// newPolygon converts raw [lon,lat] rings into a geoPolygon with its
// bounding box precomputed.
func newPolygon(raw [][][2]float64) geoPolygon {
	p := geoPolygon{minLon: 180, maxLon: -180, minLat: 90, maxLat: -90}
	for _, raw := range raw {
		r := make(ring, len(raw))
		for i, c := range raw {
			r[i] = point{lon: c[0], lat: c[1]}
			p.minLon = min(p.minLon, r[i].lon)
			p.maxLon = max(p.maxLon, r[i].lon)
			p.minLat = min(p.minLat, r[i].lat)
			p.maxLat = max(p.maxLat, r[i].lat)
		}
		p.rings = append(p.rings, r)
	}
	return p
}

// contains is an even-odd point-in-polygon test summed across every
// ring of the polygon in one pass. Running the exterior ring and every
// hole ring through the same parity count is what makes holes work
// automatically: crossing into a hole flips parity back to "outside"
// without any special-case subtraction.
func (p geoPolygon) contains(lon, lat float64) bool {
	if lon < p.minLon || lon > p.maxLon || lat < p.minLat || lat > p.maxLat {
		return false
	}
	inside := false
	for _, r := range p.rings {
		n := len(r)
		for i := 0; i < n; i++ {
			a, b := r[i], r[(i+1)%n]
			if (a.lat > lat) != (b.lat > lat) {
				xCross := a.lon + (lat-a.lat)*(b.lon-a.lon)/(b.lat-a.lat)
				if lon < xCross {
					inside = !inside
				}
			}
		}
	}
	return inside
}

// rasterize samples one point per cell at the cell's centre — the
// convention documented on the generated file — against every
// polygon, OR-ing the result. It returns gridHeight strings of
// gridWidth characters, '#' for land and '.' for sea.
func rasterize(polys []geoPolygon) []string {
	rows := make([]string, gridHeight)
	for j := 0; j < gridHeight; j++ {
		lat := north - (float64(j)+0.5)*(north-south)/gridHeight
		var b strings.Builder
		b.Grow(gridWidth)
		for i := 0; i < gridWidth; i++ {
			lon := -180.0 + (float64(i)+0.5)*360.0/gridWidth
			land := false
			for _, p := range polys {
				if p.contains(lon, lat) {
					land = true
					break
				}
			}
			if land {
				b.WriteByte('#')
			} else {
				b.WriteByte('.')
			}
		}
		rows[j] = b.String()
	}
	return rows
}

// writeMask emits the generated Go source. The header records
// provenance and the regeneration command, so a future reader never
// has to guess where the data came from or how to refresh it.
func writeMask(path, src string, rows []string) error {
	var b strings.Builder
	fmt.Fprintf(&b, "// Code generated by tools/mapgen; DO NOT EDIT.\n")
	fmt.Fprintf(&b, "//\n")
	fmt.Fprintf(&b, "// Source: Natural Earth 110m land (public domain, naturalearthdata.com),\n")
	fmt.Fprintf(&b, "// fetched from:\n")
	fmt.Fprintf(&b, "//\n")
	fmt.Fprintf(&b, "//\t%s\n", src)
	fmt.Fprintf(&b, "//\n")
	fmt.Fprintf(&b, "// Regenerate with:\n")
	fmt.Fprintf(&b, "//\n")
	fmt.Fprintf(&b, "//\tgo run ./tools/mapgen -out internal/widgets/worldmask.go\n")
	fmt.Fprintf(&b, "package widgets\n\n")
	fmt.Fprintf(&b, "// landMask is a %dx%d equirectangular land/sea grid at 1° per cell,\n", gridWidth, gridHeight)
	fmt.Fprintf(&b, "// cropped %g°N..%g°S (row 0 is the north edge). Each row is exactly\n", north, -south)
	fmt.Fprintf(&b, "// %d columns spanning -180°..180° longitude; '#' is land sampled at\n", gridWidth)
	fmt.Fprintf(&b, "// the cell centre, '.' is sea. See worldmap.go for how this is\n")
	fmt.Fprintf(&b, "// projected and downsampled to the terminal's size.\n")
	fmt.Fprintf(&b, "var landMask = [%d]string{\n", gridHeight)
	for _, row := range rows {
		fmt.Fprintf(&b, "\t%q,\n", row)
	}
	fmt.Fprintf(&b, "}\n")

	return os.WriteFile(path, []byte(b.String()), 0o644)
}
