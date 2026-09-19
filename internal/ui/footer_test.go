package ui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestFooterWidthIsExact(t *testing.T) {
	m := testModel(t, 3)
	m.lastOK = time.Now().Add(-3 * time.Minute)

	for _, w := range []int{60, 80, 120} {
		if got := lipgloss.Width(m.footer(w)); got != w {
			t.Errorf("footer width = %d, want %d", got, w)
		}
	}
}

func TestFooterShowsAge(t *testing.T) {
	m := testModel(t, 3)
	m.lastOK = time.Now().Add(-3 * time.Minute)

	if !strings.Contains(m.footer(120), "3m ago") {
		t.Errorf("footer = %q, want it to mention 3m ago", m.footer(120))
	}
}

func TestFooterShowsRefreshing(t *testing.T) {
	m := testModel(t, 3)
	m.loading = true

	if !strings.Contains(strings.ToLower(m.footer(120)), "refreshing") {
		t.Errorf("footer = %q, want a refreshing indicator", m.footer(120))
	}
}

func TestFooterShowsStalenessOnError(t *testing.T) {
	m := testModel(t, 3)
	m.lastErr = errors.New("network down")
	m.lastOK = time.Now().Add(-2 * time.Hour)

	got := m.footer(120)
	if !strings.Contains(got, "⚠") {
		t.Errorf("footer = %q, want a warning marker", got)
	}
	if !strings.Contains(got, "2h") {
		t.Errorf("footer = %q, want the staleness age", got)
	}
}

func TestFooterNeverFetched(t *testing.T) {
	m := testModel(t, 3)
	if got := lipgloss.Width(m.footer(80)); got != 80 {
		t.Errorf("width = %d, want 80", got)
	}
}

func TestFooterHintsBoldOneColorPerFunction(t *testing.T) {
	// go test stdout is not a TTY, so lipgloss would strip styling;
	// pin the profile and restore it for the tests that follow.
	prev := lipgloss.ColorProfile()
	defer lipgloss.SetColorProfile(prev)
	lipgloss.SetColorProfile(termenv.ANSI256)

	m := testModel(t, 3)
	got := m.footer(120)

	functions := []string{"↑↓ city", "←→ day", "⏎ zoom", "m map", "a add", "/ filter", "s sort", "? help"}
	seen := map[string]string{}
	for _, fn := range functions {
		if !strings.Contains(got, fn) {
			t.Errorf("footer = %q, want a hint for %q", got, fn)
			continue
		}
		sgr := sgrBefore(got, fn)
		if !strings.HasPrefix(sgr, "\x1b[1;") {
			t.Errorf("hint %q rendered with %q, want bold", fn, sgr)
		}
		if other, dup := seen[sgr]; dup {
			t.Errorf("hints %q and %q share %q, want one colour per function", other, fn, sgr)
		}
		seen[sgr] = fn
	}
}

// sgrBefore returns the escape sequence immediately preceding the first
// occurrence of substr in s — i.e. the style a rendered run starts with.
func sgrBefore(s, substr string) string {
	i := strings.Index(s, substr)
	if i < 0 {
		return ""
	}
	j := strings.LastIndex(s[:i], "\x1b[")
	if j < 0 {
		return ""
	}
	return s[j:i]
}

func TestHumanAge(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Second, "just now"},
		{90 * time.Second, "1m ago"},
		{45 * time.Minute, "45m ago"},
		{3 * time.Hour, "3h ago"},
		{50 * time.Hour, "2d ago"},
	}
	for _, tc := range tests {
		if got := humanAge(tc.d); got != tc.want {
			t.Errorf("humanAge(%v) = %q, want %q", tc.d, got, tc.want)
		}
	}
}
