package weather

import (
	"net/url"
	"strings"
	"testing"
)

func testCities() []City {
	return []City{
		{ID: "a", Name: "Paris", Lat: 48.8566, Lon: 2.3522, TZ: "Europe/Paris"},
		{ID: "b", Name: "Tokyo", Lat: 35.6762, Lon: 139.6503, TZ: "Asia/Tokyo"},
	}
}

func TestBuildURLBatchesCoordinates(t *testing.T) {
	c := NewClient(7)
	raw := c.buildURL(testCities())

	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("buildURL produced an unparseable URL: %v", err)
	}
	q := u.Query()

	if got, want := q.Get("latitude"), "48.8566,35.6762"; got != want {
		t.Errorf("latitude = %q, want %q", got, want)
	}
	if got, want := q.Get("longitude"), "2.3522,139.6503"; got != want {
		t.Errorf("longitude = %q, want %q", got, want)
	}
	if got, want := q.Get("timezone"), "auto"; got != want {
		t.Errorf("timezone = %q, want %q", got, want)
	}
	if got, want := q.Get("forecast_days"), "7"; got != want {
		t.Errorf("forecast_days = %q, want %q", got, want)
	}
}

// Every variable the UI renders must be requested, or it silently
// arrives as zero and the grid shows plausible-looking wrong data.
func TestBuildURLRequestsEveryRenderedVariable(t *testing.T) {
	c := NewClient(7)
	u, _ := url.Parse(c.buildURL(testCities()))
	q := u.Query()

	for _, want := range []string{
		"temperature_2m", "apparent_temperature", "weather_code",
		"wind_speed_10m", "wind_direction_10m",
	} {
		if !strings.Contains(q.Get("current"), want) {
			t.Errorf("current is missing %q (got %q)", want, q.Get("current"))
		}
	}
	for _, want := range []string{
		"temperature_2m", "precipitation_probability", "precipitation",
		"weather_code", "wind_speed_10m", "wind_direction_10m",
	} {
		if !strings.Contains(q.Get("hourly"), want) {
			t.Errorf("hourly is missing %q (got %q)", want, q.Get("hourly"))
		}
	}
	for _, want := range []string{
		"weather_code", "temperature_2m_max", "temperature_2m_min",
		"precipitation_sum", "precipitation_probability_max",
		"wind_speed_10m_max", "uv_index_max", "sunrise", "sunset",
	} {
		if !strings.Contains(q.Get("daily"), want) {
			t.Errorf("daily is missing %q (got %q)", want, q.Get("daily"))
		}
	}
}

func TestBuildURLSingleCity(t *testing.T) {
	c := NewClient(3)
	u, _ := url.Parse(c.buildURL(testCities()[:1]))
	q := u.Query()

	if got, want := q.Get("latitude"), "48.8566"; got != want {
		t.Errorf("latitude = %q, want %q (no trailing comma)", got, want)
	}
	if got, want := q.Get("forecast_days"), "3"; got != want {
		t.Errorf("forecast_days = %q, want %q", got, want)
	}
}

func TestNewClientClampsDays(t *testing.T) {
	if got := NewClient(0).Days; got != 1 {
		t.Errorf("NewClient(0).Days = %d, want clamped to 1", got)
	}
	if got := NewClient(99).Days; got != 16 {
		t.Errorf("NewClient(99).Days = %d, want clamped to 16", got)
	}
}
