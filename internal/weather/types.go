// Package weather holds the domain types for forecast data and the
// Provider interface that fetches it.
//
// All values in this package are canonical metric: Celsius, km/h, mm.
// Conversion to other unit systems happens in internal/units at render
// time only.
package weather

import (
	"context"
	"strings"
	"time"
)

// City is a saved location. It is persisted in config (TOML) and
// embedded in the forecast cache (JSON), hence both sets of tags.
type City struct {
	ID      string  `toml:"id" json:"id"`
	Name    string  `toml:"name" json:"name"`
	Admin   string  `toml:"admin" json:"admin"`
	Country string  `toml:"country" json:"country"`
	CC      string  `toml:"cc" json:"cc"`
	Lat     float64 `toml:"lat" json:"lat"`
	Lon     float64 `toml:"lon" json:"lon"`
	TZ      string  `toml:"tz" json:"tz"`
}

// Full renders the disambiguated name used in search results and the
// detail header, skipping empty components.
func (c City) Full() string {
	parts := make([]string, 0, 3)
	for _, p := range []string{c.Name, c.Admin, c.Country} {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, " · ")
}

// Hour is a single hourly observation or forecast point, in the city's
// local time.
type Hour struct {
	Time       time.Time `json:"time"`
	TempC      float64   `json:"temp_c"`
	ApparentC  float64   `json:"apparent_c"`
	PrecipMM   float64   `json:"precip_mm"`
	WindKPH    float64   `json:"wind_kph"`
	PrecipProb int       `json:"precip_prob"`
	WindDir    int       `json:"wind_dir"`
	Code       int       `json:"code"`
}

// Day is one local calendar day. Aligning by the city's local calendar
// day (not UTC) is deliberate: the user asks "what is Friday like
// there", so buckets must match local days.
type Day struct {
	Date       time.Time `json:"date"`
	Code       int       `json:"code"`
	HiC        float64   `json:"hi_c"`
	LoC        float64   `json:"lo_c"`
	PrecipMM   float64   `json:"precip_mm"`
	PrecipProb int       `json:"precip_prob"`
	WindMaxKPH float64   `json:"wind_max_kph"`
	UVMax      float64   `json:"uv_max"`
	Sunrise    time.Time `json:"sunrise"`
	Sunset     time.Time `json:"sunset"`
	Hours      []Hour    `json:"hours"`
}

// Forecast is the complete fetched result for one city.
type Forecast struct {
	CityID    string    `json:"city_id"`
	Current   Hour      `json:"current"`
	Days      []Day     `json:"days"`
	FetchedAt time.Time `json:"fetched_at"`
}

// MeanHigh is the average daily high across the forecast window. It is
// the ranking key for the warmest and coldest sort modes.
func (f Forecast) MeanHigh() float64 {
	if len(f.Days) == 0 {
		return 0
	}
	var sum float64
	for _, d := range f.Days {
		sum += d.HiC
	}
	return sum / float64(len(f.Days))
}

// TotalPrecip is summed precipitation across the forecast window. It is
// the ranking key for the driest sort mode.
func (f Forecast) TotalPrecip() float64 {
	var sum float64
	for _, d := range f.Days {
		sum += d.PrecipMM
	}
	return sum
}

// Provider fetches forecasts for every city in one call. The batched
// shape is deliberate: Open-Meteo accepts comma-separated coordinates,
// so N cities cost one HTTP request, which removes all per-city
// in-flight tracking from the UI layer.
type Provider interface {
	FetchAll(ctx context.Context, cities []City) ([]Forecast, error)
}
