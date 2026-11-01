package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tonisoueid/360WeatherDashTUI/internal/weather"
)

func TestCacheRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.json")
	at := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)

	c := NewCache()
	c.Forecasts["paris"] = weather.Forecast{
		CityID:    "paris",
		FetchedAt: at,
		Days:      []weather.Day{{HiC: 18, LoC: 12, Code: 3}},
	}

	if err := SaveCache(path, c); err != nil {
		t.Fatalf("SaveCache: %v", err)
	}

	got := LoadCache(path)
	f, ok := got.Forecasts["paris"]
	if !ok {
		t.Fatalf("paris missing from loaded cache: %+v", got)
	}
	if !f.FetchedAt.Equal(at) {
		t.Errorf("FetchedAt = %v, want %v", f.FetchedAt, at)
	}
	if len(f.Days) != 1 || f.Days[0].HiC != 18 {
		t.Errorf("Days = %+v, want one day with HiC 18", f.Days)
	}
}

func TestLoadCacheMissingIsEmptyNotFatal(t *testing.T) {
	got := LoadCache(filepath.Join(t.TempDir(), "absent.json"))
	if got.Forecasts == nil {
		t.Fatal("Forecasts map must be non-nil so callers can write to it")
	}
	if len(got.Forecasts) != 0 {
		t.Errorf("want empty cache, got %d entries", len(got.Forecasts))
	}
}

func TestLoadCacheCorruptIsEmptyNotFatal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.json")
	if err := os.WriteFile(path, []byte("{{{ truncated"), 0o600); err != nil {
		t.Fatal(err)
	}

	got := LoadCache(path)
	if got.Forecasts == nil || len(got.Forecasts) != 0 {
		t.Errorf("corrupt cache must load as empty, got %+v", got)
	}
}

func TestLoadCacheWrongVersionIsDiscarded(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.json")
	body := `{"version":99,"forecasts":{"paris":{"city_id":"paris"}}}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	got := LoadCache(path)
	if len(got.Forecasts) != 0 {
		t.Errorf("a future cache version must be discarded, got %+v", got)
	}
}
