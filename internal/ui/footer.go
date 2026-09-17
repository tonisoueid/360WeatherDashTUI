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

// footer renders the key hints plus a data-freshness indicator.
//
// Freshness is shown at all times, not only on failure: a dashboard
// quietly displaying this morning's weather at six in the evening is
// worse than one showing nothing, because nothing on screen says so.
func (m Model) footer(width int) string {
	th := newTheme()

	hints := " ↑↓ city · ←→ day · ⏎ zoom · a add · / filter · s sort · ? help"
	if m.sortMode != sortManual {
		hints = " sorted: " + m.sortMode.String() + " ·" + hints
	}

	var status string
	switch {
	case m.confirmDelete:
		if c, ok := m.selectedCity(); ok {
			status = th.warn.Render("press d again to delete " + c.Name + " · esc cancels")
		}
	case m.loading:
		status = m.spinner.View() + " refreshing…"
	case m.lastErr != nil && !m.lastOK.IsZero():
		status = th.warn.Render(
			fmt.Sprintf("⚠ stale %s · %s", humanAge(time.Since(m.lastOK)), errSummary(m.lastErr)))
	case m.lastErr != nil:
		status = th.warn.Render("⚠ " + errSummary(m.lastErr))
	case !m.lastOK.IsZero():
		status = th.dim.Render("updated " + humanAge(time.Since(m.lastOK)))
	default:
		status = th.dim.Render("no data yet")
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
