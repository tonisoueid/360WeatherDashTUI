// Package geocode resolves a typed city name to coordinates and a
// timezone using the Open-Meteo geocoding API.
//
// Resolution happens exactly once, when a city is added. The resulting
// latitude, longitude, and IANA timezone are stored in config forever,
// so refreshes never re-geocode: the ambiguity this package exists to
// resolve stays resolved, and each refresh costs one request fewer.
package geocode

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/tonisoueid/360WeatherDashTUI/internal/weather"
)

// DefaultBaseURL is the Open-Meteo geocoding search endpoint.
const DefaultBaseURL = "https://geocoding-api.open-meteo.com/v1/search"

// maxResults is how many candidates the picker shows. Ten is enough to
// surface the intended Paris without turning the modal into a list to
// scroll through.
const maxResults = 10

// Place is one geocoding candidate.
type Place struct {
	ID        int     `json:"id"`
	Name      string  `json:"name"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Country   string  `json:"country"`
	CC        string  `json:"country_code"`
	Admin1    string  `json:"admin1"`
	Timezone  string  `json:"timezone"`
}

// Label renders the candidate for the picker. It includes the
// administrative region because disambiguation is the entire purpose of
// this list — there are around thirty places named Paris.
func (p Place) Label() string {
	parts := make([]string, 0, 3)
	for _, s := range []string{p.Name, p.Admin1, p.Country} {
		if s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, " · ")
}

// ToCity converts a candidate into a persistable city.
//
// The ID is derived from the geocoding database's own ID, which is
// stable across sessions. That stability is what lets the cache key on
// city ID and survive restarts.
func (p Place) ToCity() weather.City {
	return weather.City{
		ID:      "om-" + strconv.Itoa(p.ID),
		Name:    p.Name,
		Admin:   p.Admin1,
		Country: p.Country,
		CC:      p.CC,
		Lat:     p.Latitude,
		Lon:     p.Longitude,
		TZ:      p.Timezone,
	}
}

// Client searches the Open-Meteo geocoding API.
type Client struct {
	BaseURL string
	HTTP    *http.Client
}

// NewClient returns a geocoding client with a short timeout — search is
// interactive, so a slow response is worse than no response.
func NewClient() *Client {
	return &Client{
		BaseURL: DefaultBaseURL,
		HTTP:    &http.Client{Timeout: 8 * time.Second},
	}
}

type searchResponse struct {
	Results []Place `json:"results"`
}

// Search returns candidate places for a query. A blank query returns
// nothing without touching the network, which matters because the
// caller debounces keystrokes and will hit this on every backspace.
func (c *Client) Search(ctx context.Context, query string) ([]Place, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}

	q := url.Values{}
	q.Set("name", query)
	q.Set("count", strconv.Itoa(maxResults))
	q.Set("language", "en")
	q.Set("format", "json")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"?"+q.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("build geocode request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("geocode search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("geocoding API returned %s", resp.Status)
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read geocode response: %w", err)
	}

	// The API omits "results" entirely when nothing matches, which
	// decodes to a nil slice — an empty result, not an error.
	var out searchResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("decode geocode response: %w", err)
	}
	return out.Results, nil
}
