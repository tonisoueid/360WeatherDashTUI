package ui

import (
	"errors"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/tsoueid/weatherdash/internal/weather"
)

// Init renders from cache first, then starts a background refresh.
//
// The model already holds cached forecasts when this runs, so View()
// draws a populated grid before the request is even sent. That is what
// makes startup feel instant rather than one network round-trip.
//
// The fetch itself is kicked off as a message rather than launched
// here: Init's receiver is a copy the program discards, so consuming a
// reqID inside it would mark the first response stale forever. Only
// Update's returned model replaces the program's.
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, kickRefreshCmd())
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

	case refreshKickMsg:
		return m.refreshCmd()

	case debounceMsg:
		// Only the newest debounce fires a request.
		if msg.seq != m.searchSeq || strings.TrimSpace(msg.query) == "" {
			return m, nil
		}
		m.searching = true
		return m, geocodeCmd(m.geocoder, msg.query, msg.seq)

	case geocodeMsg:
		// Responses can arrive out of order; ignore superseded ones.
		if msg.seq != m.searchSeq {
			return m, nil
		}
		m.searching = false
		m.searchErr = msg.err
		if msg.err == nil {
			m.searchResults = msg.results
			m.searchSel = 0
		}
		return m, nil

	case configSavedMsg:
		if msg.err != nil {
			m.lastErr = msg.err
		}
		return m, nil

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
	case screenDetail:
		return m.handleDetailKey(msg)
	case screenAddCity:
		return m.handleAddCityKey(msg)
	}
	return m, nil
}

func (m Model) handleGridKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.confirmDelete && !key.Matches(msg, m.keys.Delete) {
		m.confirmDelete = false
	}

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
	case key.Matches(msg, m.keys.Enter):
		// Nothing to zoom into with no cities; stay put rather than
		// opening an empty detail screen.
		if len(m.cities) > 0 {
			m.screen = screenDetail
		}
	case key.Matches(msg, m.keys.Add):
		m.screen = screenAddCity
		m.search.SetValue("")
		m.search.Focus()
		m.searchResults = nil
		m.searchSel = 0
		m.searchErr = nil
		return m, textinput.Blink

	case key.Matches(msg, m.keys.Delete):
		if !m.confirmDelete {
			// Deleting a city is cheap to redo but annoying to
			// discover, so it takes two presses rather than one.
			m.confirmDelete = true
			return m, nil
		}
		m.confirmDelete = false
		m = m.deleteSelectedCity()
		return m, saveConfigCmd(m.configPath, m.cfg)
	}
	return m, nil
}

// handleDetailKey moves the SAME cursor the grid uses, so escaping
// returns to exactly the cell the user zoomed out of and panning inside
// the detail view needs no round trip through the grid.
func (m Model) handleDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Back):
		m.screen = screenGrid
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
