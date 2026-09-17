package ui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
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
