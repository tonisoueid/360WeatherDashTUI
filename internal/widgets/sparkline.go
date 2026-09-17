package widgets

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// sparkRamp is the eight-level block ramp used for value series.
var sparkRamp = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// precipRamp shades precipitation probability. The first level is a
// mid-dot rather than a space so an empty forecast still reads as a
// row of data rather than a rendering gap.
var precipRamp = []rune{'·', '░', '▒', '▓', '█'}

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

// PrecipBand renders precipitation probabilities as a shaded band
// exactly width columns wide.
//
// Unlike Sparkline this uses ABSOLUTE thresholds, not a normalised
// range: an 80% chance must look the same in every row, or the band
// stops answering "will it rain" and starts answering "is this hour
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
