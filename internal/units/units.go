// Package units converts canonical metric values to a display system.
//
// Everything stored by weatherdash is metric. This package is the only
// place conversion happens, and it happens at render time, so toggling
// units never requires refetching or rewriting cached data.
package units

import (
	"fmt"
	"math"
)

// System is a unit display system.
type System int

const (
	// Metric renders Celsius, km/h, and millimetres.
	Metric System = iota
	// Imperial renders Fahrenheit, mph, and inches.
	Imperial
)

// String returns the config file spelling.
func (s System) String() string {
	if s == Imperial {
		return "imperial"
	}
	return "metric"
}

// Parse reads the config file spelling.
func Parse(s string) (System, error) {
	switch s {
	case "metric":
		return Metric, nil
	case "imperial":
		return Imperial, nil
	default:
		return Metric, fmt.Errorf("unknown unit system %q (want \"metric\" or \"imperial\")", s)
	}
}

// Toggle returns the other system.
func (s System) Toggle() System {
	if s == Metric {
		return Imperial
	}
	return Metric
}

// TempNum converts Celsius to the display system, rounded to a whole
// degree. Grid cells need the bare number for width budgeting.
func (s System) TempNum(c float64) int {
	if s == Imperial {
		return int(math.Round(c*9/5 + 32))
	}
	return int(math.Round(c))
}

// Temp renders a temperature with a degree sign and no unit letter —
// the letter is redundant in a dashboard where the system is global.
func (s System) Temp(c float64) string {
	return fmt.Sprintf("%d°", s.TempNum(c))
}

// Speed renders a wind speed with its unit.
func (s System) Speed(kph float64) string {
	if s == Imperial {
		return fmt.Sprintf("%d mph", int(math.Round(kph*0.621371)))
	}
	return fmt.Sprintf("%d km/h", int(math.Round(kph)))
}

// Precip renders a precipitation amount. Inches get one decimal because
// whole inches are too coarse to be useful.
func (s System) Precip(mm float64) string {
	if s == Imperial {
		return fmt.Sprintf("%.1f\"", mm/25.4)
	}
	return fmt.Sprintf("%dmm", int(math.Round(mm)))
}
