package widgets

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
)

func TestSunArcWidth(t *testing.T) {
	rise := time.Date(2026, 9, 17, 7, 0, 0, 0, time.UTC)
	set := time.Date(2026, 9, 17, 19, 0, 0, 0, time.UTC)
	now := time.Date(2026, 9, 17, 13, 0, 0, 0, time.UTC)

	for _, w := range []int{10, 20, 40, 60} {
		got := SunArc(rise, set, now, w)
		if lipgloss.Width(got) != w {
			t.Errorf("SunArc width = %d, want %d (%q)", lipgloss.Width(got), w, got)
		}
	}
}

func TestSunArcMarksCurrentPosition(t *testing.T) {
	rise := time.Date(2026, 9, 17, 6, 0, 0, 0, time.UTC)
	set := time.Date(2026, 9, 17, 18, 0, 0, 0, time.UTC)
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC) // exactly midday

	got := SunArc(rise, set, now, 13)
	// Runes here are multi-byte (━ is U+2501, 3 bytes in UTF-8), so the
	// column position is the RUNE index, not strings.IndexRune's byte
	// offset — using the byte offset directly would overshoot by 2 for
	// every rune preceding the marker.
	runes := []rune(got)
	idx := -1
	for i, r := range runes {
		if r == '☉' {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatalf("SunArc %q has no sun marker", got)
	}
	// Midday through a 13-column arc lands on the centre column.
	if idx != 6 {
		t.Errorf("sun at column %d, want 6 (%q)", idx, got)
	}
}

// Outside daylight hours there is no position to mark, so the arc
// renders without a marker rather than clamping it to an end.
func TestSunArcOmitsMarkerAtNight(t *testing.T) {
	rise := time.Date(2026, 9, 17, 7, 0, 0, 0, time.UTC)
	set := time.Date(2026, 9, 17, 19, 0, 0, 0, time.UTC)
	night := time.Date(2026, 9, 17, 23, 0, 0, 0, time.UTC)

	got := SunArc(rise, set, night, 20)
	if strings.ContainsRune(got, '☉') {
		t.Errorf("SunArc at night = %q, want no sun marker", got)
	}
	if lipgloss.Width(got) != 20 {
		t.Errorf("width = %d, want 20", lipgloss.Width(got))
	}
}

func TestSunArcZeroTimes(t *testing.T) {
	got := SunArc(time.Time{}, time.Time{}, time.Now(), 10)
	if lipgloss.Width(got) != 10 {
		t.Errorf("width = %d, want 10", lipgloss.Width(got))
	}
}
