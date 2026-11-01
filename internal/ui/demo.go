package ui

import (
	"math"
	"time"

	"github.com/tsoueid/360WeatherDashTUI/internal/store"
	"github.com/tsoueid/360WeatherDashTUI/internal/weather"
)

// demoCity is one synthetic location plus the shape of its weather.
//
// lat/lon are real coordinates, not synthetic like the rest of the
// data: the world map screen plots demo cities exactly as it would
// plot real ones, so a reviewer sees Tokyo over Japan rather than
// every city stacked on Null Island.
type demoCity struct {
	name, cc, country, tz string
	lat, lon              float64
	baseTemp              float64
	code                  int
}

// DemoModel returns a model populated with synthetic data.
//
// It exists so the UI can be developed and reviewed before any network
// code is written, and so a reviewer can see the grid without an
// internet connection. It is wired to the --demo flag, never to the
// default path.
func DemoModel() Model {
	demo := []demoCity{
		{"Paris", "FR", "France", "Europe/Paris", 48.8566, 2.3522, 16, 3},
		{"Beirut", "LB", "Lebanon", "Asia/Beirut", 33.8938, 35.5018, 27, 0},
		{"Tokyo", "JP", "Japan", "Asia/Tokyo", 35.6762, 139.6503, 12, 61},
		{"Oslo", "NO", "Norway", "Europe/Oslo", 59.9139, 10.7522, 4, 71},
		{"Lisbon", "PT", "Portugal", "Europe/Lisbon", 38.7223, -9.1393, 23, 0},
		{"Berlin", "DE", "Germany", "Europe/Berlin", 52.5200, 13.4050, 15, 3},
		{"Cairo", "EG", "Egypt", "Africa/Cairo", 30.0444, 31.2357, 31, 0},
		{"Reykjavík", "IS", "Iceland", "Atlantic/Reykjavik", 64.1466, -21.9426, 6, 51},
	}

	cfg := store.DefaultConfig()
	cache := store.NewCache()
	start := time.Now().Truncate(24 * time.Hour)

	for i, d := range demo {
		city := weather.City{
			ID: "demo-" + d.name, Name: d.name, CC: d.cc,
			Country: d.country, TZ: d.tz, Lat: d.lat, Lon: d.lon,
		}
		cfg.Cities = append(cfg.Cities, city)

		days := make([]weather.Day, cfg.ForecastDays)
		for n := range days {
			// A gentle sine wave gives the sparklines a believable
			// shape instead of a straight line.
			wobble := math.Sin(float64(n+i) * 0.8)
			hi := d.baseTemp + wobble*4
			lo := hi - 6

			hours := make([]weather.Hour, 24)
			for h := range hours {
				diurnal := math.Sin(float64(h-6) / 24 * 2 * math.Pi)
				hours[h] = weather.Hour{
					Time:       start.AddDate(0, 0, n).Add(time.Duration(h) * time.Hour),
					TempC:      lo + (hi-lo)*(diurnal+1)/2,
					ApparentC:  lo + (hi-lo)*(diurnal+1)/2 - 1.5,
					PrecipProb: int(math.Abs(wobble) * 90),
					PrecipMM:   math.Abs(wobble) * 2,
					WindKPH:    8 + math.Abs(wobble)*20,
					WindDir:    (h*15 + i*30) % 360,
					Code:       d.code,
				}
			}

			date := start.AddDate(0, 0, n)
			days[n] = weather.Day{
				Date: date, Code: d.code, HiC: hi, LoC: lo,
				PrecipMM:   math.Abs(wobble) * 12,
				PrecipProb: int(math.Abs(wobble) * 95),
				WindMaxKPH: 12 + math.Abs(wobble)*20,
				UVMax:      math.Max(0, 6-math.Abs(wobble)*4),
				Sunrise:    date.Add(7 * time.Hour),
				Sunset:     date.Add(19*time.Hour + 30*time.Minute),
				Hours:      hours,
			}
		}

		cache.Forecasts[city.ID] = weather.Forecast{
			CityID: city.ID, Current: days[0].Hours[12],
			Days: days, FetchedAt: time.Now().UTC(),
		}
	}

	m := New(cfg, cache)
	// The demo data was generated this instant; say so in the footer
	// instead of leaving the pre-first-fetch "no data yet" on screen.
	m.lastOK = time.Now()
	return m
}
