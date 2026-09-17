package ui

import (
	"strings"
	"testing"
	"time"
)

func TestNoticeAppearsInTheFooter(t *testing.T) {
	m := testModel(t, 2).WithNotice("config was corrupt; backed up to config.toml.bak")

	if !strings.Contains(m.footer(120), "backed up") {
		t.Errorf("footer = %q, want the notice", m.footer(120))
	}
}

// The notice is about startup, so it must outrank routine status.
func TestNoticeOutranksFreshness(t *testing.T) {
	m := testModel(t, 2).WithNotice("config recovered")
	m.lastOK = timeNow()

	if strings.Contains(m.footer(120), "updated") {
		t.Error("the notice must take precedence over the freshness line")
	}
}

func TestNoticeClearsOnKeypress(t *testing.T) {
	m := testModel(t, 2).WithNotice("config recovered")
	m.startedAt = m.startedAt.Add(-(noticeGrace + time.Second)) // past the grace window
	m = send(m, "j")
	if m.notice != "" {
		t.Error("any keypress must dismiss the notice")
	}
}

// A terminal answering the startup colour queries emits key-shaped
// events immediately after launch; those must not dismiss the notice
// before the user has seen it.
func TestNoticeSurvivesStartupKeyChatter(t *testing.T) {
	m := testModel(t, 2).WithNotice("config recovered")
	// Freshly constructed: within the grace window.
	m = send(m, "j", "]", "\x1b[B", "enter")
	if m.notice == "" {
		t.Error("startup key chatter must not dismiss the notice")
	}
}
