package ui

import (
	"time"

	"github.com/tsoueid/weatherdash/internal/geocode"
	"github.com/tsoueid/weatherdash/internal/weather"
)

// forecastsMsg is the result of one batched fetch.
//
// cityIDs is the snapshot the positional response maps onto. Carrying
// it in the message is what lets Update convert positional results into
// an ID-keyed map without ever consulting the live city list, which may
// have changed while the request was in flight.
type forecastsMsg struct {
	reqID     int
	cityIDs   []string
	forecasts []weather.Forecast
	err       error
}

// tickMsg fires the auto-refresh (Task 33).
type tickMsg time.Time

// geocodeMsg carries city-search results (Task 26).
type geocodeMsg struct {
	seq     int
	results []geocode.Place
	err     error
}

// debounceMsg fires after the search input has been idle (Task 26).
type debounceMsg struct {
	seq   int
	query string
}
