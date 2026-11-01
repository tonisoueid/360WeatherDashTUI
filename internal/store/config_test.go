package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tsoueid/360WeatherDashTUI/internal/weather"
)

func TestDefaultConfigIsValid(t *testing.T) {
	c := DefaultConfig()
	if c.Units != "metric" {
		t.Errorf("Units = %q, want metric", c.Units)
	}
	if c.RefreshMinutes != 15 {
		t.Errorf("RefreshMinutes = %d, want 15", c.RefreshMinutes)
	}
	if c.ForecastDays != 7 {
		t.Errorf("ForecastDays = %d, want 7", c.ForecastDays)
	}
	if len(c.Cities) != 0 {
		t.Errorf("Cities = %d, want empty on first run", len(c.Cities))
	}
}

func TestConfigRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")

	want := DefaultConfig()
	want.Units = "imperial"
	want.Cities = []weather.City{{
		ID: "01J", Name: "Paris", Admin: "Île-de-France",
		Country: "France", CC: "FR",
		Lat: 48.8566, Lon: 2.3522, TZ: "Europe/Paris",
	}}

	if err := SaveConfig(path, want); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	got, rec, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if rec.Recovered {
		t.Error("clean config should not report recovery")
	}
	if got.Units != "imperial" {
		t.Errorf("Units = %q, want imperial", got.Units)
	}
	if len(got.Cities) != 1 || got.Cities[0].Name != "Paris" {
		t.Fatalf("Cities = %+v, want one Paris", got.Cities)
	}
	if got.Cities[0].TZ != "Europe/Paris" {
		t.Errorf("TZ = %q, want Europe/Paris", got.Cities[0].TZ)
	}
}

func TestLoadConfigMissingReturnsDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absent.toml")

	got, rec, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("missing config must not error, got %v", err)
	}
	if rec.Recovered {
		t.Error("a missing file is not a recovery, it is a first run")
	}
	if got.RefreshMinutes != 15 {
		t.Errorf("want defaults, got %+v", got)
	}
}

func TestLoadConfigCorruptBacksUpAndRecovers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("this is not [valid toml"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, rec, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("corrupt config must not error, got %v", err)
	}
	if !rec.Recovered {
		t.Fatal("corrupt config must report Recovered")
	}
	if rec.BackupPath == "" {
		t.Fatal("recovery must name the backup path")
	}
	if _, err := os.Stat(rec.BackupPath); err != nil {
		t.Errorf("backup file must exist at %s: %v", rec.BackupPath, err)
	}
	if got.RefreshMinutes != 15 {
		t.Errorf("recovered config must be the defaults, got %+v", got)
	}
}

func TestLoadConfigAppliesDefaultsToMissingFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	// A hand-edited file that only sets one field.
	if err := os.WriteFile(path, []byte("units = \"imperial\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, _, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Units != "imperial" {
		t.Errorf("Units = %q, want imperial", got.Units)
	}
	if got.RefreshMinutes != 15 {
		t.Errorf("RefreshMinutes = %d, want the default 15", got.RefreshMinutes)
	}
	if got.ForecastDays != 7 {
		t.Errorf("ForecastDays = %d, want the default 7", got.ForecastDays)
	}
}

func TestLoadConfigClampsForecastDays(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("forecast_days = 99\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, _, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.ForecastDays != 16 {
		t.Errorf("ForecastDays = %d, want clamped to 16", got.ForecastDays)
	}
}
