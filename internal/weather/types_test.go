package weather

import (
	"math"
	"testing"
	"time"
)

func TestCityFull(t *testing.T) {
	c := City{Name: "Paris", Admin: "Île-de-France", Country: "France"}
	if got, want := c.Full(), "Paris · Île-de-France · France"; got != want {
		t.Errorf("Full() = %q, want %q", got, want)
	}
}

func TestCityFullOmitsEmptyAdmin(t *testing.T) {
	c := City{Name: "Singapore", Country: "Singapore"}
	if got, want := c.Full(), "Singapore · Singapore"; got != want {
		t.Errorf("Full() = %q, want %q", got, want)
	}
}

func TestForecastAggregates(t *testing.T) {
	f := Forecast{Days: []Day{
		{HiC: 10, PrecipMM: 1.5},
		{HiC: 20, PrecipMM: 2.5},
	}}
	if got := f.MeanHigh(); math.Abs(got-15) > 1e-9 {
		t.Errorf("MeanHigh() = %v, want 15", got)
	}
	if got := f.TotalPrecip(); math.Abs(got-4) > 1e-9 {
		t.Errorf("TotalPrecip() = %v, want 4", got)
	}
}

func TestForecastAggregatesEmpty(t *testing.T) {
	var f Forecast
	if got := f.MeanHigh(); got != 0 {
		t.Errorf("MeanHigh() on empty = %v, want 0", got)
	}
}

func TestDayHoursRoundTrip(t *testing.T) {
	d := Day{Date: time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC)}
	if d.Date.Day() != 19 {
		t.Errorf("Date.Day() = %d, want 19", d.Date.Day())
	}
}
