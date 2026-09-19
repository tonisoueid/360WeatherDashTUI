package widgets

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestPadIsColumnAccurate(t *testing.T) {
	tests := []struct{ name, in string }{
		{"ascii", "Paris"},
		{"accented", "Málaga"},
		{"cjk", "東京"},
		{"glyph", "☀"},
		{"empty", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Pad(tc.in, 12)
			if w := lipgloss.Width(got); w != 12 {
				t.Errorf("Pad(%q, 12) width = %d, want 12 (got %q)", tc.in, w, got)
			}
		})
	}
}

func TestPadTruncatesWhenTooLong(t *testing.T) {
	got := Pad("a very long city name indeed", 8)
	if w := lipgloss.Width(got); w != 8 {
		t.Errorf("width = %d, want 8 (got %q)", w, got)
	}
}

func TestPadLeftRightAligns(t *testing.T) {
	got := PadLeft("18°", 6)
	if got != "   18°" {
		t.Errorf("PadLeft = %q, want %q", got, "   18°")
	}
}

func TestPadZeroOrNegativeWidth(t *testing.T) {
	if got := Pad("Paris", 0); got != "" {
		t.Errorf("Pad(_, 0) = %q, want empty", got)
	}
	if got := Pad("Paris", -3); got != "" {
		t.Errorf("Pad(_, -3) = %q, want empty", got)
	}
}

func TestTruncateNeverSplitsAGlyph(t *testing.T) {
	// Truncating a double-width glyph to an odd budget must not emit
	// half a character; it pads instead.
	got := Truncate("東京都", 3)
	if w := lipgloss.Width(got); w > 3 {
		t.Errorf("Truncate width = %d, want <= 3 (got %q)", w, got)
	}
}

func TestResampleExactLength(t *testing.T) {
	in := []float64{1, 2, 3, 4}
	got := Resample(in, 4)
	for i := range in {
		if got[i] != in[i] {
			t.Errorf("Resample[%d] = %v, want %v", i, got[i], in[i])
		}
	}
}

func TestResampleDownsamplesByAveraging(t *testing.T) {
	// Four points into two buckets: means of (1,3) and (5,7).
	got := Resample([]float64{1, 3, 5, 7}, 2)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0] != 2 || got[1] != 6 {
		t.Errorf("Resample = %v, want [2 6]", got)
	}
}

func TestResampleUpsamples(t *testing.T) {
	got := Resample([]float64{0, 10}, 5)
	if len(got) != 5 {
		t.Fatalf("len = %d, want 5", len(got))
	}
	if got[0] != 0 || got[4] != 10 {
		t.Errorf("endpoints = %v,%v; want 0,10", got[0], got[4])
	}
}

func TestResampleEdgeCases(t *testing.T) {
	if got := Resample(nil, 5); len(got) != 0 {
		t.Errorf("Resample(nil, 5) = %v, want empty", got)
	}
	if got := Resample([]float64{1, 2}, 0); len(got) != 0 {
		t.Errorf("Resample(_, 0) = %v, want empty", got)
	}
}

func TestSplicePreservesWidthAndStyle(t *testing.T) {
	line := "\x1b[31mRed\x1b[0m tail"
	got := Splice(line, 1, "XY")
	if w := lipgloss.Width(got); w != lipgloss.Width(line) {
		t.Errorf("Splice width = %d, want %d: %q", w, lipgloss.Width(line), got)
	}
	// The covered cells are gone, the tail keeps its colour by
	// replaying the active SGR after the insertion.
	if !strings.HasPrefix(got, "\x1b[31mR") {
		t.Errorf("the left side must keep its prefix: %q", got)
	}
	if !strings.Contains(got, "XY") || strings.Contains(got, "ed tail") {
		t.Errorf("the covered cells must be replaced: %q", got)
	}
	if !strings.Contains(got, "XY\x1b[31m\x1b[0m tail") {
		t.Errorf("the tail must replay the colour active at the cut: %q", got)
	}
}

func TestSplicePlainLines(t *testing.T) {
	got := Splice("abcdef", 2, "XY")
	if got != "abXYef" {
		t.Errorf("Splice plain = %q, want abXYef", got)
	}
	if got := Splice("abc", 5, "Z"); got != "abcZ" {
		t.Errorf("Splice past the end = %q, want abcZ", got)
	}
}

func TestCellAt(t *testing.T) {
	if got := CellAt("abc", 1); got != 'b' {
		t.Errorf("CellAt plain = %q, want 'b'", got)
	}
	// Escape sequences take no columns.
	styled := "\x1b[31mab\x1b[0mc"
	if got := CellAt(styled, 0); got != 'a' {
		t.Errorf("CellAt styled col 0 = %q, want 'a'", got)
	}
	if got := CellAt(styled, 2); got != 'c' {
		t.Errorf("CellAt styled col 2 = %q, want 'c'", got)
	}
	// Out-of-range columns read as blank, not garbage.
	if got := CellAt("ab", 5); got != ' ' {
		t.Errorf("CellAt past end = %q, want blank", got)
	}
	if got := CellAt("ab", -1); got != ' ' {
		t.Errorf("CellAt before start = %q, want blank", got)
	}
	if got := CellAt("", 0); got != ' ' {
		t.Errorf("CellAt empty = %q, want blank", got)
	}
}
