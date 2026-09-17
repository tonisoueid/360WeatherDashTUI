// Command weatherdash is a terminal dashboard for comparing weather
// across a saved list of cities.
package main

import (
	"flag"
	"fmt"
	"os"

	// Embed the timezone database so per-city local time is correct
	// even on minimal systems with no tzdata installed.
	_ "time/tzdata"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/tsoueid/weatherdash/internal/geocode"
	"github.com/tsoueid/weatherdash/internal/store"
	"github.com/tsoueid/weatherdash/internal/ui"
	"github.com/tsoueid/weatherdash/internal/version"
	"github.com/tsoueid/weatherdash/internal/weather"
)

func main() {
	demo := flag.Bool("demo", false, "run with synthetic data and no network")
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("weatherdash %s\n", version.Version)
		return
	}

	model, err := buildModel(*demo)
	if err != nil {
		// This is the ONLY place the program exits on error. Once the
		// TUI is running, every failure is surfaced in the footer.
		fmt.Fprintf(os.Stderr, "weatherdash: %v\n", err)
		os.Exit(1)
	}

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "weatherdash: %v\n", err)
		os.Exit(1)
	}
}

// buildModel loads config and cache, or synthesises them in demo mode.
func buildModel(demo bool) (ui.Model, error) {
	if demo {
		return ui.DemoModel(), nil
	}

	cfgPath, err := store.ConfigPath()
	if err != nil {
		return ui.Model{}, err
	}
	cfg, rec, err := store.LoadConfig(cfgPath)
	if err != nil {
		return ui.Model{}, err
	}

	cachePath, err := store.CachePath()
	if err != nil {
		return ui.Model{}, err
	}

	// LoadCache never fails: a missing or corrupt cache is an empty
	// one, and the first fetch repopulates it.
	model := ui.New(cfg, store.LoadCache(cachePath)).WithDeps(ui.Deps{
		Provider:   weather.NewClient(cfg.ForecastDays),
		Geocoder:   geocode.NewClient(),
		ConfigPath: cfgPath,
		CachePath:  cachePath,
	})

	if rec.Recovered {
		msg := "config was unreadable and has been reset"
		if rec.BackupPath != "" {
			msg += "; the old file is at " + rec.BackupPath
		}
		model = model.WithNotice(msg)
	}
	return model, nil
}
