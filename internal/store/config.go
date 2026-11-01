package store

import (
	"bytes"
	"fmt"
	"os"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/tonisoueid/360WeatherDashTUI/internal/weather"
)

// Forecast window bounds. Open-Meteo supports up to 16 days; fewer than
// 3 makes the matrix pointless.
const (
	MinForecastDays = 3
	MaxForecastDays = 16
)

// Config is the on-disk user configuration.
type Config struct {
	Units          string         `toml:"units"`
	RefreshMinutes int            `toml:"refresh_minutes"`
	Density        string         `toml:"density"`
	Icons          string         `toml:"icons"`
	Sort           string         `toml:"sort"`
	ForecastDays   int            `toml:"forecast_days"`
	Cities         []weather.City `toml:"city"`
}

// Recovery reports that a corrupt config was salvaged rather than
// honoured. The UI surfaces this so a user whose city list vanished
// learns why, and where the old file went.
type Recovery struct {
	Recovered  bool
	BackupPath string
	Reason     string
}

// DefaultConfig is the configuration for a first run: no cities, metric
// units, a quarter-hour refresh.
func DefaultConfig() Config {
	return Config{
		Units:          "metric",
		RefreshMinutes: 15,
		Density:        "compact",
		Icons:          "unicode",
		Sort:           "manual",
		ForecastDays:   7,
		Cities:         nil,
	}
}

// LoadConfig reads the config at path.
//
// It never returns an error for a missing or corrupt file. A missing
// file is a first run and yields defaults. A corrupt file is backed up
// next to the original and reported via Recovery, because silently
// discarding twenty saved cities is the worst possible outcome.
func LoadConfig(path string) (Config, Recovery, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return DefaultConfig(), Recovery{}, nil
	}
	if err != nil {
		return DefaultConfig(), Recovery{}, fmt.Errorf("read config: %w", err)
	}

	// Decode onto the defaults so a hand-edited file that sets only one
	// key keeps sensible values everywhere else.
	cfg := DefaultConfig()
	if _, err := toml.Decode(string(raw), &cfg); err != nil {
		backup := path + ".bak"
		// Best effort: if the backup cannot be written we still recover,
		// we just cannot point the user at the old file.
		if writeErr := WriteAtomic(backup, raw, 0o600); writeErr != nil {
			backup = ""
		}
		return DefaultConfig(), Recovery{
			Recovered:  true,
			BackupPath: backup,
			Reason:     err.Error(),
		}, nil
	}

	cfg.normalise()
	return cfg, Recovery{}, nil
}

// normalise clamps out-of-range values rather than rejecting the file.
// A hand-edited config with forecast_days = 99 should work, not fail.
func (c *Config) normalise() {
	if c.ForecastDays < MinForecastDays {
		c.ForecastDays = MinForecastDays
	}
	if c.ForecastDays > MaxForecastDays {
		c.ForecastDays = MaxForecastDays
	}
	if c.RefreshMinutes < 1 {
		c.RefreshMinutes = 1
	}
}

// RefreshInterval is the auto-refresh period as a duration.
func (c Config) RefreshInterval() time.Duration {
	return time.Duration(c.RefreshMinutes) * time.Minute
}

// SaveConfig writes the config atomically.
func SaveConfig(path string, c Config) error {
	var buf bytes.Buffer
	buf.WriteString("# weatherdash configuration\n")
	buf.WriteString("# Edit by hand freely; unknown or out-of-range values are clamped.\n\n")
	if err := toml.NewEncoder(&buf).Encode(c); err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	return WriteAtomic(path, buf.Bytes(), 0o600)
}
