package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// A country flag in the header is drawn, not typed: emoji flags render
// as plain letters anywhere the terminal font lacks colour emoji, and a
// three-cell glyph cannot honestly render a saltire or a canton. So
// each flag is rasterized to twelve columns × eight half-rows — four
// terminal lines tall at the true 3:2 flag aspect — and mounted on a
// light panel, which keeps dark-navy flags legible on the dark
// terminals this app targets. Eight rows also divide by two and four,
// so Spain's 1:2:1 and every bicolour land on their exact proportions.

// flagGrid is a flag as twelve columns of eight half-rows of hex
// colours.
type flagGrid [8][12]string

// flagPanelColor is the light grey behind every flag — and behind the
// whole header block, so the header reads as one banner.
var flagPanelColor = lipgloss.Color("#767676")

// flagPanel is the light mount behind every flag.
var flagPanel = lipgloss.NewStyle().Background(flagPanelColor)

// fillGrid returns a flag of one colour.
func fillGrid(color string) flagGrid {
	var g flagGrid
	for r := range g {
		for c := range g[r] {
			g[r][c] = color
		}
	}
	return g
}

// put colours single cells, for charges drawn dot by dot.
func put(g *flagGrid, color string, cells ...[2]int) {
	for _, d := range cells {
		g[d[0]][d[1]] = color
	}
}

// vert returns equal vertical stripes, hoist to fly. Twelve columns
// divide by two and three, so every vertical stripe count is exact.
func vert(stripes ...string) flagGrid {
	var g flagGrid
	for c := 0; c < 12; c++ {
		s := stripes[c*len(stripes)/12]
		for r := range g {
			g[r][c] = s
		}
	}
	return g
}

// horiz returns horizontal stripes, top to bottom. Eight rows do not
// divide by three or five, so the remainder goes to the outer bands —
// a symmetric (3,2,3) reads truer than a lopsided (3,3,2).
func horiz(stripes ...string) flagGrid {
	counts := horizRowCounts(len(stripes))
	var g flagGrid
	r := 0
	for i, s := range stripes {
		for n := 0; n < counts[i]; n++ {
			for c := range g[r] {
				g[r][c] = s
			}
			r++
		}
	}
	return g
}

// horizRowCounts splits eight rows across n bands, spare rows to the
// outer bands for symmetry.
func horizRowCounts(n int) []int {
	counts := make([]int, n)
	for i := range counts {
		counts[i] = 8 / n
	}
	for i := 0; i < 8%n; i++ {
		counts[i*2%n]++
	}
	return counts
}

// nordic returns a Nordic cross: the arm set toward the hoist at the
// traditional three-eighths, two cells thick.
func nordic(field, cross string) flagGrid {
	g := fillGrid(field)
	for r := range g {
		g[r][4], g[r][5] = cross, cross
	}
	for c := 0; c < 12; c++ {
		g[3][c], g[4][c] = cross, cross
	}
	return g
}

// discOn returns a field carrying a centred disc.
func discOn(field, charge string) flagGrid {
	g := fillGrid(field)
	for r := 2; r <= 5; r++ {
		for c := 4; c <= 7; c++ {
			g[r][c] = charge
		}
	}
	return g
}

