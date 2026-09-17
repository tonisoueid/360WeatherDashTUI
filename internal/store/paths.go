// Package store persists the city list (TOML config) and the forecast
// cache (JSON). Every write goes through WriteAtomic so that an
// interrupted save can never destroy a user's saved cities.
package store

import (
	"fmt"
	"os"
	"path/filepath"
)

// appDir is the subdirectory used under the XDG config and cache roots.
const appDir = "weatherdash"

// ConfigPath returns ~/.config/weatherdash/config.toml (or the platform
// equivalent). It does not create anything.
func ConfigPath() (string, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locate user config dir: %w", err)
	}
	return filepath.Join(root, appDir, "config.toml"), nil
}

// CachePath returns ~/.cache/weatherdash/cache.json (or the platform
// equivalent). It does not create anything.
func CachePath() (string, error) {
	root, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("locate user cache dir: %w", err)
	}
	return filepath.Join(root, appDir, "cache.json"), nil
}
