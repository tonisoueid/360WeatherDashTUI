package weather

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Open-Meteo emits local timestamps without an offset suffix when a
// timezone is requested, so they are parsed in the resolved location.
const (
	layoutDateTime = "2006-01-02T15:04"
	layoutDate     = "2006-01-02"
)

type rawCurrent struct {
	Time                string   `json:"time"`
	Temperature2m       *float64 `json:"temperature_2m"`
	ApparentTemperature *float64 `json:"apparent_temperature"`
	WeatherCode         *int     `json:"weather_code"`
	WindSpeed10m        *float64 `json:"wind_speed_10m"`
	WindDirection10m    *int     `json:"wind_direction_10m"`
}

type rawHourly struct {
	Time                     []string   `json:"time"`
	Temperature2m            []*float64 `json:"temperature_2m"`
	ApparentTemperature      []*float64 `json:"apparent_temperature"`
	PrecipitationProbability []*int     `json:"precipitation_probability"`
	Precipitation            []*float64 `json:"precipitation"`
	WeatherCode              []*int     `json:"weather_code"`
	WindSpeed10m             []*float64 `json:"wind_speed_10m"`
	WindDirection10m         []*int     `json:"wind_direction_10m"`
}

type rawDaily struct {
	Time                        []string   `json:"time"`
	WeatherCode                 []*int     `json:"weather_code"`
	Temperature2mMax            []*float64 `json:"temperature_2m_max"`
	Temperature2mMin            []*float64 `json:"temperature_2m_min"`
	PrecipitationSum            []*float64 `json:"precipitation_sum"`
	PrecipitationProbabilityMax []*int     `json:"precipitation_probability_max"`
	WindSpeed10mMax             []*float64 `json:"wind_speed_10m_max"`
	UVIndexMax                  []*float64 `json:"uv_index_max"`
	Sunrise                     []string   `json:"sunrise"`
	Sunset                      []string   `json:"sunset"`
}

type rawLocation struct {
	Latitude         float64    `json:"latitude"`
	Longitude        float64    `json:"longitude"`
	Timezone         string     `json:"timezone"`
	UTCOffsetSeconds int        `json:"utc_offset_seconds"`
	Current          rawCurrent `json:"current"`
	Hourly           rawHourly  `json:"hourly"`
	Daily            rawDaily   `json:"daily"`
}

// decodeLocations normalises the two response shapes Open-Meteo uses.
//
// A request with several coordinates returns a JSON array; a request
// with exactly one returns a bare object. Handling only the array shape
// produces code that works until a user deletes down to a single city,
// which is why the shape is sniffed rather than assumed.
func decodeLocations(raw []byte) ([]rawLocation, error) {
	trimmed := bytes.TrimLeft(raw, " \t\r\n")
	if len(trimmed) == 0 {
		return nil, errors.New("empty response body")
	}

	if trimmed[0] == '[' {
		var locs []rawLocation
		if err := json.Unmarshal(trimmed, &locs); err != nil {
			return nil, fmt.Errorf("decode location array: %w", err)
		}
		return locs, nil
	}

	var one rawLocation
	if err := json.Unmarshal(trimmed, &one); err != nil {
		return nil, fmt.Errorf("decode single location: %w", err)
	}
	return []rawLocation{one}, nil
}

// resolveLocation turns the API's timezone name into a *time.Location.
//
// LoadLocation is tried first because it is correct across daylight
// saving transitions inside the forecast window. It needs the tz
// database, which main.go embeds via the time/tzdata import; if it is
// somehow unavailable, a fixed offset is still far better than UTC.
// This function never returns nil.
func resolveLocation(tz string, offsetSec int) *time.Location {
	if tz != "" {
		if loc, err := time.LoadLocation(tz); err == nil {
			return loc
		}
	}
	name := tz
	if name == "" {
		name = "local"
	}
	return time.FixedZone(name, offsetSec)
}

