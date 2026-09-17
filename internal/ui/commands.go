package ui

import (
	"context"
	"errors"
	"time"

	tea "github.com/charmbracelet/bubbletea"

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
