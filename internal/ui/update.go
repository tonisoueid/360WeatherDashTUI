package ui

import (
	"errors"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/tsoueid/weatherdash/internal/weather"
)

// Init renders from cache first, then starts a background refresh.
//
// The model already holds cached forecasts when this runs, so View()
// draws a populated grid before the request is even sent. That is what
// makes startup feel instant rather than one network round-trip.
func (m Model) Init() tea.Cmd {
	next, fetch := m.refreshCmd()
	m = next
	cmds := []tea.Cmd{m.spinner.Tick}
	if fetch != nil {
		cmds = append(cmds, fetch)
	}
	return tea.Batch(cmds...)
}

// refreshCmd starts a fetch, returning the updated model and command.
//
// It consumes a request ID so that any response from an earlier request
// can be recognised and discarded.
func (m Model) refreshCmd() (Model, tea.Cmd) {
	if m.provider == nil || len(m.cities) == 0 {
		return m, nil
	}
	if time.Now().Before(m.backoffTil) {
		// Rate limited. Refusing here rather than at the transport keeps
		// us from spending the user's quota on requests we know will fail.
		return m, nil
	}

	m.reqID++
	m.loading = true
	return m, fetchCmd(m.provider, m.cities, m.reqID)
}

// applyForecasts merges a fetch result into the model.
func (m Model) applyForecasts(msg forecastsMsg) Model {
	// Discard superseded responses. Without this, a manual r landing
	// after the ticker's request would let the older result overwrite
	// the newer one.
	if msg.reqID != m.reqID {
		return m
	}
	m.loading = false

	if msg.err != nil {
		m.lastErr = msg.err
		var rl *weather.RateLimitError
		if errors.As(msg.err, &rl) {
			m.backoffTil = time.Now().Add(rl.RetryAfter)
		}
		// Cached data is deliberately left untouched: stale numbers with
		// an honest staleness marker beat an empty grid.
		return m
	}

	m.lastErr = nil
	m.lastOK = time.Now()
	m.backoffTil = time.Time{}

	// Build the set of cities that still exist, so a result for one
	// deleted mid-flight is dropped rather than resurrected.
	live := make(map[string]bool, len(m.cities))
	for _, c := range m.cities {
		live[c.ID] = true
	}

	for i, f := range msg.forecasts {
		id := f.CityID
		if id == "" && i < len(msg.cityIDs) {
			id = msg.cityIDs[i]
		}
		if !live[id] {
			continue
		}
		f.CityID = id
		m.data[id] = f
	}

	m.clampCursor()
	return m
}

// Update is a pure function of (model, message). No I/O happens here —
// network work is returned as a tea.Cmd and arrives back as a message,
// which is what makes the entire loop testable without mocks.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		m.help.Width = msg.Width
		// A resize changes how many rows fit, which can strand the
		// cursor outside the viewport.
		m.ensureCursorVisible()
		return m, nil

	case forecastsMsg:
		m = m.applyForecasts(msg)
		return m, saveCacheCmd(m.cachePath, m.data)

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

// handleKey routes a keypress to the active screen. Phases 6 to 8 add
// the other screens' branches.
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keys.Quit) {
		return m, tea.Quit
	}

	switch m.screen {
	case screenGrid:
		return m.handleGridKey(msg)
	}
	return m, nil
}

func (m Model) handleGridKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Up):
		m.moveCity(-1)
		m.ensureCursorVisible()
	case key.Matches(msg, m.keys.Down):
		m.moveCity(1)
		m.ensureCursorVisible()
	case key.Matches(msg, m.keys.Left):
		m.moveDay(-1)
	case key.Matches(msg, m.keys.Right):
		m.moveDay(1)
	case key.Matches(msg, m.keys.Refresh):
		return m.refreshCmd()
	}
	return m, nil
}