// bigFlags is the flag table, keyed by ISO 3166-1 alpha-2 code. It is
// curated, not exhaustive: a country not listed here renders no flag
// rather than a wrong one.
var bigFlags = map[string]flagGrid{
	// Vertical stripes: Italy and Ireland are vertical tricolours —
	// green at the hoist — as are the rest of this group.
	"FR": vert("#0055A4", "#FFFFFF", "#EF4135"),
	"BE": vert("#262626", "#FDDA24", "#EF3340"),
	"MX": vert("#006847", "#FFFFFF", "#CE1126"),
	"RO": vert("#002B7F", "#FCD116", "#CE1126"),
	"NG": vert("#008751", "#FFFFFF", "#008751"),
	"CA": vert("#FF0000", "#FFFFFF", "#FF0000"),
	"PE": vert("#D91023", "#FFFFFF", "#D91023"),
	"IT": vert("#009246", "#FFFFFF", "#CE2B37"),
	"IE": vert("#169B62", "#FFFFFF", "#FF883E"),

	// Horizontal stripes. Black stripes are charcoal, not #000000, so
	// they read against a dark terminal background.
	"AT": horiz("#ED2939", "#FFFFFF", "#ED2939"),
	"AR": horiz("#74ACDF", "#FFFFFF", "#74ACDF"),
	"EG": horiz("#CE1126", "#FFFFFF", "#262626"),
	"IN": horiz("#FF9933", "#FFFFFF", "#138808"),
	"LT": horiz("#FDB913", "#006A44", "#C8102E"),
	"DE": horiz("#262626", "#DD0000", "#FFCC00"),
	"NL": horiz("#AE1C28", "#FFFFFF", "#21468B"),
	"RU": horiz("#FFFFFF", "#0039A6", "#D52B1E"),
	"HU": horiz("#CE2939", "#FFFFFF", "#477050"),
	"BG": horiz("#FFFFFF", "#00966E", "#D62612"),
	"HR": horiz("#FF0000", "#FFFFFF", "#171796"),
	"LV": horiz("#9E3039", "#FFFFFF", "#9E3039"),
	"EE": horiz("#262626", "#0072CE", "#FFFFFF"),
	"CO": horiz("#FCD116", "#003893", "#CE1126"),
	"TH": horiz("#A51931", "#FFFFFF", "#2D2A4A", "#FFFFFF", "#A51931"),
	"UA": horiz("#0057B7", "#FFD700"),
	"PL": horiz("#FFFFFF", "#DC143C"),
	"ID": horiz("#CE1126", "#FFFFFF"),
	"SG": horiz("#ED2939", "#FFFFFF"),

	// Nordic crosses.
	"SE": nordic("#006AA7", "#FECC02"),
	"DK": nordic("#C8102E", "#FFFFFF"),
	"FI": nordic("#FFFFFF", "#003580"),
	"IS": nordic("#02529C", "#DC1E35"),
	"NO": norwayFlag(),

	// Discs on a field.
	"JP": discOn("#FFFFFF", "#BC002D"),
	"BD": discOn("#006A4E", "#F72B45"),

	// Charges and composites.
	"US": usaFlag(),
	"GB": unionJack(),
	"ES": spainFlag(),
	"AU": australiaFlag(),
	"BR": brazilFlag(),
	"CN": chinaFlag(),
	"VN": vietnamFlag(),
	"TR": turkeyFlag(),
	"CH": swissFlag(),
	"GR": greeceFlag(),
	"KR": koreaFlag(),
	"CZ": czechFlag(),
	"CL": chileFlag(),
	"PT": portugalFlag(),
	"LB": lebanonFlag(),
}

// banded1to2to1 paints red-white-red (or any three colours) at the
// exact 1:2:1 the eight-row budget permits: two rows, four rows, two
// rows.
func banded1to2to1(top, mid, bot string) flagGrid {
	g := fillGrid(mid)
	for c := range g[0] {
		g[0][c], g[1][c] = top, top
		g[6][c], g[7][c] = bot, bot
	}
	return g
}

// spainFlag: red over yellow over red at the official 1:2:1.
func spainFlag() flagGrid {
	return banded1to2to1("#AA151B", "#F1BF00", "#AA151B")
}

// lebanonFlag: red-white-red at 1:2:1 with the cedar in the white.
func lebanonFlag() flagGrid {
	g := banded1to2to1("#ED1C24", "#FFFFFF", "#ED1C24")
	put(&g, "#00A651", [2]int{3, 5}, [2]int{3, 6}, [2]int{4, 5}, [2]int{4, 6})
	return g
}

