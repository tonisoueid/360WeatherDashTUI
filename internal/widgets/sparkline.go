package widgets

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// sparkRamp is the eight-level block ramp used for value series.
var sparkRamp = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// precipRamp shades precipitation probability. The first level is a
// mid-dot rather than a space so a dry day still reads as a row of
// data rather than a rendering gap. It shades the grid's compact
// day cells, where there is no room for numbers; the detail screen
// prints the values instead (PrecipNumbers).
var precipRamp = []rune{'·', '░', '▒', '▓', '█'}

// precipLevel maps a probability percentage to a shade. Thresholds are
// absolute and deliberately uneven: the interesting decision boundary
// sits around "might rain" (30%) and "will rain" (60%).
func precipLevel(pct float64) rune {
	switch {
	case pct < 10:
		return precipRamp[0]
	case pct < 30:
		return precipRamp[1]
	case pct < 60:
		return precipRamp[2]
	case pct < 80:
		return precipRamp[3]
	default:
		return precipRamp[4]
	}
}

// PrecipBand renders precipitation probabilities as a shaded band
// exactly width columns wide.
//
// Unlike Sparkline this uses ABSOLUTE thresholds, not a normalised
// range: an 80% chance must look the same in every cell, or the band
// stops answering "will it rain" and starts answering "is this day
// wetter than its neighbours", which is not the question.
func PrecipBand(probs []int, width int) string {
	if width <= 0 {
		return ""
	}
	if len(probs) == 0 {
		return strings.Repeat(string(precipRamp[0]), width)
	}

	vals := make([]float64, len(probs))
	for i, p := range probs {
		vals[i] = float64(p)
	}

	var b strings.Builder
	for _, v := range Resample(vals, width) {
		b.WriteRune(precipLevel(v))
	}
	return b.String()
}

// Colorize optionally styles each point of a series by its value.
// Passing nil renders plain text, which is what golden tests do so they
// compare characters instead of ANSI escape sequences.
type Colorize func(value float64) lipgloss.Style

// Sparkline renders vals as a block-character series exactly width
// columns wide, resampling as needed.
//
// The series is normalised against its own min and max, so the ramp
// always spans the data's actual range. That makes shape legible at the
// cost of absolute comparability between rows — which is the right
// trade for a 24-hour temperature curve, where the question is "when
// does it peak", not "how does this compare to Oslo".
func Sparkline(vals []float64, width int, c Colorize) string {
	if width <= 0 {
		return ""
	}
	pts := Resample(vals, width)
	if len(pts) == 0 {
		return strings.Repeat(" ", width)
	}

	min, max := pts[0], pts[0]
	for _, v := range pts {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}

	var b strings.Builder
	span := max - min
	for _, v := range pts {
		var idx int
		if span == 0 {
			// A flat series has no range to normalise against. Render a
			// consistent mid-height line rather than dividing by zero.
			// (len-1)/2, not len/2: the ramp's indices run 0..len-1, so
			// halving the highest index gives the true midpoint.
			idx = (len(sparkRamp) - 1) / 2
		} else {
			idx = int((v - min) / span * float64(len(sparkRamp)-1))
		}
		if idx < 0 {
			idx = 0
		}
		if idx >= len(sparkRamp) {
			idx = len(sparkRamp) - 1
		}

		ch := string(sparkRamp[idx])
		if c != nil {
			ch = c(v).Render(ch)
		}
		b.WriteString(ch)
	}
	return b.String()
}

// PrecipNumbers prints each hour's precipitation probability as a
// value with its own percent sign — "5%", "45%", "100%" — at that
// hour's column, exactly width columns wide: the value the reader
// wants, on the line itself, under its hour mark on the ruler's
// midnight-to-midnight scale. No key to decode, no resampling that
// averages hours into values that never happened. The trailing % also
// delimits values that touch when columns run short, so even
// shoulder-to-shoulder figures read one by one. Each value can be
// coloured by the chance it states, via the same optional hook the
// other series take.
//
// Probabilities are clamped to 0–100 so bad data cannot print five
// columns of "1000%".
func PrecipNumbers(probs []int, width int, colorize Colorize) string {
	if width <= 0 {
		return ""
	}
	if len(probs) == 0 {
		return strings.Repeat(" ", width)
	}
	return numberRow(width, len(probs), func(i int) (string, float64) {
		p := probs[i]
		if p < 0 {
			p = 0
		}
		if p > 100 {
			p = 100
		}
		return fmt.Sprintf("%d%%", p), float64(p)
	}, colorize)
}

