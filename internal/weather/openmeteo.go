package weather

import (
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
