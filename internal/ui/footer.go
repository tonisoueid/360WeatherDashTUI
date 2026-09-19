package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/tsoueid/weatherdash/internal/widgets"
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

// footerHints lists the functions left to right. The styles come from
// the theme, so the palette stays changeable in one place.
func footerHints(th theme) []footerHint {
	return []footerHint{
		{"↑↓ city", th.hintCity},
		{"←→ day", th.hintDay},
		{"⏎ zoom", th.hintZoom},
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

// renderHints lays out the hint line: bold text, one colour per
// function, dim separators. lipglossWidth measures the styled string,
// so the ANSI codes never skew the caller's gap math.
func (m Model) renderHints(th theme) string {
	hs := footerHints(th)
	if m.sortMode != sortManual {
		hs = append([]footerHint{{"sorted: " + m.sortMode.String(), th.hintSorted}}, hs...)
	}
	return joinHints(th, hs)
}

// footer renders the key hints plus a data-freshness indicator.
//
// Freshness is shown at all times, not only on failure: a dashboard
// quietly displaying this morning's weather at six in the evening is
// worse than one showing nothing, because nothing on screen says so.
func (m Model) footer(width int) string {
	th := newTheme()

	hints := m.renderHints(th)

	var status string
	switch {
	case m.notice != "":
		status = th.warn.Bold(true).Render(m.notice)
	case m.loading:
		status = m.spinner.View() + th.footerText.Render(" refreshing…")
	case m.lastErr != nil && !m.lastOK.IsZero():
		status = th.warn.Bold(true).Render(
			fmt.Sprintf("⚠ stale %s · %s", humanAge(time.Since(m.lastOK)), errSummary(m.lastErr)))
	case m.lastErr != nil:
		status = th.warn.Bold(true).Render("⚠ " + errSummary(m.lastErr))
	case !m.lastOK.IsZero():
		status = th.dim.Bold(true).Render("updated " + humanAge(time.Since(m.lastOK)))
	default:
		status = th.dim.Bold(true).Render("no data yet")
	}

	gap := width - lipglossWidth(hints) - lipglossWidth(status) - 2
	if gap < 1 {
		// Too narrow for both: the status wins, because it is the part
		// the user cannot get any other way.
		return widgets.Pad(" "+status, width)
	}
	return widgets.Pad(hints+strings.Repeat(" ", gap)+status+" ", width)
}

// errSummary trims an error to something that fits on one line.
func errSummary(err error) string {
	s := err.Error()
	if len(s) > 40 {
		s = s[:37] + "…"
	}
	return s
}