// Null-safe accessors. Open-Meteo emits null for individual points when
// a model has no value, so every series read goes through these.
func fAt(s []*float64, i int) float64 {
	if i < 0 || i >= len(s) || s[i] == nil {
		return 0
	}
	return *s[i]
}

func iAt(s []*int, i int) int {
	if i < 0 || i >= len(s) || s[i] == nil {
		return 0
	}
	return *s[i]
}

func fVal(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}

func iVal(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

func parseAt(s []string, i int, layout string, loc *time.Location) time.Time {
	if i < 0 || i >= len(s) {
		return time.Time{}
	}
	t, err := time.ParseInLocation(layout, s[i], loc)
	if err != nil {
		return time.Time{}
	}
	return t
}

// toForecast maps one decoded location onto the domain type, resolving
// all timestamps in the city's local zone.
//
// Hourly points are bucketed by their LOCAL calendar date, which is the
// semantics the matrix needs: the user asks "what is Friday like there",
// not "what happens during Friday UTC".
func (r rawLocation) toForecast(city City) (Forecast, error) {
	loc := resolveLocation(r.Timezone, r.UTCOffsetSeconds)

	if len(r.Daily.Time) == 0 {
		return Forecast{}, fmt.Errorf("no daily data for %s", city.Name)
	}

	// Index days by their date string so hours can be attached in one pass.
	days := make([]Day, len(r.Daily.Time))
	index := make(map[string]int, len(r.Daily.Time))
	for i, ds := range r.Daily.Time {
		days[i] = Day{
			Date:       parseAt(r.Daily.Time, i, layoutDate, loc),
			Code:       iAt(r.Daily.WeatherCode, i),
			HiC:        fAt(r.Daily.Temperature2mMax, i),
			LoC:        fAt(r.Daily.Temperature2mMin, i),
			PrecipMM:   fAt(r.Daily.PrecipitationSum, i),
			PrecipProb: iAt(r.Daily.PrecipitationProbabilityMax, i),
			WindMaxKPH: fAt(r.Daily.WindSpeed10mMax, i),
			UVMax:      fAt(r.Daily.UVIndexMax, i),
			Sunrise:    parseAt(r.Daily.Sunrise, i, layoutDateTime, loc),
			Sunset:     parseAt(r.Daily.Sunset, i, layoutDateTime, loc),
		}
		index[ds] = i
	}

	for i, ts := range r.Hourly.Time {
		t, err := time.ParseInLocation(layoutDateTime, ts, loc)
		if err != nil {
			continue // a single unparseable stamp must not lose the city
		}
		di, ok := index[t.Format(layoutDate)]
		if !ok {
			continue // hourly extends past the daily window
		}
		days[di].Hours = append(days[di].Hours, Hour{
			Time:       t,
			TempC:      fAt(r.Hourly.Temperature2m, i),
			ApparentC:  fAt(r.Hourly.ApparentTemperature, i),
			PrecipMM:   fAt(r.Hourly.Precipitation, i),
			WindKPH:    fAt(r.Hourly.WindSpeed10m, i),
			PrecipProb: iAt(r.Hourly.PrecipitationProbability, i),
			WindDir:    iAt(r.Hourly.WindDirection10m, i),
			Code:       iAt(r.Hourly.WeatherCode, i),
		})
	}

	current := Hour{
		TempC:     fVal(r.Current.Temperature2m),
		ApparentC: fVal(r.Current.ApparentTemperature),
		WindKPH:   fVal(r.Current.WindSpeed10m),
		WindDir:   iVal(r.Current.WindDirection10m),
		Code:      iVal(r.Current.WeatherCode),
	}
	if t, err := time.ParseInLocation(layoutDateTime, r.Current.Time, loc); err == nil {
		current.Time = t
	}

	return Forecast{
		CityID:    city.ID,
		Current:   current,
		Days:      days,
		FetchedAt: time.Now().UTC(),
	}, nil
}