// norwayFlag: red field, white-fimbriated blue cross.
func norwayFlag() flagGrid {
	g := fillGrid("#BA0C2F")
	for r := range g {
		g[r][4], g[r][5], g[r][6] = "#FFFFFF", "#00205B", "#FFFFFF"
	}
	for c := 0; c < 12; c++ {
		g[3][c], g[4][c] = "#00205B", "#00205B"
	}
	return g
}

// usaFlag: stripes under a star-spangled canton of dots.
func usaFlag() flagGrid {
	g := horiz("#B22234", "#FFFFFF", "#B22234", "#FFFFFF",
		"#B22234", "#FFFFFF", "#B22234", "#FFFFFF")
	for r := 0; r <= 3; r++ {
		for c := 0; c <= 4; c++ {
			g[r][c] = "#3C3B6E"
			if (r+c)%2 == 0 {
				g[r][c] = "#FFFFFF"
			}
		}
	}
	return g
}

// unionJack: blue field, white saltire corner to corner, red cross of
// St George fimbriated white.
func unionJack() flagGrid {
	g := fillGrid("#012169")
	for r := 0; r < 8; r++ {
		t := (r*11 + 3) / 7
		g[r][t] = "#FFFFFF"
		g[r][11-t] = "#FFFFFF"
	}
	for r := 0; r < 8; r++ {
		g[r][4], g[r][5], g[r][6], g[r][7] = "#FFFFFF", "#C8102E", "#C8102E", "#FFFFFF"
	}
	for c := 0; c < 12; c++ {
		g[3][c], g[4][c] = "#C8102E", "#C8102E"
	}
	return g
}

// australiaFlag: a whisper of the Union Jack in the canton, the
// Southern Cross, and the Commonwealth Star.
func australiaFlag() flagGrid {
	g := fillGrid("#012169")
	put(&g, "#FFFFFF",
		[2]int{0, 0}, [2]int{0, 4}, [2]int{2, 0}, [2]int{2, 4}, [2]int{1, 2})
	put(&g, "#C8102E", [2]int{0, 2}, [2]int{2, 2})
	put(&g, "#FFFFFF",
		[2]int{2, 9}, [2]int{4, 7}, [2]int{5, 10}, [2]int{7, 9}, [2]int{6, 2})
	return g
}

// brazilFlag: green field, yellow rhombus, blue disc.
func brazilFlag() flagGrid {
	g := fillGrid("#009C3B")
	rhombus := [8][2]int{{5, 6}, {4, 7}, {3, 8}, {2, 9}, {2, 9}, {3, 8}, {4, 7}, {5, 6}}
	for r, rg := range rhombus {
		for c := rg[0]; c <= rg[1]; c++ {
			g[r][c] = "#FFDF00"
		}
	}
	for r := 3; r <= 4; r++ {
		for c := 5; c <= 6; c++ {
			g[r][c] = "#002776"
		}
	}
	return g
}

// chinaFlag: the large star and three of its companions.
func chinaFlag() flagGrid {
	g := fillGrid("#DE2910")
	put(&g, "#FFDE00",
		[2]int{2, 2}, [2]int{4, 2}, [2]int{3, 1}, [2]int{3, 2}, [2]int{3, 3})
	put(&g, "#FFDE00", [2]int{1, 5}, [2]int{2, 6}, [2]int{4, 6})
	return g
}

// vietnamFlag: a five-point star, drawn as its own silhouette.
func vietnamFlag() flagGrid {
	g := fillGrid("#DA251D")
	put(&g, "#FFCD00",
		[2]int{2, 5}, [2]int{4, 5}, [2]int{3, 4}, [2]int{3, 5}, [2]int{3, 6})
	return g
}

// turkeyFlag: a crescent arc opening toward the star.
func turkeyFlag() flagGrid {
	g := fillGrid("#E30A17")
	put(&g, "#FFFFFF",
		[2]int{1, 5}, [2]int{2, 4}, [2]int{3, 4}, [2]int{4, 4}, [2]int{5, 5},
		[2]int{2, 7}, [2]int{3, 7})
	return g
}

