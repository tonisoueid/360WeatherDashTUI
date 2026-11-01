package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/tonisoueid/360WeatherDashTUI/internal/widgets"
)

// humanAge renders a duration the way a person would say it.
func humanAge(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

// footerHint is one key hint in the footer bar: a function, bold, in a
// colour of its own.
type footerHint struct {
	text  string
	style lipglossStyle
}

// footerHints lists the functions left to right, most essential first:
// footerWith drops from the END of this list when the line runs out
// of room, so a hint's position here doubles as its survival priority
// under space pressure.
func footerHints(th theme) []footerHint {
	return []footerHint{
		{"↑↓ city", th.hintCity},
		{"←→ day", th.hintDay},
		{"⏎ zoom", th.hintZoom},
		{"m map", th.hintMap},
		{"a add", th.hintAdd},
		{"/ filter", th.hintFilter},
		{"s sort", th.hintSort},
		{"? help", th.hintHelp},
	}
}

// detailHints lists the detail screen's functions, in the same
// one-colour-per-function voice as the grid's footer.
func detailHints(th theme) []footerHint {
	return []footerHint{
		{"←→ day", th.hintDay},
		{"↑↓ city", th.hintCity},
		{"esc back", th.hintBack},
		{"u units", th.hintUnits},
		{"? help", th.hintHelp},
	}
}

// mapHints lists the world map's functions. "m grid" leads rather than
// trailing, because on this screen going back to the list is the
// action most people reach for first.
func mapHints(th theme) []footerHint {
	return []footerHint{
		{"m grid", th.hintBack},
		{"←→ day", th.hintDay},
		{"↑↓ city", th.hintCity},
		{"⏎ zoom", th.hintZoom},
		{"u units", th.hintUnits},
		{"? help", th.hintHelp},
	}
}

// joinHints lays out a hint line: bold coloured functions, dim
// separators, the same voice on every screen.
func joinHints(th theme, hs []footerHint) string {
	var parts []string
	for _, h := range hs {
		if len(parts) > 0 {
			parts = append(parts, th.dim.Render("·"))
		}
		parts = append(parts, h.style.Render(h.text))
	}
	return " " + strings.Join(parts, " ")
}

// gridHints returns the grid's hint list, with the "sorted: X" chip
// prefixed when a non-manual sort is active.
func (m Model) gridHints(th theme) []footerHint {
	hs := footerHints(th)
	if m.sortMode != sortManual {
		hs = append([]footerHint{{"sorted: " + m.sortMode.String(), th.hintSorted}}, hs...)
	}
	return hs
}

// footer renders the grid's key hints plus a data-freshness indicator.
//
// Freshness is shown at all times, not only on failure: a dashboard
// quietly displaying this morning's weather at six in the evening is
// worse than one showing nothing, because nothing on screen says so.
func (m Model) footer(width int) string {
	th := newTheme()
	return m.footerWith(width, th, m.gridHints(th))
}

// mapFooter renders the world map's key hints plus the same freshness
// indicator the grid shows — the map is a second home view, not a
// lesser one, so it does not lose the "is this stale" signal.
func (m Model) mapFooter(width int) string {
	th := newTheme()
	return m.footerWith(width, th, mapHints(th))
}

// footerWith lays out one hint line plus the freshness/status text
// that both footer and mapFooter share.
//
// When both don't fit, hints are dropped ONE AT A TIME from the end of
// hs — least essential first, per the order footerHints/mapHints
// declare — rather than all at once. A status message ("updated 3m
// ago") is common, not exceptional, and losing every hint the moment
// one appears would make the footer useless on the single most common
// terminal width. Only when nothing at all fits does status take the
// whole line.
func (m Model) footerWith(width int, th theme, hs []footerHint) string {
	var status string
	switch {
	case m.notice != "":
		status = th.warn.Bold(true).Render(m.notice)
	case m.loading:
		status = m.spinner.View() + th.footerText.Render(" refreshing…")
	case m.lastErr != nil && !m.lastOK.IsZero():
		status = th.warn.Bold(true).Render(
			fmt.Sprintf("⚠ stale %s · %s", humanAge(m.now().Sub(m.lastOK)), errSummary(m.lastErr)))
	case m.lastErr != nil:
		status = th.warn.Bold(true).Render("⚠ " + errSummary(m.lastErr))
	case !m.lastOK.IsZero():
		status = th.dim.Bold(true).Render("updated " + humanAge(m.now().Sub(m.lastOK)))
	default:
		status = th.dim.Bold(true).Render("no data yet")
	}

	for {
		hints := joinHints(th, hs)
		gap := width - lipglossWidth(hints) - lipglossWidth(status) - 2
		if gap >= 1 {
			return widgets.Pad(hints+strings.Repeat(" ", gap)+status+" ", width)
		}
		if len(hs) == 0 {
			// Too narrow for both: the status wins, because it is the
			// part the user cannot get any other way.
			return widgets.Pad(" "+status, width)
		}
		hs = hs[:len(hs)-1]
	}
}

// errSummary trims an error to something that fits on one line.
func errSummary(err error) string {
	s := err.Error()
	if len(s) > 40 {
		s = s[:37] + "…"
	}
	return s
}
