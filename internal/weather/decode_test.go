package weather

import (
	"os"
	"testing"
	"time"
)

func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return raw
}

func TestDecodeLocationsArray(t *testing.T) {
	locs, err := decodeLocations(loadFixture(t, "two_cities.json"))
	if err != nil {
		t.Fatalf("decodeLocations: %v", err)
	}
	if len(locs) != 2 {
		t.Fatalf("got %d locations, want 2", len(locs))
	}
	if locs[1].Timezone != "Asia/Tokyo" {
		t.Errorf("locs[1].Timezone = %q, want Asia/Tokyo", locs[1].Timezone)
	}
}

// Open-Meteo returns a bare object for a single coordinate. Code written
// only against the array shape works until a user deletes down to one
// city, so this case is tested explicitly rather than assumed.
func TestDecodeLocationsSingleObject(t *testing.T) {
	locs, err := decodeLocations(loadFixture(t, "one_city.json"))
	if err != nil {
		t.Fatalf("decodeLocations on a bare object: %v", err)
	}
	if len(locs) != 1 {
		t.Fatalf("got %d locations, want 1", len(locs))
	}
	if locs[0].Timezone != "Europe/Paris" {
		t.Errorf("Timezone = %q, want Europe/Paris", locs[0].Timezone)
	}
}

func TestDecodeLocationsRejectsGarbage(t *testing.T) {
	if _, err := decodeLocations([]byte("{{{ truncated")); err == nil {
		t.Error("malformed JSON must return an error")
	}
	if _, err := decodeLocations([]byte("   ")); err == nil {
		t.Error("empty body must return an error")
	}
}

func TestToForecastMapsDailyFields(t *testing.T) {
	locs, _ := decodeLocations(loadFixture(t, "two_cities.json"))
	city := City{ID: "paris", Name: "Paris", TZ: "Europe/Paris"}

	f, err := locs[0].toForecast(city)
	if err != nil {
		t.Fatalf("toForecast: %v", err)
	}
	if f.CityID != "paris" {
		t.Errorf("CityID = %q, want paris", f.CityID)
	}
	if len(f.Days) != 2 {
		t.Fatalf("got %d days, want 2", len(f.Days))
	}

	d := f.Days[1]
	if d.HiC != 15 || d.LoC != 11 {
		t.Errorf("day 1 hi/lo = %v/%v, want 15/11", d.HiC, d.LoC)
	}
	if d.Code != 63 {
		t.Errorf("day 1 Code = %d, want 63", d.Code)
	}
	if d.PrecipMM != 12 {
		t.Errorf("day 1 PrecipMM = %v, want 12", d.PrecipMM)
	}
	if d.PrecipProb != 90 {
		t.Errorf("day 1 PrecipProb = %d, want 90", d.PrecipProb)
	}
	if d.UVMax != 2 {
		t.Errorf("day 1 UVMax = %v, want 2", d.UVMax)
	}
	if h, m := d.Sunrise.Hour(), d.Sunrise.Minute(); h != 7 || m != 14 {
		t.Errorf("day 1 Sunrise = %02d:%02d, want 07:14", h, m)
	}
}

// Hourly points must land in the day matching their LOCAL calendar date.
func TestToForecastGroupsHoursByLocalDay(t *testing.T) {
	locs, _ := decodeLocations(loadFixture(t, "two_cities.json"))
	f, err := locs[0].toForecast(City{ID: "paris", TZ: "Europe/Paris"})
	if err != nil {
		t.Fatal(err)
	}

	if got := len(f.Days[0].Hours); got != 2 {
		t.Errorf("day 0 has %d hours, want 2", got)
	}
	if got := len(f.Days[1].Hours); got != 2 {
		t.Errorf("day 1 has %d hours, want 2", got)
	}
	if got := f.Days[1].Hours[0].TempC; got != 13 {
		t.Errorf("day 1 hour 0 TempC = %v, want 13", got)
	}
	if got := f.Days[0].Hours[0].Time.Hour(); got != 0 {
		t.Errorf("day 0 hour 0 local hour = %d, want 0", got)
	}
}

// A null in any series must become a zero value, never a panic.
func TestToForecastHandlesNulls(t *testing.T) {
	locs, _ := decodeLocations(loadFixture(t, "two_cities.json"))
	f, err := locs[1].toForecast(City{ID: "tokyo", TZ: "Asia/Tokyo"})
	if err != nil {
		t.Fatalf("nulls in the hourly series must not error: %v", err)
	}
	h := f.Days[0].Hours[1]
	if h.PrecipProb != 0 || h.PrecipMM != 0 {
		t.Errorf("null hour = prob %d / mm %v, want zeroes", h.PrecipProb, h.PrecipMM)
	}
}

func TestToForecastMapsCurrent(t *testing.T) {
	locs, _ := decodeLocations(loadFixture(t, "two_cities.json"))
	f, _ := locs[0].toForecast(City{ID: "paris", TZ: "Europe/Paris"})

	if f.Current.TempC != 18.4 {
		t.Errorf("Current.TempC = %v, want 18.4", f.Current.TempC)
	}
	if f.Current.WindDir != 315 {
		t.Errorf("Current.WindDir = %d, want 315", f.Current.WindDir)
	}
	if f.Current.Code != 3 {
		t.Errorf("Current.Code = %d, want 3", f.Current.Code)
	}
	if f.FetchedAt.IsZero() {
		t.Error("FetchedAt must be stamped")
	}
}

func TestResolveLocationFallsBackToFixedOffset(t *testing.T) {
	loc := resolveLocation("Not/ARealZone", 3600)
	if loc == nil {
		t.Fatal("resolveLocation must never return nil")
	}
	_, offset := time.Now().In(loc).Zone()
	if offset != 3600 {
		t.Errorf("fallback offset = %d, want 3600", offset)
	}
}