// PrecipAmounts prints each hour's rainfall in mm — "0.4", "2.5",
// "12" — at that hour's column on the same scale, so the amount row
// reads hour by hour against the ruler like the chance row above it.
// A dry hour prints a bare dash, a value that is not a value: the row
// reads as a full day of measurements rather than a sparse one, and
// the dash never collides with anything. Each figure can be coloured
// by the intensity it states; the unit lives in the row's gutter
// label, not on every figure.
func PrecipAmounts(mms []float64, width int, colorize Colorize) string {
	if width <= 0 {
		return ""
	}
	if len(mms) == 0 {
		return strings.Repeat(" ", width)
	}
	return numberRow(width, len(mms), func(i int) (string, float64) {
		v := mms[i]
		if v < 0 {
			v = 0
		}
		if v < 0.05 {
			return "-", 0
		}
		if v < 10 {
			return fmt.Sprintf("%.1f", v), v
		}
		return fmt.Sprintf("%.0f", v), v
	}, colorize)
}

// numberRow is the engine the rain rows share: it lays one value per
// sample at that sample's column, exactly width columns wide, with
// the sample's numeric value returned for the optional colour hook.
//
// A value that would start before the previous one ends is skipped
// rather than written over it — at widths too tight for every hour, a
// missing number is honest and a garbled one is not. Text is trimmed
// to its room BEFORE styling, because cutting a styled string would
// split its escape sequence. A sample returning empty text leaves its
// column blank and claims no room.
func numberRow(width, samples int, at func(i int) (text string, value float64), colorize Colorize) string {
	var b strings.Builder
	next := 0 // first column still free
	for i := 0; i < samples; i++ {
		text, value := at(i)
		if text == "" {
			continue
		}
		pos := i * width / samples
		if pos < next {
			continue
		}
		runes := []rune(text)
		if len(runes) > width-pos {
			runes = runes[:width-pos]
		}
		b.WriteString(strings.Repeat(" ", pos-next))
		text = string(runes)
		if colorize != nil {
			text = colorize(value).Render(text)
		}
		b.WriteString(text)
		next = pos + len(runes)
	}
	if next < width {
		b.WriteString(strings.Repeat(" ", width-next))
	}
	return b.String()
}

// SparklineRows is Sparkline drawn rows lines tall, top line first,
// each exactly width columns wide. The series is normalised the same
// way, but across 8*rows levels: a column is full blocks in the lines
// below its level, one ramp glyph in the line its level falls in, and
// blank above. rows <= 1 returns the single Sparkline line.
func SparklineRows(vals []float64, width, rows int, c Colorize) []string {
	if rows <= 1 {
		return []string{Sparkline(vals, width, c)}
	}
	if width <= 0 {
		return make([]string, rows)
	}
	pts := Resample(vals, width)
	out := make([]string, rows)
	if len(pts) == 0 {
		for i := range out {
			out[i] = strings.Repeat(" ", width)
		}
		return out
	}

	min, max := pts[0], pts[0]
	for _, v := range pts {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	levels := len(sparkRamp) * rows
	span := max - min

	bs := make([]strings.Builder, rows)
	for _, v := range pts {
		idx := (levels - 1) / 2
		if span != 0 {
			idx = int((v - min) / span * float64(levels-1))
		}
		if idx < 0 {
			idx = 0
		}
		if idx >= levels {
			idx = levels - 1
		}
		for r := 0; r < rows; r++ { // r counts from the bottom
			var ch string
			switch lo := r * len(sparkRamp); {
			case idx >= lo+len(sparkRamp):
				ch = string(sparkRamp[len(sparkRamp)-1])
			case idx >= lo:
				ch = string(sparkRamp[idx-lo])
			default:
				bs[rows-1-r].WriteString(" ")
				continue
			}
			if c != nil {
				ch = c(v).Render(ch)
			}
			bs[rows-1-r].WriteString(ch)
		}
	}
	for i := range out {
		out[i] = bs[i].String()
	}
	return out
}
