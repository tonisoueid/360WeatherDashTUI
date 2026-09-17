package ui

import (
	"context"
	"errors"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/tsoueid/weatherdash/internal/geocode"
	"github.com/tsoueid/weatherdash/internal/store"
	"github.com/tsoueid/weatherdash/internal/weather"
)

// fetchTimeout bounds one batched request. Twenty cities is a single
// call, so a slow response means a slow network, not a large payload.
const fetchTimeout = 20 * time.Second

// fetchCmd builds the command that retrieves every city's forecast in
// one request.
//
// The city slice is COPIED into the closure rather than read from the
// model. The command runs on another goroutine after Update returns, so
// reading live model state here would see the list as it is when the
// response lands — and a city deleted meanwhile would shift every
// result by one, silently pairing each city with its neighbour's
// weather.
func fetchCmd(p weather.Provider, cities []weather.City, reqID int) tea.Cmd {
	snapshot := make([]weather.City, len(cities))
	copy(snapshot, cities)

	ids := make([]string, len(snapshot))
	for i, c := range snapshot {
		ids[i] = c.ID
	}

	return func() tea.Msg {
		if p == nil {
			return forecastsMsg{reqID: reqID, cityIDs: ids,
				err: errors.New("no weather provider configured")}
		}

		ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
		defer cancel()

		forecasts, err := p.FetchAll(ctx, snapshot)
		return forecastsMsg{
			reqID:     reqID,
			cityIDs:   ids,
			forecasts: forecasts,
			err:       err,
		}
	}
}

// kickRefreshCmd posts the message that starts the first fetch.
//
// See refreshKickMsg for why the initial refresh cannot be started
// directly inside Init.
func kickRefreshCmd() tea.Cmd {
	return func() tea.Msg { return refreshKickMsg{} }
}

// saveCacheCmd persists forecasts in the background.
//
// The error is swallowed on purpose: a cache that fails to write costs
// one slow startup and nothing else, so interrupting the user with it
// would be noise. Config write failures, which lose saved cities, are
// surfaced — see Task 28.
func saveCacheCmd(path string, data map[string]weather.Forecast) tea.Cmd {
	if path == "" || len(data) == 0 {
		return nil
	}
	snapshot := make(map[string]weather.Forecast, len(data))
	for k, v := range data {
		snapshot[k] = v
	}
	return func() tea.Msg {
		c := store.NewCache()
		c.Forecasts = snapshot
		_ = store.SaveCache(path, c)
		return nil
	}
}

// searchDebounce is how long the input must be idle before a search
// fires. Long enough that typing a city name costs one request rather
// than one per character; short enough to feel immediate.
const searchDebounce = 250 * time.Millisecond

// debounceCmd fires after the search input has been idle.
//
// Every keystroke schedules one of these. Only the message whose seq
// matches the model's current sequence triggers a request, so the four
// timers left over from typing "Pari" expire harmlessly.
func debounceCmd(query string, seq int) tea.Cmd {
	return tea.Tick(searchDebounce, func(time.Time) tea.Msg {
		return debounceMsg{seq: seq, query: query}
	})
}

// geocodeCmd performs a city search.
//
// The result carries its sequence number because responses can arrive
// out of order: a request for "Pari" may well land after the one for
// "Paris", and applying it would replace the right answer with a stale
// one.
func geocodeCmd(g *geocode.Client, query string, seq int) tea.Cmd {
	return func() tea.Msg {
		if g == nil {
			return geocodeMsg{seq: seq, err: errors.New("no geocoder configured")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		results, err := g.Search(ctx, query)
		return geocodeMsg{seq: seq, results: results, err: err}
	}
}
