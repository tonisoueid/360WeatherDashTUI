package weather

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// DefaultBaseURL is the Open-Meteo forecast endpoint. It is a field on
// Client rather than a constant so tests can point at an httptest server.
const DefaultBaseURL = "https://api.open-meteo.com/v1/forecast"

// Variable lists. These are the single source of truth for what the API
// returns; every field the UI renders must appear here or it silently
// arrives as a zero value.
const (
	currentVars = "temperature_2m,apparent_temperature,weather_code," +
		"wind_speed_10m,wind_direction_10m,relative_humidity_2m"

	hourlyVars = "temperature_2m,apparent_temperature," +
		"precipitation_probability,precipitation,weather_code," +
		"wind_speed_10m,wind_direction_10m"

	dailyVars = "weather_code,temperature_2m_max,temperature_2m_min," +
		"precipitation_sum,precipitation_probability_max," +
		"wind_speed_10m_max,uv_index_max,sunrise,sunset"
)

// Client is a batched Open-Meteo forecast client. It implements Provider.
type Client struct {
	BaseURL string
	HTTP    *http.Client
	Days    int
}

// NewClient returns a client requesting the given forecast window,
// clamped to what the API supports.
func NewClient(days int) *Client {
	if days < 1 {
		days = 1
	}
	if days > 16 {
		days = 16
	}
	return &Client{
		BaseURL: DefaultBaseURL,
		HTTP:    &http.Client{Timeout: 15 * time.Second},
		Days:    days,
	}
}

// buildURL encodes every city into a single request. Open-Meteo accepts
// comma-separated coordinates and resolves timezone=auto per coordinate,
// so N cities cost exactly one HTTP request regardless of N.
//
// Coordinate order is the contract: the response is a positional list
// matching this input order. Callers must snapshot the city slice they
// pass here and map results back by index against that snapshot.
func (c *Client) buildURL(cities []City) string {
	lats := make([]string, len(cities))
	lons := make([]string, len(cities))
	for i, city := range cities {
		lats[i] = strconv.FormatFloat(city.Lat, 'f', -1, 64)
		lons[i] = strconv.FormatFloat(city.Lon, 'f', -1, 64)
	}

	q := url.Values{}
	q.Set("latitude", strings.Join(lats, ","))
	q.Set("longitude", strings.Join(lons, ","))
	q.Set("timezone", "auto")
	q.Set("forecast_days", strconv.Itoa(c.Days))
	q.Set("current", currentVars)
	q.Set("hourly", hourlyVars)
	q.Set("daily", dailyVars)

	return c.BaseURL + "?" + q.Encode()
}

// RateLimitError reports an HTTP 429. It carries Retry-After so the UI
// can back off for the stated interval instead of guessing.
type RateLimitError struct {
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("rate limited, retry after %s", e.RetryAfter)
}

// maxBody caps how much of a response we will read. Twenty cities of
// hourly data is well under a megabyte; anything past 8MB is a bug or a
// hostile response, and reading it would be a memory problem.
const maxBody = 8 << 20

// FetchAll retrieves forecasts for every city in a single HTTP request.
//
// The response is positional, matching the coordinate order in the
// request, so each result is stamped with the corresponding City.ID
// before returning. Everything downstream keys on that ID, which is why
// a concurrent add, delete, or reorder cannot misalign rows.
//
// An empty city list is a no-op, not an error: a first-run user has no
// cities, and firing a coordinate-less request would fail confusingly.
func (c *Client) FetchAll(ctx context.Context, cities []City) ([]Forecast, error) {
	if len(cities) == 0 {
		return nil, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.buildURL(cities), nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch forecasts: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, &RateLimitError{RetryAfter: retryAfter(resp)}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("forecast API returned %s", resp.Status)
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	locs, err := decodeLocations(raw)
	if err != nil {
		return nil, err
	}
	if len(locs) != len(cities) {
		return nil, fmt.Errorf(
			"forecast API returned %d locations for %d cities", len(locs), len(cities))
	}

	out := make([]Forecast, 0, len(cities))
	for i, loc := range locs {
		f, err := loc.toForecast(cities[i])
		if err != nil {
			return nil, fmt.Errorf("map %s: %w", cities[i].Name, err)
		}
		out = append(out, f)
	}
	return out, nil
}

// retryAfter reads the Retry-After header, falling back to a
// conservative default when it is absent or unparseable.
func retryAfter(resp *http.Response) time.Duration {
	const fallback = 60 * time.Second
	v := resp.Header.Get("Retry-After")
	if v == "" {
		return fallback
	}
	if secs, err := strconv.Atoi(v); err == nil && secs > 0 {
		return time.Duration(secs) * time.Second
	}
	return fallback
}

// Compile-time proof that Client satisfies Provider. Without this the
// mismatch would only surface when ui.New is first wired up.
var _ Provider = (*Client)(nil)
