package ui

import (
	"sort"
	"strings"

	"github.com/tsoueid/weatherdash/internal/weather"
)

// sortMode is how the city axis is ordered.
type sortMode int

const (
	// sortManual is the user's saved order.
	sortManual sortMode = iota
	sortWarmest
	sortColdest
	sortDriest
)

func (s sortMode) String() string {
	switch s {
	case sortWarmest:
		return "warmest"
	case sortColdest:
		return "coldest"
	case sortDriest:
		return "driest"
	default:
		return "manual"
	}
}

func parseSortMode(s string) sortMode {
	switch s {
	case "warmest":
		return sortWarmest
	case "coldest":
		return sortColdest
	case "driest":
		return sortDriest
	default:
		return sortManual
	}
}

// Next cycles through the modes.
func (s sortMode) Next() sortMode {
	if s == sortDriest {
		return sortManual
	}
	return s + 1
}

// rebuildView regenerates the display order from the saved order.
//
// It is the ONLY place m.cities is derived, and it always starts from
// m.cfg.Cities. That is what keeps sorting a view transform: the saved
// order is never overwritten, so manual order is always one keypress
// away no matter how the list has been sorted.
//
// The cursor is carried across by CITY ID, not by row index. Preserving
// the index instead would teleport the user to a different city every
// time they pressed s, which reads as a bug even when the sort itself
// is correct.
func (m Model) rebuildView() Model {
	anchor := ""
	if c, ok := m.selectedCity(); ok {
		anchor = c.ID
	}

	out := make([]weather.City, 0, len(m.cfg.Cities))
	q := strings.ToLower(strings.TrimSpace(m.filterQuery))
	for _, c := range m.cfg.Cities {
		if q == "" || matchesFilter(c, q) {
			out = append(out, c)
		}
	}

	if m.sortMode != sortManual {
		sort.SliceStable(out, func(i, j int) bool {
			return m.lessBy(m.sortMode, out[i], out[j])
		})
	}

	m.cities = out

	// Re-anchor the cursor on the same city where possible.
	m.cursorCity = 0
	if anchor != "" {
		for i, c := range out {
			if c.ID == anchor {
				m.cursorCity = i
				break
			}
		}
	}

	m.clampCursor()
	m.ensureCursorVisible()
	return m
}

// lessBy ranks two cities under the active mode.
//
// All ranking keys are computed over the WHOLE forecast window, not the
// selected day: "driest" means driest week, which is the question a
// planning view is asking. Cities with no data rank last in every mode,
// so an unfetched row never displaces a real answer from the top.
func (m Model) lessBy(mode sortMode, a, b weather.City) bool {
	fa, oka := m.data[a.ID]
	fb, okb := m.data[b.ID]

	if oka != okb {
		return oka // the one with data sorts first
	}
	if !oka {
		return a.Name < b.Name
	}

	switch mode {
	case sortWarmest:
		if fa.MeanHigh() != fb.MeanHigh() {
			return fa.MeanHigh() > fb.MeanHigh()
		}
	case sortColdest:
		if fa.MeanHigh() != fb.MeanHigh() {
			return fa.MeanHigh() < fb.MeanHigh()
		}
	case sortDriest:
		if fa.TotalPrecip() != fb.TotalPrecip() {
			return fa.TotalPrecip() < fb.TotalPrecip()
		}
	}
	// Name is the tiebreak so the order is stable across refreshes.
	return a.Name < b.Name
}
