// Package widgets renders forecast data to plain strings.
//
// Every function here is pure: data in, string out, no Bubble Tea and
// no global state. That keeps View() an assembly step rather than a
// drawing method, and it means the entire visual layer is testable
// without a terminal.
//
// Colour is deliberately NOT applied here. Widgets emit plain text and
// accept an optional Colorize hook, so golden tests compare characters
// rather than ANSI escape sequences.
package widgets

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Pad left-aligns s in a field exactly w terminal columns wide,
// truncating if necessary.
//
// Width is measured with lipgloss.Width, never len: "Málaga" is 7 bytes
// but 6 columns, and "東京" is 6 bytes but 4 columns. Padding by byte
// count moves the grid's right border on every row with a non-ASCII
// name.
func Pad(s string, w int) string {
	if w <= 0 {
		return ""
	}
	s = Truncate(s, w)
	if gap := w - lipgloss.Width(s); gap > 0 {
		return s + strings.Repeat(" ", gap)
	}
	return s
}

// PadLeft right-aligns s in a field exactly w columns wide. Numeric
// columns read better right-aligned, so temperatures use this.
func PadLeft(s string, w int) string {
	if w <= 0 {
		return ""
	}
	s = Truncate(s, w)
	if gap := w - lipgloss.Width(s); gap > 0 {
		return strings.Repeat(" ", gap) + s
	}
	return s
}

// Truncate cuts s to at most w columns without ever splitting a glyph.
// A double-width rune that would straddle the boundary is dropped
// rather than half-drawn, so the result may be one column short — which
// Pad then fills.
func Truncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= w {
		return s
	}
	var b strings.Builder
	used := 0
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if used+rw > w {
			break
		}
		b.WriteRune(r)
		used += rw
	}
	return b.String()
}

// Resample maps vals onto exactly n points.
//
// Downsampling averages each bucket, which is the right choice for
// temperature and precipitation: taking every k-th sample would let a
// single spike vanish, while the mean keeps the shape honest.
// Upsampling interpolates linearly.
func Resample(vals []float64, n int) []float64 {
	if n <= 0 || len(vals) == 0 {
		return nil
	}
	if len(vals) == n {
		out := make([]float64, n)
		copy(out, vals)
		return out
	}

	out := make([]float64, n)

	if len(vals) > n {
		for i := range out {
			lo := i * len(vals) / n
			hi := (i + 1) * len(vals) / n
			if hi <= lo {
				hi = lo + 1
			}
			var sum float64
			for _, v := range vals[lo:hi] {
				sum += v
			}
			out[i] = sum / float64(hi-lo)
		}
		return out
	}

	if n == 1 {
		out[0] = vals[0]
		return out
	}
	for i := range out {
		pos := float64(i) * float64(len(vals)-1) / float64(n-1)
		lo := int(pos)
		hi := lo + 1
		if hi >= len(vals) {
			out[i] = vals[len(vals)-1]
			continue
		}
		frac := pos - float64(lo)
		out[i] = vals[lo]*(1-frac) + vals[hi]*frac
	}
	return out
}