// swissFlag: the square white cross, centred.
func swissFlag() flagGrid {
	g := fillGrid("#DA291C")
	for r := range g {
		g[r][5], g[r][6] = "#FFFFFF", "#FFFFFF"
	}
	for c := 3; c <= 8; c++ {
		g[3][c], g[4][c] = "#FFFFFF", "#FFFFFF"
	}
	return g
}

// greeceFlag: alternating stripes with the striped canton.
func greeceFlag() flagGrid {
	g := horiz("#0D5EAF", "#FFFFFF", "#0D5EAF", "#FFFFFF",
		"#0D5EAF", "#FFFFFF", "#0D5EAF", "#FFFFFF")
	for r := 0; r <= 3; r++ {
		for c := 0; c <= 5; c++ {
			g[r][c] = "#0D5EAF"
		}
		g[r][2] = "#FFFFFF"
	}
	for c := 0; c <= 5; c++ {
		g[1][c], g[2][c] = "#FFFFFF", "#FFFFFF"
	}
	return g
}

// koreaFlag: the taegeuk over the field, trigram dashes at the
// corners.
func koreaFlag() flagGrid {
	g := fillGrid("#FFFFFF")
	for c := 4; c <= 7; c++ {
		g[3][c] = "#CD2E3A"
		g[4][c] = "#0047A0"
	}
	put(&g, "#262626",
		[2]int{0, 0}, [2]int{0, 1}, [2]int{1, 0},
		[2]int{0, 10}, [2]int{0, 11}, [2]int{1, 11},
		[2]int{6, 0}, [2]int{7, 0}, [2]int{7, 1},
		[2]int{6, 11}, [2]int{7, 10}, [2]int{7, 11})
	return g
}

// czechFlag: white over red with the blue hoist triangle.
func czechFlag() flagGrid {
	g := horiz("#FFFFFF", "#D7101A")
	triangle := [8]int{4, 5, 6, 7, 7, 6, 5, 4}
	for r, width := range triangle {
		for c := 0; c < width; c++ {
			g[r][c] = "#11457E"
		}
	}
	return g
}

// chileFlag: white over red with a star-spangled blue canton.
func chileFlag() flagGrid {
	g := horiz("#FFFFFF", "#D52B1E")
	for r := 0; r <= 3; r++ {
		for c := 0; c <= 3; c++ {
			g[r][c] = "#0033A0"
		}
	}
	g[1][1] = "#FFFFFF"
	return g
}

// portugalFlag: the 2:3 green-to-red split with the sphere at the
// seam.
func portugalFlag() flagGrid {
	g := fillGrid("#DA291C")
	for r := range g {
		for c := 0; c <= 4; c++ {
			g[r][c] = "#046A38"
		}
	}
	put(&g, "#FCD116", [2]int{3, 5}, [2]int{4, 5}, [2]int{3, 6}, [2]int{4, 6})
	return g
}

// flagLines renders a country's flag as four terminal lines: twelve
// half-block cells wide — the top half of each glyph in the upper
// row's colour, the bottom half in the lower row's — framed by the
// light panel. Unknown codes return nil so the caller draws nothing.
func flagLines(cc string) []string {
	g, ok := bigFlags[strings.ToUpper(strings.TrimSpace(cc))]
	if !ok {
		return nil
	}
	lines := make([]string, 4)
	for k := range lines {
		var b strings.Builder
		for c := 0; c < 12; c++ {
			st := lipgloss.NewStyle().
				Foreground(lipgloss.Color(g[2*k][c])).
				Background(lipgloss.Color(g[2*k+1][c]))
			b.WriteString(st.Render("▀"))
		}
		lines[k] = flagPanel.Render("  ") + b.String() + flagPanel.Render("  ")
	}
	return lines
}
