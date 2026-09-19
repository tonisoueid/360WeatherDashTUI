package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestFlagLinesUnknownCodes(t *testing.T) {
	for _, cc := range []string{"", "X", "ZZ", "1F", "  "} {
		if got := flagLines(cc); got != nil {
			t.Errorf("flagLines(%q) = %v, want nil", cc, got)
		}
	}
}

func TestFlagLinesCaseInsensitive(t *testing.T) {
	a, b := flagLines("fr"), flagLines("FR")
	if len(a) != len(b) {
		t.Fatal("flagLines must accept lowercase country codes")
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("line %d differs: %q vs %q", i, a[i], b[i])
		}
	}
}

// The flag must be drawn in colour, not typed as text: four lines of
// twelve half-blocks, each carrying foreground and background colour,
// framed by the light panel.
func TestFlagLinesAreColouredArt(t *testing.T) {
	prev := lipgloss.ColorProfile()
	defer lipgloss.SetColorProfile(prev)
	lipgloss.SetColorProfile(termenv.ANSI256)

	lines := flagLines("FR")
	if len(lines) != 4 {
		t.Fatalf("flagLines renders %d lines, want 4", len(lines))
	}
	for i, l := range lines {
		if w := lipgloss.Width(l); w != 2+12+2 {
			t.Errorf("line %d width = %d, want 16: %q", i, w, l)
		}
		if n := strings.Count(l, "▀"); n != 12 {
			t.Errorf("line %d has %d half blocks, want 12: %q", i, n, l)
		}
		if !strings.Contains(l, "38;5;") || !strings.Contains(l, "48;5;") {
			t.Errorf("line %d must set foreground and background colour: %q", i, l)
		}
	}
}

// Every cell of every grid must be well-formed hex, and every flag
// must use at least two colours, or it renders as a coloured block.
func TestFlagGridsWellFormed(t *testing.T) {
	for cc, g := range bigFlags {
		seen := map[string]bool{}
		for _, row := range g {
			for _, c := range row {
				var r, gr, b int
				if n, err := fmt.Sscanf(c, "#%02x%02x%02x", &r, &gr, &b); err != nil || n != 3 {
					t.Errorf("%s colour %q is not #rrggbb", cc, c)
				}
				seen[c] = true
			}
		}
		if len(seen) < 2 {
			t.Errorf("%s uses only %d distinct colour(s), want at least 2", cc, len(seen))
		}
	}
}

// Pure black cells vanish against a dark terminal background — the
// Germany flag's hoist stripe is the cautionary tale — so no colour
// may be near-black in every channel.
func TestFlagColoursVisibleOnDarkTerminals(t *testing.T) {
	const floor = 0x1A
	for cc, g := range bigFlags {
		for _, row := range g {
			for _, c := range row {
				var r, gr, b int
				if _, err := fmt.Sscanf(c, "#%02x%02x%02x", &r, &gr, &b); err != nil {
					continue // malformed hex is reported by the test above
				}
				if r < floor && gr < floor && b < floor {
					t.Errorf("%s colour %q is near-black in every channel, invisible on a dark terminal",
						cc, c)
				}
			}
		}
	}
}

// Tricolour orientation is easy to get subtly wrong: Italy and Ireland
// run their stripes hoist-to-fly (columns), Spain stacks its stripes
// (rows) at the exact 1:2:1. A regression here rotates the flag.
func TestTricolourOrientations(t *testing.T) {
	if it := bigFlags["IT"]; it[0][0] != "#009246" || it[7][11] != "#CE2B37" || it[0][5] != "#FFFFFF" {
		t.Errorf("Italy must run green at the hoist through white to red at the fly")
	}
	if ie := bigFlags["IE"]; ie[0][0] != "#169B62" || ie[7][11] != "#FF883E" || ie[0][5] != "#FFFFFF" {
		t.Errorf("Ireland must run green at the hoist through white to orange at the fly")
	}
	es := bigFlags["ES"]
	if es[0][5] != "#AA151B" || es[7][5] != "#AA151B" || es[4][5] != "#F1BF00" {
		t.Errorf("Spain must stack red over yellow over red")
	}
	yellow := 0
	for _, row := range es {
		for _, c := range row {
			if c == "#F1BF00" {
				yellow++
			}
		}
	}
	if yellow != 4*12 {
		t.Errorf("Spain's yellow band must be exactly half the flag (1:2:1): %d of 96 cells are yellow", yellow)
	}
}

// The two flags that forced the big renderer keep their identity
// colours: canton, stripes, and field must all survive.
func TestUnionJackAndOldGloryColours(t *testing.T) {
	for cc, want := range map[string][]string{
		"GB": {"#012169", "#C8102E", "#FFFFFF"},
		"US": {"#3C3B6E", "#B22234", "#FFFFFF"},
	} {
		seen := map[string]bool{}
		for _, row := range bigFlags[cc] {
			for _, c := range row {
				seen[c] = true
			}
		}
		for _, w := range want {
			if !seen[w] {
				t.Errorf("%s lost its %s colour", cc, w)
			}
		}
	}
}
