# weatherdash Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Bubble Tea TUI whose home screen is a cities × days weather matrix, where any cell zooms into that city's hourly detail for that day.

**Architecture:** Seven single-responsibility packages. `store`/`units` have no internal dependencies; `weather` holds domain types plus a **batched** Open-Meteo client that fetches all cities in one HTTP request; `widgets` are pure `func(data, width) string` renderers; `ui` holds the Bubble Tea model where a single shared `{city, day}` cursor drives both screens. Data is stored canonical-metric and converted only at render time.

**Tech Stack:** Go 1.26, `bubbletea`, `bubbles`, `lipgloss`, `BurntSushi/toml`, stdlib `net/http`/`encoding/json`. No weather client library.

**Spec:** `docs/superpowers/specs/2026-09-17-weatherdash-design.md`

## Global Constraints

Every task's requirements implicitly include this section.

- **Go 1.26.** Module path `github.com/tsoueid/weatherdash`.
- **Dependencies are limited to:** `github.com/charmbracelet/bubbletea`, `github.com/charmbracelet/bubbles`, `github.com/charmbracelet/lipgloss`, `github.com/BurntSushi/toml`, and `github.com/charmbracelet/x/exp/teatest` (test-only). Adding any other module requires an explicit decision, not a silent `go get`.
- **All stored values are canonical metric** — Celsius, km/h, mm. Conversion happens only in `internal/units` at render time. No unit-converted value is ever written to cache or config.
- **All terminal width measurement uses `lipgloss.Width()`, never `len()`.** Weather glyphs are double-width and city names are multi-byte; `len()` drifts borders.
- **Zero `log.Fatal` / `os.Exit` in `internal/ui` or below.** Only `cmd/weatherdash/main.go` may exit, and only for pre-TUI startup failures.
- **All config and cache writes are atomic** via `store.WriteAtomic` (temp file in the *target directory*, then rename).
- **Sorting is a view transform.** No sort operation may reorder `Config.Cities`.
- **XDG paths** via `os.UserConfigDir()` / `os.UserCacheDir()`, subdirectory `weatherdash`.
- **Every task ends with a commit.** Conventional-commit prefixes (`feat:`, `test:`, `fix:`, `chore:`).

## Phase Map

| Phase | Deliverable | Tasks |
|---|---|---|
| 0 | Repo scaffold that builds and tests | 1 |
| 1 | Domain types, units, persistence | 2-6 |
| 2 | Batched Open-Meteo + geocoding clients | 7-11 |
| 3 | Pure widget renderers | 12-15 |
| 4 | **Runnable grid app on fixture data** | 16-20 |
| 5 | Live data, cache-first startup | 21-23 |
| 6 | Detail screen | 24-25 |
| 7 | Add/delete cities | 26-28 |
| 8 | Sort, filter, density, units, help | 29-32 |
| 9 | Auto-refresh | 33 |
| 10 | Polish and golden tests | 34-36 |

**Phase 4 is the milestone that matters.** At the end of Task 20 the application runs, draws a real grid, and responds to keys — with zero network code written. Every later phase adds to something already visible.

---

# Phase 0 — Project Foundation

### Task 1: Repo scaffold

**Files:**
- Create: `go.mod`, `.gitignore`, `Makefile`, `cmd/weatherdash/main.go`, `internal/version/version.go`, `internal/version/version_test.go`

**Interfaces:**
- Consumes: nothing
- Produces: `version.Version` (string const), a `make test` target, a buildable binary

- [ ] **Step 1: Initialise the repository and module**

```bash
cd <repo root>
git init
go mod init github.com/tsoueid/weatherdash
```

- [ ] **Step 2: Add dependencies**

```bash
go get github.com/charmbracelet/bubbletea@latest
go get github.com/charmbracelet/bubbles@latest
go get github.com/charmbracelet/lipgloss@latest
go get github.com/BurntSushi/toml@latest
go get github.com/charmbracelet/x/exp/teatest@latest
```

- [ ] **Step 3: Write `.gitignore`**

```gitignore
/weatherdash
/dist/
*.out
.DS_Store
```

- [ ] **Step 4: Write the failing test**

Create `internal/version/version_test.go`:

```go
package version

import "testing"

func TestVersionIsSet(t *testing.T) {
	if Version == "" {
		t.Fatal("Version must not be empty")
	}
}
```

- [ ] **Step 5: Run it to confirm it fails**

Run: `go test ./internal/version/`
Expected: FAIL — `undefined: Version`

- [ ] **Step 6: Write the minimal implementation**

Create `internal/version/version.go`:

```go
// Package version records the build version of weatherdash.
package version

// Version is the semantic version of this build.
const Version = "0.1.0-dev"
```

- [ ] **Step 7: Run it to confirm it passes**

Run: `go test ./internal/version/`
Expected: PASS

- [ ] **Step 8: Write the placeholder entrypoint**

Create `cmd/weatherdash/main.go`:

```go
// Command weatherdash is a terminal dashboard for comparing weather
// across a saved list of cities.
package main

import (
	"fmt"

	"github.com/tsoueid/weatherdash/internal/version"
)

func main() {
	fmt.Printf("weatherdash %s\n", version.Version)
}
```

- [ ] **Step 9: Write the Makefile**

```makefile
.PHONY: build test lint run tidy

build:
	go build -o weatherdash ./cmd/weatherdash

test:
	go test ./... -race

lint:
	go vet ./...
	gofmt -l .

run: build
	./weatherdash

tidy:
	go mod tidy
```

- [ ] **Step 10: Verify the full toolchain works**

Run: `make build && make test && make lint && ./weatherdash`
Expected: binary builds, tests pass, `gofmt -l .` prints nothing, running prints `weatherdash 0.1.0-dev`

- [ ] **Step 11: Commit**

```bash
git add .
git commit -m "chore: scaffold go module, makefile, and version package"
```

---

# Phase 1 — Domain Types, Units, Persistence

This phase has no network and no UI. Everything here is testable in isolation, which is why it comes first.

### Task 2: Weather domain types

**Files:**
- Create: `internal/weather/types.go`, `internal/weather/types_test.go`

**Interfaces:**
- Consumes: nothing
- Produces: `weather.City`, `weather.Hour`, `weather.Day`, `weather.Forecast`, `weather.Provider`; methods `City.Full() string`, `Forecast.MeanHigh() float64`, `Forecast.TotalPrecip() float64`

> **Why struct tags for both TOML and JSON:** `City` is persisted in config (TOML) and embedded in cache (JSON). Carrying both tags on one type avoids a duplicate DTO and a mapping layer that would drift.

- [ ] **Step 1: Write the failing test**

Create `internal/weather/types_test.go`:

```go
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
```

- [ ] **Step 2: Run it to confirm it fails**

Run: `go test ./internal/weather/`
Expected: FAIL — `undefined: City`

- [ ] **Step 3: Write the implementation**

Create `internal/weather/types.go`:

```go
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
```

- [ ] **Step 4: Run it to confirm it passes**

Run: `go test ./internal/weather/ -v`
Expected: PASS, 5 tests

- [ ] **Step 5: Commit**

```bash
git add internal/weather/
git commit -m "feat: add weather domain types and Provider interface"
```

---

### Task 3: Units conversion and formatting

**Files:**
- Create: `internal/units/units.go`, `internal/units/units_test.go`

**Interfaces:**
- Consumes: nothing
- Produces: `units.System` (with `units.Metric`, `units.Imperial`), `units.Parse(string) (System, error)`, and methods `String()`, `Temp(c float64) string`, `TempNum(c float64) int`, `Speed(kph float64) string`, `Precip(mm float64) string`, `Toggle() System`

> **Why this package has no dependencies:** it is the only place that knows about non-metric units. Keeping it dependency-free means the `u` toggle can never require a refetch, and cached data can never be unit-tainted.

- [ ] **Step 1: Write the failing test**

Create `internal/units/units_test.go`:

```go
package units

import "testing"

func TestTempFormatting(t *testing.T) {
	tests := []struct {
		name string
		sys  System
		c    float64
		want string
	}{
		{"metric whole", Metric, 18, "18°"},
		{"metric rounds", Metric, 18.6, "19°"},
		{"metric negative", Metric, -3.2, "-3°"},
		{"imperial", Imperial, 18, "64°"},
		{"imperial freezing", Imperial, 0, "32°"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.sys.Temp(tc.c); got != tc.want {
				t.Errorf("Temp(%v) = %q, want %q", tc.c, got, tc.want)
			}
		})
	}
}

func TestSpeedFormatting(t *testing.T) {
	if got, want := Metric.Speed(12.4), "12 km/h"; got != want {
		t.Errorf("Metric.Speed = %q, want %q", got, want)
	}
	if got, want := Imperial.Speed(16.09), "10 mph"; got != want {
		t.Errorf("Imperial.Speed = %q, want %q", got, want)
	}
}

func TestPrecipFormatting(t *testing.T) {
	if got, want := Metric.Precip(12.4), "12mm"; got != want {
		t.Errorf("Metric.Precip = %q, want %q", got, want)
	}
	if got, want := Imperial.Precip(25.4), `1.0"`; got != want {
		t.Errorf("Imperial.Precip = %q, want %q", got, want)
	}
}

func TestParseRoundTrip(t *testing.T) {
	for _, s := range []System{Metric, Imperial} {
		got, err := Parse(s.String())
		if err != nil {
			t.Fatalf("Parse(%q) errored: %v", s.String(), err)
		}
		if got != s {
			t.Errorf("Parse(%q) = %v, want %v", s.String(), got, s)
		}
	}
}

func TestParseRejectsUnknown(t *testing.T) {
	if _, err := Parse("kelvin"); err == nil {
		t.Fatal("Parse(\"kelvin\") should return an error")
	}
}

func TestToggle(t *testing.T) {
	if Metric.Toggle() != Imperial || Imperial.Toggle() != Metric {
		t.Error("Toggle must swap between the two systems")
	}
}
```

- [ ] **Step 2: Run it to confirm it fails**

Run: `go test ./internal/units/`
Expected: FAIL — `undefined: System`

- [ ] **Step 3: Write the implementation**

Create `internal/units/units.go`:

```go
// Package units converts canonical metric values to a display system.
//
// Everything stored by weatherdash is metric. This package is the only
// place conversion happens, and it happens at render time, so toggling
// units never requires refetching or rewriting cached data.
package units

import (
	"fmt"
	"math"
)

// System is a unit display system.
type System int

const (
	// Metric renders Celsius, km/h, and millimetres.
	Metric System = iota
	// Imperial renders Fahrenheit, mph, and inches.
	Imperial
)

// String returns the config file spelling.
func (s System) String() string {
	if s == Imperial {
		return "imperial"
	}
	return "metric"
}

// Parse reads the config file spelling.
func Parse(s string) (System, error) {
	switch s {
	case "metric":
		return Metric, nil
	case "imperial":
		return Imperial, nil
	default:
		return Metric, fmt.Errorf("unknown unit system %q (want \"metric\" or \"imperial\")", s)
	}
}

// Toggle returns the other system.
func (s System) Toggle() System {
	if s == Metric {
		return Imperial
	}
	return Metric
}

// TempNum converts Celsius to the display system, rounded to a whole
// degree. Grid cells need the bare number for width budgeting.
func (s System) TempNum(c float64) int {
	if s == Imperial {
		return int(math.Round(c*9/5 + 32))
	}
	return int(math.Round(c))
}

// Temp renders a temperature with a degree sign and no unit letter —
// the letter is redundant in a dashboard where the system is global.
func (s System) Temp(c float64) string {
	return fmt.Sprintf("%d°", s.TempNum(c))
}

// Speed renders a wind speed with its unit.
func (s System) Speed(kph float64) string {
	if s == Imperial {
		return fmt.Sprintf("%d mph", int(math.Round(kph*0.621371)))
	}
	return fmt.Sprintf("%d km/h", int(math.Round(kph)))
}

// Precip renders a precipitation amount. Inches get one decimal because
// whole inches are too coarse to be useful.
func (s System) Precip(mm float64) string {
	if s == Imperial {
		return fmt.Sprintf("%.1f\"", mm/25.4)
	}
	return fmt.Sprintf("%dmm", int(math.Round(mm)))
}
```

- [ ] **Step 4: Run it to confirm it passes**

Run: `go test ./internal/units/ -v`
Expected: PASS, all subtests

- [ ] **Step 5: Commit**

```bash
git add internal/units/
git commit -m "feat: add units package for metric/imperial rendering"
```

---

### Task 4: XDG paths and atomic writes

**Files:**
- Create: `internal/store/paths.go`, `internal/store/atomic.go`, `internal/store/atomic_test.go`

**Interfaces:**
- Consumes: nothing
- Produces: `store.ConfigPath() (string, error)`, `store.CachePath() (string, error)`, `store.WriteAtomic(path string, data []byte, perm os.FileMode) error`

> **Why the temp file must live in the target directory:** `os.Rename` is only atomic within a filesystem. Creating the temp file in `/tmp` and renaming into `~/.config` fails with `invalid cross-device link` on any machine where those are separate mounts — which is common. This is the single most important detail in the task.

- [ ] **Step 1: Write the failing test**

Create `internal/store/atomic_test.go`:

```go
package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteAtomicCreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "out.txt")

	if err := WriteAtomic(path, []byte("hello"), 0o600); err != nil {
		t.Fatalf("WriteAtomic: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "hello" {
		t.Errorf("content = %q, want %q", got, "hello")
	}
}

func TestWriteAtomicOverwrites(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")

	if err := WriteAtomic(path, []byte("first"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteAtomic(path, []byte("second"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, _ := os.ReadFile(path)
	if string(got) != "second" {
		t.Errorf("content = %q, want %q", got, "second")
	}
}

// A failed write must leave the previous good file untouched, and must
// not leave temp files behind for the next reader to trip over.
func TestWriteAtomicLeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")

	if err := WriteAtomic(path, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("directory contains %v, want only out.txt", names)
	}
}

func TestWriteAtomicSetsPermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")

	if err := WriteAtomic(path, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("perm = %o, want 600", perm)
	}
}
```

- [ ] **Step 2: Run it to confirm it fails**

Run: `go test ./internal/store/`
Expected: FAIL — `undefined: WriteAtomic`

- [ ] **Step 3: Write the atomic writer**

Create `internal/store/atomic.go`:

```go
package store

import (
	"fmt"
	"os"
	"path/filepath"
)

// WriteAtomic writes data to path such that a crash mid-write can never
// leave a truncated or partial file in place. It writes to a temporary
// file in the SAME DIRECTORY as the target, fsyncs it, then renames.
//
// The same-directory requirement is not stylistic: os.Rename is only
// atomic within a single filesystem, so a temp file in /tmp would fail
// with EXDEV on any machine where /tmp is a separate mount.
func WriteAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file in %s: %w", dir, err)
	}
	tmpName := tmp.Name()

	// From here on every failure path must remove the temp file.
	cleanup := func() { _ = os.Remove(tmpName) }

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		cleanup()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		cleanup()
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		cleanup()
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		cleanup()
		return fmt.Errorf("rename into place: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Write the path resolver**

Create `internal/store/paths.go`:

```go
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
```

- [ ] **Step 5: Run it to confirm it passes**

Run: `go test ./internal/store/ -v -race`
Expected: PASS, 4 tests

- [ ] **Step 6: Commit**

```bash
git add internal/store/
git commit -m "feat: add XDG path resolution and atomic file writes"
```

---

### Task 5: Config load and save with corruption recovery

**Files:**
- Create: `internal/store/config.go`, `internal/store/config_test.go`

**Interfaces:**
- Consumes: `store.WriteAtomic` (Task 4), `weather.City` (Task 2)
- Produces: `store.Config` struct, `store.DefaultConfig() Config`, `store.LoadConfig(path string) (Config, Recovery, error)`, `store.SaveConfig(path string, c Config) error`, `store.Recovery` struct

> **Why `LoadConfig` returns a `Recovery` rather than an error on corruption:** a user with twenty saved cities and one stray character in their TOML must not lose the file or face a hard failure. The design backs the bad file up, starts fresh, and reports what happened so the UI can tell them — a silent reset would be worse than a crash.

- [ ] **Step 1: Write the failing test**

Create `internal/store/config_test.go`:

```go
package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tsoueid/weatherdash/internal/weather"
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
```

- [ ] **Step 2: Run it to confirm it fails**

Run: `go test ./internal/store/ -run TestConfig`
Expected: FAIL — `undefined: DefaultConfig`

- [ ] **Step 3: Write the implementation**

Create `internal/store/config.go`:

```go
package store

import (
	"bytes"
	"fmt"
	"os"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/tsoueid/weatherdash/internal/weather"
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
```

- [ ] **Step 4: Run it to confirm it passes**

Run: `go test ./internal/store/ -v -race`
Expected: PASS, 10 tests

- [ ] **Step 5: Commit**

```bash
git add internal/store/
git commit -m "feat: add TOML config with defaults and corruption recovery"
```

---

### Task 6: Forecast cache

**Files:**
- Create: `internal/store/cache.go`, `internal/store/cache_test.go`

**Interfaces:**
- Consumes: `store.WriteAtomic` (Task 4), `weather.Forecast` (Task 2)
- Produces: `store.Cache` struct with field `Forecasts map[string]weather.Forecast`, `store.LoadCache(path string) Cache`, `store.SaveCache(path string, c Cache) error`, `store.NewCache() Cache`

> **Why `LoadCache` returns no error at all:** the cache is an optimisation, never a source of truth. Any problem reading it — missing, truncated, wrong version, unreadable — has exactly one correct response: proceed with an empty cache. Returning an error would push a decision onto callers that has only one right answer.

- [ ] **Step 1: Write the failing test**

Create `internal/store/cache_test.go`:

```go
package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tsoueid/weatherdash/internal/weather"
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
```

- [ ] **Step 2: Run it to confirm it fails**

Run: `go test ./internal/store/ -run TestCache`
Expected: FAIL — `undefined: NewCache`

- [ ] **Step 3: Write the implementation**

Create `internal/store/cache.go`:

```go
package store

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/tsoueid/weatherdash/internal/weather"
)

// CacheVersion is bumped whenever the cached shape changes. A mismatch
// discards the cache rather than attempting migration — it is
// regenerated by the next successful fetch, so migration would be
// pure cost.
const CacheVersion = 1

// Cache is the last known good forecast for each city, keyed by city ID.
// Keying by ID rather than by index is what makes a reorder or delete
// between fetch and render harmless.
type Cache struct {
	Version   int                         `json:"version"`
	Forecasts map[string]weather.Forecast `json:"forecasts"`
}

// NewCache returns an empty, writable cache.
func NewCache() Cache {
	return Cache{
		Version:   CacheVersion,
		Forecasts: make(map[string]weather.Forecast),
	}
}

// LoadCache reads the cache at path. It deliberately returns no error:
// the cache is an optimisation, never a source of truth, and every
// possible failure has the same correct response — carry on with an
// empty cache and let the next fetch repopulate it.
func LoadCache(path string) Cache {
	raw, err := os.ReadFile(path)
	if err != nil {
		return NewCache()
	}

	var c Cache
	if err := json.Unmarshal(raw, &c); err != nil {
		return NewCache()
	}
	if c.Version != CacheVersion || c.Forecasts == nil {
		return NewCache()
	}
	return c
}

// SaveCache writes the cache atomically. Callers may ignore the error:
// a cache that fails to persist costs one slow startup, nothing more.
func SaveCache(path string, c Cache) error {
	c.Version = CacheVersion
	raw, err := json.Marshal(c)
	if err != nil {
		return fmt.Errorf("encode cache: %w", err)
	}
	return WriteAtomic(path, raw, 0o600)
}
```

- [ ] **Step 4: Run it to confirm it passes**

Run: `go test ./internal/store/ -v -race`
Expected: PASS, 14 tests

- [ ] **Step 5: Verify the whole phase**

Run: `make test && make lint`
Expected: all packages pass, `gofmt -l .` silent

- [ ] **Step 6: Commit**

```bash
git add internal/store/
git commit -m "feat: add forecast cache with version guard and safe fallbacks"
```

---

# Phase 2 — Weather and Geocoding Clients

### Task 7: WMO weather codes, labels, and icon sets

**Files:**
- Create: `internal/weather/wmo.go`, `internal/weather/wmo_test.go`

**Interfaces:**
- Consumes: nothing
- Produces: `weather.IconSet` (with `weather.IconsASCII`, `weather.IconsUnicode`, `weather.IconsNerd`), `weather.ParseIconSet(string) (IconSet, error)`, `weather.Icon(code int, set IconSet) string`, `weather.Label(code int) string`, `weather.IconWidth(set IconSet) int`

> **Why every icon must be exactly one cell wide:** the grid budgets `cellW` columns per cell, and the icon is part of that budget. If one glyph in one tier renders double-width, every row containing that condition shifts right and the borders visibly break. The test below asserts the width invariant across *every* code in *every* tier — that is the real protection, and it is why emoji are excluded from the cell tiers entirely.

- [ ] **Step 1: Write the failing test**

Create `internal/weather/wmo_test.go`:

```go
package weather

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// allCodes is every WMO code Open-Meteo can return.
var allCodes = []int{
	0, 1, 2, 3, 45, 48, 51, 53, 55, 56, 57,
	61, 63, 65, 66, 67, 71, 73, 75, 77,
	80, 81, 82, 85, 86, 95, 96, 99,
}

// The grid budgets a fixed number of columns per cell and the icon is
// part of that budget. A double-width glyph in any tier breaks every
// row containing that condition.
func TestIconWidthInvariant(t *testing.T) {
	for _, set := range []IconSet{IconsASCII, IconsUnicode, IconsNerd} {
		want := IconWidth(set)
		for _, code := range allCodes {
			got := lipgloss.Width(Icon(code, set))
			if got != want {
				t.Errorf("Icon(%d, %v) width = %d, want %d (glyph %q)",
					code, set, got, want, Icon(code, set))
			}
		}
	}
}

func TestIconUnknownCodeFallsBack(t *testing.T) {
	for _, set := range []IconSet{IconsASCII, IconsUnicode, IconsNerd} {
		if got := Icon(12345, set); got == "" {
			t.Errorf("Icon(12345, %v) must return a fallback glyph, got empty", set)
		}
		if got, want := lipgloss.Width(Icon(12345, set)), IconWidth(set); got != want {
			t.Errorf("fallback width = %d, want %d", got, want)
		}
	}
}

func TestLabels(t *testing.T) {
	tests := map[int]string{
		0:  "Clear sky",
		3:  "Overcast",
		65: "Heavy rain",
		95: "Thunderstorm",
	}
	for code, want := range tests {
		if got := Label(code); got != want {
			t.Errorf("Label(%d) = %q, want %q", code, got, want)
		}
	}
}

func TestLabelUnknownCode(t *testing.T) {
	if got := Label(12345); got != "Unknown" {
		t.Errorf("Label(12345) = %q, want Unknown", got)
	}
}

func TestParseIconSetRoundTrip(t *testing.T) {
	for _, s := range []IconSet{IconsASCII, IconsUnicode, IconsNerd} {
		got, err := ParseIconSet(s.String())
		if err != nil {
			t.Fatalf("ParseIconSet(%q): %v", s.String(), err)
		}
		if got != s {
			t.Errorf("ParseIconSet(%q) = %v, want %v", s.String(), got, s)
		}
	}
	if _, err := ParseIconSet("wingdings"); err == nil {
		t.Error("unknown icon set must error")
	}
}
```

- [ ] **Step 2: Run it to confirm it fails**

Run: `go test ./internal/weather/ -run TestIcon`
Expected: FAIL — `undefined: IconSet`

- [ ] **Step 3: Write the implementation**

Create `internal/weather/wmo.go`:

```go
package weather

import "fmt"

// IconSet selects the glyph vocabulary used for weather conditions.
type IconSet int

const (
	// IconsASCII is pure 7-bit ASCII. Works in every terminal and font.
	IconsASCII IconSet = iota
	// IconsUnicode uses BMP symbol glyphs chosen for narrow width.
	IconsUnicode
	// IconsNerd uses Nerd Font weather glyphs. Requires a patched font.
	IconsNerd
)

// String returns the config file spelling.
func (s IconSet) String() string {
	switch s {
	case IconsNerd:
		return "nerdfont"
	case IconsASCII:
		return "ascii"
	default:
		return "unicode"
	}
}

// ParseIconSet reads the config file spelling.
func ParseIconSet(s string) (IconSet, error) {
	switch s {
	case "ascii":
		return IconsASCII, nil
	case "unicode":
		return IconsUnicode, nil
	case "nerdfont":
		return IconsNerd, nil
	default:
		return IconsUnicode, fmt.Errorf("unknown icon set %q (want ascii, unicode, or nerdfont)", s)
	}
}

// IconWidth is the terminal cell width every glyph in the set occupies.
// Grid layout arithmetic depends on this being exact and uniform: a
// single double-width glyph would shift every row containing that
// condition. All three sets are deliberately single-width, which is why
// emoji are not used here.
func IconWidth(set IconSet) int { return 1 }

// condition groups WMO codes into the buckets we actually render.
type condition int

const (
	condClear condition = iota
	condPartly
	condCloudy
	condFog
	condDrizzle
	condRain
	condSnow
	condShowers
	condThunder
	condUnknown
)

// classify maps a WMO code to a render bucket.
// Reference: WMO 4677 present-weather codes as used by Open-Meteo.
func classify(code int) condition {
	switch code {
	case 0:
		return condClear
	case 1, 2:
		return condPartly
	case 3:
		return condCloudy
	case 45, 48:
		return condFog
	case 51, 53, 55, 56, 57:
		return condDrizzle
	case 61, 63, 65, 66, 67:
		return condRain
	case 71, 73, 75, 77:
		return condSnow
	case 80, 81, 82, 85, 86:
		return condShowers
	case 95, 96, 99:
		return condThunder
	default:
		return condUnknown
	}
}

// Glyph tables, indexed by condition. Every entry is one cell wide.
var (
	asciiGlyphs = [...]string{
		condClear: "O", condPartly: "o", condCloudy: "=", condFog: "~",
		condDrizzle: ".", condRain: "/", condSnow: "*", condShowers: ":",
		condThunder: "!", condUnknown: "?",
	}
	unicodeGlyphs = [...]string{
		condClear: "☀", condPartly: "◑", condCloudy: "☁", condFog: "≡",
		condDrizzle: "⋮", condRain: "☂", condSnow: "❄", condShowers: "⋰",
		condThunder: "⚡", condUnknown: "?",
	}
	// Nerd Font Weather range (nf-weather-*).
	nerdGlyphs = [...]string{
		condClear: "", condPartly: "", condCloudy: "",
		condFog: "", condDrizzle: "", condRain: "",
		condSnow: "", condShowers: "", condThunder: "",
		condUnknown: "",
	}
)

// Icon returns the one-cell glyph for a WMO code in the given set.
func Icon(code int, set IconSet) string {
	c := classify(code)
	switch set {
	case IconsASCII:
		return asciiGlyphs[c]
	case IconsNerd:
		return nerdGlyphs[c]
	default:
		return unicodeGlyphs[c]
	}
}

// labels are the human-readable condition names shown in the detail
// header. These are per-code, not per-bucket, because "Heavy rain" and
// "Slight rain" share a glyph but not a description.
var labels = map[int]string{
	0: "Clear sky", 1: "Mainly clear", 2: "Partly cloudy", 3: "Overcast",
	45: "Fog", 48: "Depositing rime fog",
	51: "Light drizzle", 53: "Moderate drizzle", 55: "Dense drizzle",
	56: "Light freezing drizzle", 57: "Dense freezing drizzle",
	61: "Slight rain", 63: "Moderate rain", 65: "Heavy rain",
	66: "Light freezing rain", 67: "Heavy freezing rain",
	71: "Slight snow", 73: "Moderate snow", 75: "Heavy snow",
	77: "Snow grains",
	80: "Slight rain showers", 81: "Moderate rain showers",
	82: "Violent rain showers",
	85: "Slight snow showers", 86: "Heavy snow showers",
	95: "Thunderstorm", 96: "Thunderstorm with slight hail",
	99: "Thunderstorm with heavy hail",
}

// Label returns the human-readable condition name for a WMO code.
func Label(code int) string {
	if l, ok := labels[code]; ok {
		return l
	}
	return "Unknown"
}
```

- [ ] **Step 4: Run it to confirm it passes**

Run: `go test ./internal/weather/ -v -run 'TestIcon|TestLabel|TestParseIconSet'`
Expected: PASS. **If the width invariant fails for a Unicode glyph, replace that glyph — do not relax the test.** The test is the guard that keeps the grid aligned.

- [ ] **Step 5: Commit**

```bash
git add internal/weather/
git commit -m "feat: add WMO code classification, labels, and icon sets"
```

---

### Task 8: Open-Meteo request construction

**Files:**
- Create: `internal/weather/openmeteo.go`, `internal/weather/openmeteo_test.go`

**Interfaces:**
- Consumes: `weather.City` (Task 2)
- Produces: `weather.Client` struct with fields `BaseURL string`, `HTTP *http.Client`, `Days int`; `weather.NewClient(days int) *Client`; unexported `(*Client).buildURL(cities []City) string`

> **Why the URL builder is its own task with its own test:** it encodes the entire batching decision. One request for twenty cities is the property that removes per-city concurrency from the UI, and it lives or dies on getting the comma-separated coordinate encoding and the variable list exactly right. Testing it without HTTP keeps the feedback loop instant.

- [ ] **Step 1: Write the failing test**

Create `internal/weather/openmeteo_test.go`:

```go
package weather

import (
	"net/url"
	"strings"
	"testing"
)

func testCities() []City {
	return []City{
		{ID: "a", Name: "Paris", Lat: 48.8566, Lon: 2.3522, TZ: "Europe/Paris"},
		{ID: "b", Name: "Tokyo", Lat: 35.6762, Lon: 139.6503, TZ: "Asia/Tokyo"},
	}
}

func TestBuildURLBatchesCoordinates(t *testing.T) {
	c := NewClient(7)
	raw := c.buildURL(testCities())

	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("buildURL produced an unparseable URL: %v", err)
	}
	q := u.Query()

	if got, want := q.Get("latitude"), "48.8566,35.6762"; got != want {
		t.Errorf("latitude = %q, want %q", got, want)
	}
	if got, want := q.Get("longitude"), "2.3522,139.6503"; got != want {
		t.Errorf("longitude = %q, want %q", got, want)
	}
	if got, want := q.Get("timezone"), "auto"; got != want {
		t.Errorf("timezone = %q, want %q", got, want)
	}
	if got, want := q.Get("forecast_days"), "7"; got != want {
		t.Errorf("forecast_days = %q, want %q", got, want)
	}
}

// Every variable the UI renders must be requested, or it silently
// arrives as zero and the grid shows plausible-looking wrong data.
func TestBuildURLRequestsEveryRenderedVariable(t *testing.T) {
	c := NewClient(7)
	u, _ := url.Parse(c.buildURL(testCities()))
	q := u.Query()

	for _, want := range []string{
		"temperature_2m", "apparent_temperature", "weather_code",
		"wind_speed_10m", "wind_direction_10m",
	} {
		if !strings.Contains(q.Get("current"), want) {
			t.Errorf("current is missing %q (got %q)", want, q.Get("current"))
		}
	}
	for _, want := range []string{
		"temperature_2m", "precipitation_probability", "precipitation",
		"weather_code", "wind_speed_10m", "wind_direction_10m",
	} {
		if !strings.Contains(q.Get("hourly"), want) {
			t.Errorf("hourly is missing %q (got %q)", want, q.Get("hourly"))
		}
	}
	for _, want := range []string{
		"weather_code", "temperature_2m_max", "temperature_2m_min",
		"precipitation_sum", "precipitation_probability_max",
		"wind_speed_10m_max", "uv_index_max", "sunrise", "sunset",
	} {
		if !strings.Contains(q.Get("daily"), want) {
			t.Errorf("daily is missing %q (got %q)", want, q.Get("daily"))
		}
	}
}

func TestBuildURLSingleCity(t *testing.T) {
	c := NewClient(3)
	u, _ := url.Parse(c.buildURL(testCities()[:1]))
	q := u.Query()

	if got, want := q.Get("latitude"), "48.8566"; got != want {
		t.Errorf("latitude = %q, want %q (no trailing comma)", got, want)
	}
	if got, want := q.Get("forecast_days"), "3"; got != want {
		t.Errorf("forecast_days = %q, want %q", got, want)
	}
}

func TestNewClientClampsDays(t *testing.T) {
	if got := NewClient(0).Days; got != 1 {
		t.Errorf("NewClient(0).Days = %d, want clamped to 1", got)
	}
	if got := NewClient(99).Days; got != 16 {
		t.Errorf("NewClient(99).Days = %d, want clamped to 16", got)
	}
}
```

- [ ] **Step 2: Run it to confirm it fails**

Run: `go test ./internal/weather/ -run TestBuildURL`
Expected: FAIL — `undefined: NewClient`

- [ ] **Step 3: Write the implementation**

Create `internal/weather/openmeteo.go`:

```go
package weather

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// DefaultBaseURL is the Open-Meteo forecast endpoint. It is a field on
// Client rather than a constant so tests can point at an httptest server.
const DefaultBaseURL = "https://api.open-meteo.com/v1/forecast"

// Variable lists. These are the single source of truth for what the API
// returns; every field the UI renders must appear here or it silently
// arrives as a zero value.
const (
	currentVars = "temperature_2m,apparent_temperature,weather_code," +
		"wind_speed_10m,wind_direction_10m,relative_humidity_2m"

	hourlyVars = "temperature_2m,apparent_temperature," +
		"precipitation_probability,precipitation,weather_code," +
		"wind_speed_10m,wind_direction_10m"

	dailyVars = "weather_code,temperature_2m_max,temperature_2m_min," +
		"precipitation_sum,precipitation_probability_max," +
		"wind_speed_10m_max,uv_index_max,sunrise,sunset"
)

// Client is a batched Open-Meteo forecast client. It implements Provider.
type Client struct {
	BaseURL string
	HTTP    *http.Client
	Days    int
}

// NewClient returns a client requesting the given forecast window,
// clamped to what the API supports.
func NewClient(days int) *Client {
	if days < 1 {
		days = 1
	}
	if days > 16 {
		days = 16
	}
	return &Client{
		BaseURL: DefaultBaseURL,
		HTTP:    &http.Client{Timeout: 15 * time.Second},
		Days:    days,
	}
}

// buildURL encodes every city into a single request. Open-Meteo accepts
// comma-separated coordinates and resolves timezone=auto per coordinate,
// so N cities cost exactly one HTTP request regardless of N.
//
// Coordinate order is the contract: the response is a positional list
// matching this input order. Callers must snapshot the city slice they
// pass here and map results back by index against that snapshot.
func (c *Client) buildURL(cities []City) string {
	lats := make([]string, len(cities))
	lons := make([]string, len(cities))
	for i, city := range cities {
		lats[i] = strconv.FormatFloat(city.Lat, 'f', -1, 64)
		lons[i] = strconv.FormatFloat(city.Lon, 'f', -1, 64)
	}

	q := url.Values{}
	q.Set("latitude", strings.Join(lats, ","))
	q.Set("longitude", strings.Join(lons, ","))
	q.Set("timezone", "auto")
	q.Set("forecast_days", strconv.Itoa(c.Days))
	q.Set("current", currentVars)
	q.Set("hourly", hourlyVars)
	q.Set("daily", dailyVars)

	return c.BaseURL + "?" + q.Encode()
}
```

- [ ] **Step 4: Run it to confirm it passes**

Run: `go test ./internal/weather/ -v -run 'TestBuildURL|TestNewClient'`
Expected: PASS, 4 tests

- [ ] **Step 5: Commit**

```bash
git add internal/weather/
git commit -m "feat: add batched Open-Meteo URL construction"
```

---

### Task 9: Response decoding and mapping to domain types

**Files:**
- Create: `internal/weather/decode.go`, `internal/weather/decode_test.go`
- Create: `internal/weather/testdata/two_cities.json`, `internal/weather/testdata/one_city.json`

**Interfaces:**
- Consumes: `weather.City`, `weather.Forecast`, `weather.Day`, `weather.Hour` (Task 2)
- Produces: unexported `decodeLocations([]byte) ([]rawLocation, error)`, `resolveLocation(tz string, offsetSec int) *time.Location`, `(rawLocation).toForecast(city City) (Forecast, error)`

> **The two hazards this task exists to neutralise.** First: Open-Meteo returns a JSON **object** for one coordinate and an **array** for many, so code written against the array shape works perfectly until a user deletes down to their last city. Second: timezone resolution — `time.LoadLocation` is correct across DST transitions but needs the tz database present, while a fixed offset always works but drifts an hour if the forecast window crosses a transition. The implementation tries the correct path and falls back, and `main.go` embeds the database so the fallback is rarely taken.

- [ ] **Step 1: Write the two-city fixture**

Create `internal/weather/testdata/two_cities.json`. Two locations, two forecast days, four hourly points each, with a deliberate `null` in Tokyo's precipitation series:

```json
[
  {
    "latitude": 48.86, "longitude": 2.35,
    "timezone": "Europe/Paris", "utc_offset_seconds": 7200,
    "current": {
      "time": "2026-09-17T14:00", "temperature_2m": 18.4,
      "apparent_temperature": 16.2, "weather_code": 3,
      "wind_speed_10m": 12.0, "wind_direction_10m": 315,
      "relative_humidity_2m": 62
    },
    "hourly": {
      "time": ["2026-09-17T00:00","2026-09-17T01:00","2026-09-18T00:00","2026-09-18T01:00"],
      "temperature_2m": [12.0, 11.5, 13.0, 12.5],
      "apparent_temperature": [10.0, 9.5, 11.0, 10.5],
      "precipitation_probability": [10, 20, 80, 90],
      "precipitation": [0.0, 0.1, 2.5, 3.0],
      "weather_code": [3, 3, 61, 63],
      "wind_speed_10m": [8.0, 9.0, 15.0, 18.0],
      "wind_direction_10m": [270, 280, 200, 190]
    },
    "daily": {
      "time": ["2026-09-17","2026-09-18"],
      "weather_code": [3, 63],
      "temperature_2m_max": [18.0, 15.0],
      "temperature_2m_min": [12.0, 11.0],
      "precipitation_sum": [0.1, 12.0],
      "precipitation_probability_max": [20, 90],
      "wind_speed_10m_max": [22.0, 28.0],
      "uv_index_max": [4.0, 2.0],
      "sunrise": ["2026-09-17T07:12","2026-09-18T07:14"],
      "sunset": ["2026-09-17T19:48","2026-09-18T19:46"]
    }
  },
  {
    "latitude": 35.68, "longitude": 139.65,
    "timezone": "Asia/Tokyo", "utc_offset_seconds": 32400,
    "current": {
      "time": "2026-09-17T21:00", "temperature_2m": 11.0,
      "apparent_temperature": 9.0, "weather_code": 61,
      "wind_speed_10m": 6.0, "wind_direction_10m": 90,
      "relative_humidity_2m": 88
    },
    "hourly": {
      "time": ["2026-09-17T00:00","2026-09-17T01:00","2026-09-18T00:00","2026-09-18T01:00"],
      "temperature_2m": [9.0, 8.5, 10.0, 9.5],
      "apparent_temperature": [7.0, 6.5, 8.0, 7.5],
      "precipitation_probability": [70, null, 60, 55],
      "precipitation": [1.0, null, 0.5, 0.2],
      "weather_code": [61, 61, 61, 3],
      "wind_speed_10m": [5.0, 6.0, 7.0, 6.5],
      "wind_direction_10m": [80, 90, 100, 110]
    },
    "daily": {
      "time": ["2026-09-17","2026-09-18"],
      "weather_code": [61, 61],
      "temperature_2m_max": [11.0, 12.0],
      "temperature_2m_min": [9.0, 9.5],
      "precipitation_sum": [8.0, 6.0],
      "precipitation_probability_max": [70, 60],
      "wind_speed_10m_max": [12.0, 14.0],
      "uv_index_max": [2.0, 3.0],
      "sunrise": ["2026-09-17T05:30","2026-09-18T05:31"],
      "sunset": ["2026-09-17T17:45","2026-09-18T17:43"]
    }
  }
]
```

- [ ] **Step 2: Write the single-city fixture**

Create `internal/weather/testdata/one_city.json` — **a bare object, not an array.** This is the shape the API returns for a single coordinate:

```json
{
  "latitude": 48.86, "longitude": 2.35,
  "timezone": "Europe/Paris", "utc_offset_seconds": 7200,
  "current": {
    "time": "2026-09-17T14:00", "temperature_2m": 18.4,
    "apparent_temperature": 16.2, "weather_code": 3,
    "wind_speed_10m": 12.0, "wind_direction_10m": 315,
    "relative_humidity_2m": 62
  },
  "hourly": {
    "time": ["2026-09-17T00:00","2026-09-17T01:00"],
    "temperature_2m": [12.0, 11.5],
    "apparent_temperature": [10.0, 9.5],
    "precipitation_probability": [10, 20],
    "precipitation": [0.0, 0.1],
    "weather_code": [3, 3],
    "wind_speed_10m": [8.0, 9.0],
    "wind_direction_10m": [270, 280]
  },
  "daily": {
    "time": ["2026-09-17"],
    "weather_code": [3],
    "temperature_2m_max": [18.0],
    "temperature_2m_min": [12.0],
    "precipitation_sum": [0.1],
    "precipitation_probability_max": [20],
    "wind_speed_10m_max": [22.0],
    "uv_index_max": [4.0],
    "sunrise": ["2026-09-17T07:12"],
    "sunset": ["2026-09-17T19:48"]
  }
}
```

- [ ] **Step 3: Write the failing test**

Create `internal/weather/decode_test.go`:

```go
package weather

import (
	"os"
	"testing"
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
```

Add `"time"` to the test file's imports.

- [ ] **Step 4: Run it to confirm it fails**

Run: `go test ./internal/weather/ -run TestDecode`
Expected: FAIL — `undefined: decodeLocations`

- [ ] **Step 5: Write the implementation**

Create `internal/weather/decode.go`:

```go
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
```

- [ ] **Step 6: Run it to confirm it passes**

Run: `go test ./internal/weather/ -v -run 'TestDecode|TestToForecast|TestResolveLocation'`
Expected: PASS, 8 tests

- [ ] **Step 7: Commit**

```bash
git add internal/weather/
git commit -m "feat: decode Open-Meteo responses into domain forecasts"
```

---

### Task 10: FetchAll with HTTP error handling

**Files:**
- Modify: `internal/weather/openmeteo.go` (append `FetchAll` and error types)
- Create: `internal/weather/fetch_test.go`

**Interfaces:**
- Consumes: `decodeLocations`, `toForecast` (Task 9); `Client.buildURL` (Task 8)
- Produces: `(*Client).FetchAll(ctx context.Context, cities []City) ([]Forecast, error)` — satisfying `weather.Provider`; `weather.RateLimitError` with field `RetryAfter time.Duration`; `weather.ErrNoCities`

> **Why results are matched by index against the caller's slice:** the response is positional. `FetchAll` receives the city slice and stamps each returned `Forecast` with the corresponding `City.ID`, so everything downstream works with IDs and never with positions. This is what makes an add, delete, or reorder during an in-flight request harmless.

- [ ] **Step 1: Write the failing test**

Create `internal/weather/fetch_test.go`:

```go
package weather

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func serveFixture(t *testing.T, name string) *httptest.Server {
	t.Helper()
	body := loadFixture(t, name)
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
}

func TestFetchAllMapsResultsToCityIDs(t *testing.T) {
	srv := serveFixture(t, "two_cities.json")
	defer srv.Close()

	c := NewClient(7)
	c.BaseURL = srv.URL

	cities := []City{
		{ID: "paris", Name: "Paris", Lat: 48.86, Lon: 2.35, TZ: "Europe/Paris"},
		{ID: "tokyo", Name: "Tokyo", Lat: 35.68, Lon: 139.65, TZ: "Asia/Tokyo"},
	}

	got, err := c.FetchAll(context.Background(), cities)
	if err != nil {
		t.Fatalf("FetchAll: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d forecasts, want 2", len(got))
	}
	// Positional response, keyed output: index 0 must carry city 0's ID.
	if got[0].CityID != "paris" || got[1].CityID != "tokyo" {
		t.Errorf("city IDs = %q,%q; want paris,tokyo", got[0].CityID, got[1].CityID)
	}
	if got[1].Days[0].HiC != 11 {
		t.Errorf("tokyo day 0 HiC = %v, want 11", got[1].Days[0].HiC)
	}
}

func TestFetchAllSingleCity(t *testing.T) {
	srv := serveFixture(t, "one_city.json")
	defer srv.Close()

	c := NewClient(7)
	c.BaseURL = srv.URL

	got, err := c.FetchAll(context.Background(),
		[]City{{ID: "paris", Lat: 48.86, Lon: 2.35, TZ: "Europe/Paris"}})
	if err != nil {
		t.Fatalf("single-city FetchAll: %v", err)
	}
	if len(got) != 1 || got[0].CityID != "paris" {
		t.Fatalf("got %+v, want one forecast for paris", got)
	}
}

func TestFetchAllEmptyCityListSkipsRequest(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer srv.Close()

	c := NewClient(7)
	c.BaseURL = srv.URL

	got, err := c.FetchAll(context.Background(), nil)
	if err != nil {
		t.Fatalf("empty city list must not error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d forecasts, want 0", len(got))
	}
	if called {
		t.Error("no HTTP request should be made for an empty city list")
	}
}

func TestFetchAllRateLimited(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	c := NewClient(7)
	c.BaseURL = srv.URL

	_, err := c.FetchAll(context.Background(), []City{{ID: "a", TZ: "UTC"}})
	var rl *RateLimitError
	if !errors.As(err, &rl) {
		t.Fatalf("err = %v, want a *RateLimitError", err)
	}
	if rl.RetryAfter != 30*time.Second {
		t.Errorf("RetryAfter = %v, want 30s", rl.RetryAfter)
	}
}

func TestFetchAllServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewClient(7)
	c.BaseURL = srv.URL

	if _, err := c.FetchAll(context.Background(), []City{{ID: "a", TZ: "UTC"}}); err == nil {
		t.Fatal("HTTP 500 must return an error")
	}
}

func TestFetchAllMalformedBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("{{{ truncated"))
	}))
	defer srv.Close()

	c := NewClient(7)
	c.BaseURL = srv.URL

	if _, err := c.FetchAll(context.Background(), []City{{ID: "a", TZ: "UTC"}}); err == nil {
		t.Fatal("malformed JSON must return an error")
	}
}

// A truncated response must not silently drop cities.
func TestFetchAllCountMismatch(t *testing.T) {
	srv := serveFixture(t, "one_city.json")
	defer srv.Close()

	c := NewClient(7)
	c.BaseURL = srv.URL

	_, err := c.FetchAll(context.Background(), []City{
		{ID: "a", TZ: "UTC"}, {ID: "b", TZ: "UTC"},
	})
	if err == nil {
		t.Fatal("a response with fewer locations than cities must error")
	}
}

func TestFetchAllRespectsContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	c := NewClient(7)
	c.BaseURL = srv.URL

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := c.FetchAll(ctx, []City{{ID: "a", TZ: "UTC"}}); err == nil {
		t.Fatal("a cancelled context must return an error")
	}
}
```

- [ ] **Step 2: Run it to confirm it fails**

Run: `go test ./internal/weather/ -run TestFetchAll`
Expected: FAIL — `c.FetchAll undefined`

- [ ] **Step 3: Append the implementation to `internal/weather/openmeteo.go`**

Add these imports to the file's import block: `context`, `fmt`, `io`.

```go
// RateLimitError reports an HTTP 429. It carries Retry-After so the UI
// can back off for the stated interval instead of guessing.
type RateLimitError struct {
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("rate limited, retry after %s", e.RetryAfter)
}

// maxBody caps how much of a response we will read. Twenty cities of
// hourly data is well under a megabyte; anything past 8MB is a bug or a
// hostile response, and reading it would be a memory problem.
const maxBody = 8 << 20

// FetchAll retrieves forecasts for every city in a single HTTP request.
//
// The response is positional, matching the coordinate order in the
// request, so each result is stamped with the corresponding City.ID
// before returning. Everything downstream keys on that ID, which is why
// a concurrent add, delete, or reorder cannot misalign rows.
//
// An empty city list is a no-op, not an error: a first-run user has no
// cities, and firing a coordinate-less request would fail confusingly.
func (c *Client) FetchAll(ctx context.Context, cities []City) ([]Forecast, error) {
	if len(cities) == 0 {
		return nil, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.buildURL(cities), nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch forecasts: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, &RateLimitError{RetryAfter: retryAfter(resp)}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("forecast API returned %s", resp.Status)
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	locs, err := decodeLocations(raw)
	if err != nil {
		return nil, err
	}
	if len(locs) != len(cities) {
		return nil, fmt.Errorf(
			"forecast API returned %d locations for %d cities", len(locs), len(cities))
	}

	out := make([]Forecast, 0, len(cities))
	for i, loc := range locs {
		f, err := loc.toForecast(cities[i])
		if err != nil {
			return nil, fmt.Errorf("map %s: %w", cities[i].Name, err)
		}
		out = append(out, f)
	}
	return out, nil
}

// retryAfter reads the Retry-After header, falling back to a
// conservative default when it is absent or unparseable.
func retryAfter(resp *http.Response) time.Duration {
	const fallback = 60 * time.Second
	v := resp.Header.Get("Retry-After")
	if v == "" {
		return fallback
	}
	if secs, err := strconv.Atoi(v); err == nil && secs > 0 {
		return time.Duration(secs) * time.Second
	}
	return fallback
}
```

- [ ] **Step 4: Run it to confirm it passes**

Run: `go test ./internal/weather/ -v -race`
Expected: PASS, all tests

- [ ] **Step 5: Assert the interface is satisfied at compile time**

Append to `internal/weather/openmeteo.go`:

```go
// Compile-time proof that Client satisfies Provider. Without this the
// mismatch would only surface when ui.New is first wired up.
var _ Provider = (*Client)(nil)
```

- [ ] **Step 6: Run the full suite**

Run: `make test && make lint`
Expected: PASS, gofmt silent

- [ ] **Step 7: Commit**

```bash
git add internal/weather/
git commit -m "feat: add batched FetchAll with rate-limit and error handling"
```

---

### Task 11: Geocoding client

**Files:**
- Create: `internal/geocode/geocode.go`, `internal/geocode/geocode_test.go`, `internal/geocode/testdata/paris.json`

**Interfaces:**
- Consumes: `weather.City` (Task 2)
- Produces: `geocode.Place` struct, `geocode.Client` with `BaseURL`/`HTTP`, `geocode.NewClient() *Client`, `(*Client).Search(ctx context.Context, query string) ([]Place, error)`, `(Place).ToCity() weather.City`, `(Place).Label() string`

> **Why geocoding happens exactly once, at add time:** the resulting latitude, longitude, and IANA timezone are stored in config forever. Re-geocoding on every refresh would reintroduce the ambiguity the search UI exists to resolve — there are roughly thirty places called Paris — and would cost an extra request per cycle for no benefit.

- [ ] **Step 1: Write the fixture**

Create `internal/geocode/testdata/paris.json`:

```json
{
  "results": [
    {
      "id": 2988507, "name": "Paris",
      "latitude": 48.85341, "longitude": 2.3488,
      "country": "France", "country_code": "FR",
      "admin1": "Île-de-France", "timezone": "Europe/Paris",
      "population": 2138551
    },
    {
      "id": 4717560, "name": "Paris",
      "latitude": 33.66094, "longitude": -95.55551,
      "country": "United States", "country_code": "US",
      "admin1": "Texas", "timezone": "America/Chicago",
      "population": 25171
    }
  ]
}
```

- [ ] **Step 2: Write the failing test**

Create `internal/geocode/geocode_test.go`:

```go
package geocode

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
)

func serveFixture(t *testing.T, name string) (*httptest.Server, *url.Values) {
	t.Helper()
	body, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var seen url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.URL.Query()
		_, _ = w.Write(body)
	}))
	return srv, &seen
}

func TestSearchParsesResults(t *testing.T) {
	srv, query := serveFixture(t, "paris.json")
	defer srv.Close()

	c := NewClient()
	c.BaseURL = srv.URL

	got, err := c.Search(context.Background(), "Paris")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d places, want 2", len(got))
	}
	if query.Get("name") != "Paris" {
		t.Errorf("name param = %q, want Paris", query.Get("name"))
	}
	if got[0].Admin1 != "Île-de-France" || got[1].Admin1 != "Texas" {
		t.Errorf("admin1 = %q,%q", got[0].Admin1, got[1].Admin1)
	}
}

// Disambiguation is the whole point of the search UI: two places with
// the same name must render distinguishably.
func TestPlaceLabelDisambiguates(t *testing.T) {
	srv, _ := serveFixture(t, "paris.json")
	defer srv.Close()

	c := NewClient()
	c.BaseURL = srv.URL
	places, _ := c.Search(context.Background(), "Paris")

	a, b := places[0].Label(), places[1].Label()
	if a == b {
		t.Fatalf("both places labelled %q; labels must differ", a)
	}
	if want := "Paris · Île-de-France · France"; a != want {
		t.Errorf("label = %q, want %q", a, want)
	}
}

func TestToCityCarriesTimezone(t *testing.T) {
	srv, _ := serveFixture(t, "paris.json")
	defer srv.Close()

	c := NewClient()
	c.BaseURL = srv.URL
	places, _ := c.Search(context.Background(), "Paris")

	city := places[0].ToCity()
	if city.TZ != "Europe/Paris" {
		t.Errorf("TZ = %q, want Europe/Paris", city.TZ)
	}
	if city.CC != "FR" {
		t.Errorf("CC = %q, want FR", city.CC)
	}
	if city.ID == "" {
		t.Error("ToCity must assign a stable non-empty ID")
	}
	if city.Lat == 0 || city.Lon == 0 {
		t.Errorf("coords = %v,%v, want the fixture values", city.Lat, city.Lon)
	}
}

func TestToCityIDIsStable(t *testing.T) {
	srv, _ := serveFixture(t, "paris.json")
	defer srv.Close()

	c := NewClient()
	c.BaseURL = srv.URL
	places, _ := c.Search(context.Background(), "Paris")

	if places[0].ToCity().ID != places[0].ToCity().ID {
		t.Error("ToCity must produce the same ID for the same place")
	}
	if places[0].ToCity().ID == places[1].ToCity().ID {
		t.Error("distinct places must produce distinct IDs")
	}
}

// Open-Meteo omits the results key entirely when nothing matches.
func TestSearchNoResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"generationtime_ms":0.1}`))
	}))
	defer srv.Close()

	c := NewClient()
	c.BaseURL = srv.URL

	got, err := c.Search(context.Background(), "zzzzzz")
	if err != nil {
		t.Fatalf("an empty result set must not error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d places, want 0", len(got))
	}
}

func TestSearchBlankQuery(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer srv.Close()

	c := NewClient()
	c.BaseURL = srv.URL

	got, err := c.Search(context.Background(), "   ")
	if err != nil || len(got) != 0 {
		t.Errorf("blank query: got %v, %v; want no results and no error", got, err)
	}
	if called {
		t.Error("a blank query must not reach the network")
	}
}

func TestSearchServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	c := NewClient()
	c.BaseURL = srv.URL

	if _, err := c.Search(context.Background(), "Paris"); err == nil {
		t.Fatal("HTTP 502 must return an error")
	}
}
```

- [ ] **Step 3: Run it to confirm it fails**

Run: `go test ./internal/geocode/`
Expected: FAIL — `undefined: NewClient`

- [ ] **Step 4: Write the implementation**

Create `internal/geocode/geocode.go`:

```go
// Package geocode resolves a typed city name to coordinates and a
// timezone using the Open-Meteo geocoding API.
//
// Resolution happens exactly once, when a city is added. The resulting
// latitude, longitude, and IANA timezone are stored in config forever,
// so refreshes never re-geocode: the ambiguity this package exists to
// resolve stays resolved, and each refresh costs one request fewer.
package geocode

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/tsoueid/weatherdash/internal/weather"
)

// DefaultBaseURL is the Open-Meteo geocoding search endpoint.
const DefaultBaseURL = "https://geocoding-api.open-meteo.com/v1/search"

// maxResults is how many candidates the picker shows. Ten is enough to
// surface the intended Paris without turning the modal into a list to
// scroll through.
const maxResults = 10

// Place is one geocoding candidate.
type Place struct {
	ID        int     `json:"id"`
	Name      string  `json:"name"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Country   string  `json:"country"`
	CC        string  `json:"country_code"`
	Admin1    string  `json:"admin1"`
	Timezone  string  `json:"timezone"`
}

// Label renders the candidate for the picker. It includes the
// administrative region because disambiguation is the entire purpose of
// this list — there are around thirty places named Paris.
func (p Place) Label() string {
	parts := make([]string, 0, 3)
	for _, s := range []string{p.Name, p.Admin1, p.Country} {
		if s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, " · ")
}

// ToCity converts a candidate into a persistable city.
//
// The ID is derived from the geocoding database's own ID, which is
// stable across sessions. That stability is what lets the cache key on
// city ID and survive restarts.
func (p Place) ToCity() weather.City {
	return weather.City{
		ID:      "om-" + strconv.Itoa(p.ID),
		Name:    p.Name,
		Admin:   p.Admin1,
		Country: p.Country,
		CC:      p.CC,
		Lat:     p.Latitude,
		Lon:     p.Longitude,
		TZ:      p.Timezone,
	}
}

// Client searches the Open-Meteo geocoding API.
type Client struct {
	BaseURL string
	HTTP    *http.Client
}

// NewClient returns a geocoding client with a short timeout — search is
// interactive, so a slow response is worse than no response.
func NewClient() *Client {
	return &Client{
		BaseURL: DefaultBaseURL,
		HTTP:    &http.Client{Timeout: 8 * time.Second},
	}
}

type searchResponse struct {
	Results []Place `json:"results"`
}

// Search returns candidate places for a query. A blank query returns
// nothing without touching the network, which matters because the
// caller debounces keystrokes and will hit this on every backspace.
func (c *Client) Search(ctx context.Context, query string) ([]Place, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}

	q := url.Values{}
	q.Set("name", query)
	q.Set("count", strconv.Itoa(maxResults))
	q.Set("language", "en")
	q.Set("format", "json")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"?"+q.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("build geocode request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("geocode search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("geocoding API returned %s", resp.Status)
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read geocode response: %w", err)
	}

	// The API omits "results" entirely when nothing matches, which
	// decodes to a nil slice — an empty result, not an error.
	var out searchResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("decode geocode response: %w", err)
	}
	return out.Results, nil
}
```

- [ ] **Step 5: Run it to confirm it passes**

Run: `go test ./internal/geocode/ -v -race`
Expected: PASS, 7 tests

- [ ] **Step 6: Verify the whole phase**

Run: `make test && make lint`
Expected: all packages pass

- [ ] **Step 7: Commit**

```bash
git add internal/geocode/
git commit -m "feat: add Open-Meteo geocoding client for city search"
```

---

# Phase 3 — Pure Widget Renderers

Everything in this phase is `func(data, width) string` with no Bubble Tea import. That is what keeps `View()` an assembly step rather than a drawing method, and it is what makes the whole visual layer testable without a terminal.

### Task 12: Width-safe text measurement and resampling

**Files:**
- Create: `internal/widgets/measure.go`, `internal/widgets/measure_test.go`

**Interfaces:**
- Consumes: nothing
- Produces: `widgets.Pad(s string, w int) string`, `widgets.PadLeft(s string, w int) string`, `widgets.Truncate(s string, w int) string`, `widgets.Resample(vals []float64, n int) []float64`

> **Why `Pad` cannot use `len`:** `len("Málaga")` is 7 bytes but 6 columns, and `len("☀")` is 3 bytes but 1 column. Padding by byte length puts the grid's right border in a different place on every row whose city has an accent. Every function in this package pads and truncates through `lipgloss.Width`.

- [ ] **Step 1: Write the failing test**

Create `internal/widgets/measure_test.go`:

```go
package widgets

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestPadIsColumnAccurate(t *testing.T) {
	tests := []struct{ name, in string }{
		{"ascii", "Paris"},
		{"accented", "Málaga"},
		{"cjk", "東京"},
		{"glyph", "☀"},
		{"empty", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Pad(tc.in, 12)
			if w := lipgloss.Width(got); w != 12 {
				t.Errorf("Pad(%q, 12) width = %d, want 12 (got %q)", tc.in, w, got)
			}
		})
	}
}

func TestPadTruncatesWhenTooLong(t *testing.T) {
	got := Pad("a very long city name indeed", 8)
	if w := lipgloss.Width(got); w != 8 {
		t.Errorf("width = %d, want 8 (got %q)", w, got)
	}
}

func TestPadLeftRightAligns(t *testing.T) {
	got := PadLeft("18°", 6)
	if got != "   18°" {
		t.Errorf("PadLeft = %q, want %q", got, "   18°")
	}
}

func TestPadZeroOrNegativeWidth(t *testing.T) {
	if got := Pad("Paris", 0); got != "" {
		t.Errorf("Pad(_, 0) = %q, want empty", got)
	}
	if got := Pad("Paris", -3); got != "" {
		t.Errorf("Pad(_, -3) = %q, want empty", got)
	}
}

func TestTruncateNeverSplitsAGlyph(t *testing.T) {
	// Truncating a double-width glyph to an odd budget must not emit
	// half a character; it pads instead.
	got := Truncate("東京都", 3)
	if w := lipgloss.Width(got); w > 3 {
		t.Errorf("Truncate width = %d, want <= 3 (got %q)", w, got)
	}
}

func TestResampleExactLength(t *testing.T) {
	in := []float64{1, 2, 3, 4}
	got := Resample(in, 4)
	for i := range in {
		if got[i] != in[i] {
			t.Errorf("Resample[%d] = %v, want %v", i, got[i], in[i])
		}
	}
}

func TestResampleDownsamplesByAveraging(t *testing.T) {
	// Four points into two buckets: means of (1,3) and (5,7).
	got := Resample([]float64{1, 3, 5, 7}, 2)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0] != 2 || got[1] != 6 {
		t.Errorf("Resample = %v, want [2 6]", got)
	}
}

func TestResampleUpsamples(t *testing.T) {
	got := Resample([]float64{0, 10}, 5)
	if len(got) != 5 {
		t.Fatalf("len = %d, want 5", len(got))
	}
	if got[0] != 0 || got[4] != 10 {
		t.Errorf("endpoints = %v,%v; want 0,10", got[0], got[4])
	}
}

func TestResampleEdgeCases(t *testing.T) {
	if got := Resample(nil, 5); len(got) != 0 {
		t.Errorf("Resample(nil, 5) = %v, want empty", got)
	}
	if got := Resample([]float64{1, 2}, 0); len(got) != 0 {
		t.Errorf("Resample(_, 0) = %v, want empty", got)
	}
}
```

- [ ] **Step 2: Run it to confirm it fails**

Run: `go test ./internal/widgets/`
Expected: FAIL — `undefined: Pad`

- [ ] **Step 3: Write the implementation**

Create `internal/widgets/measure.go`:

```go
// Package widgets renders forecast data to plain strings.
//
// Every function here is pure: data in, string out, no Bubble Tea and
// no global state. That keeps View() an assembly step rather than a
// drawing method, and it means the entire visual layer is testable
// without a terminal.
//
// Colour is deliberately NOT applied here. Widgets emit plain text and
// accept an optional Colorize hook, so golden tests compare characters
// rather than ANSI escape sequences.
package widgets

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Pad left-aligns s in a field exactly w terminal columns wide,
// truncating if necessary.
//
// Width is measured with lipgloss.Width, never len: "Málaga" is 7 bytes
// but 6 columns, and "東京" is 6 bytes but 4 columns. Padding by byte
// count moves the grid's right border on every row with a non-ASCII
// name.
func Pad(s string, w int) string {
	if w <= 0 {
		return ""
	}
	s = Truncate(s, w)
	if gap := w - lipgloss.Width(s); gap > 0 {
		return s + strings.Repeat(" ", gap)
	}
	return s
}

// PadLeft right-aligns s in a field exactly w columns wide. Numeric
// columns read better right-aligned, so temperatures use this.
func PadLeft(s string, w int) string {
	if w <= 0 {
		return ""
	}
	s = Truncate(s, w)
	if gap := w - lipgloss.Width(s); gap > 0 {
		return strings.Repeat(" ", gap) + s
	}
	return s
}

// Truncate cuts s to at most w columns without ever splitting a glyph.
// A double-width rune that would straddle the boundary is dropped
// rather than half-drawn, so the result may be one column short — which
// Pad then fills.
func Truncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= w {
		return s
	}
	var b strings.Builder
	used := 0
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if used+rw > w {
			break
		}
		b.WriteRune(r)
		used += rw
	}
	return b.String()
}

// Resample maps vals onto exactly n points.
//
// Downsampling averages each bucket, which is the right choice for
// temperature and precipitation: taking every k-th sample would let a
// single spike vanish, while the mean keeps the shape honest.
// Upsampling interpolates linearly.
func Resample(vals []float64, n int) []float64 {
	if n <= 0 || len(vals) == 0 {
		return nil
	}
	if len(vals) == n {
		out := make([]float64, n)
		copy(out, vals)
		return out
	}

	out := make([]float64, n)

	if len(vals) > n {
		for i := range out {
			lo := i * len(vals) / n
			hi := (i + 1) * len(vals) / n
			if hi <= lo {
				hi = lo + 1
			}
			var sum float64
			for _, v := range vals[lo:hi] {
				sum += v
			}
			out[i] = sum / float64(hi-lo)
		}
		return out
	}

	if n == 1 {
		out[0] = vals[0]
		return out
	}
	for i := range out {
		pos := float64(i) * float64(len(vals)-1) / float64(n-1)
		lo := int(pos)
		hi := lo + 1
		if hi >= len(vals) {
			out[i] = vals[len(vals)-1]
			continue
		}
		frac := pos - float64(lo)
		out[i] = vals[lo]*(1-frac) + vals[hi]*frac
	}
	return out
}
```

- [ ] **Step 4: Run it to confirm it passes**

Run: `go test ./internal/widgets/ -v`
Expected: PASS, 9 tests

- [ ] **Step 5: Commit**

```bash
git add internal/widgets/
git commit -m "feat: add column-accurate padding and series resampling"
```

---

### Task 13: Sparkline and precipitation band

**Files:**
- Create: `internal/widgets/sparkline.go`, `internal/widgets/sparkline_test.go`

**Interfaces:**
- Consumes: `widgets.Resample` (Task 12)
- Produces: `widgets.Colorize` func type, `widgets.Sparkline(vals []float64, width int, c Colorize) string`, `widgets.PrecipBand(probs []int, width int) string`

> **Why the precipitation band is a separate row rather than colour on the sparkline:** two independent quantities encoded in one glyph are unreadable. Stacking a temperature sparkline over a shaded probability band lets the eye find "the dry window" — the single most useful thing the detail screen does — without decoding anything.

- [ ] **Step 1: Write the failing test**

Create `internal/widgets/sparkline_test.go`:

```go
package widgets

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestSparklineWidth(t *testing.T) {
	vals := []float64{1, 5, 3, 8, 2, 7, 4}
	for _, w := range []int{1, 5, 7, 24, 40} {
		got := Sparkline(vals, w, nil)
		if lipgloss.Width(got) != w {
			t.Errorf("Sparkline width %d = %d (got %q)", w, lipgloss.Width(got), got)
		}
	}
}

func TestSparklineMapsRangeToFullRamp(t *testing.T) {
	got := Sparkline([]float64{0, 1, 2, 3, 4, 5, 6, 7}, 8, nil)
	if got != "▁▂▃▄▅▆▇█" {
		t.Errorf("Sparkline = %q, want the full ramp", got)
	}
}

// A flat series has no range to normalise against. It must render as a
// consistent mid-height line, not divide by zero.
func TestSparklineFlatSeries(t *testing.T) {
	got := Sparkline([]float64{5, 5, 5, 5}, 4, nil)
	if lipgloss.Width(got) != 4 {
		t.Fatalf("width = %d, want 4", lipgloss.Width(got))
	}
	for _, r := range got {
		if r != '▄' {
			t.Errorf("flat series rendered %q, want all ▄", got)
			break
		}
	}
}

func TestSparklineNegativeValues(t *testing.T) {
	got := Sparkline([]float64{-10, 0, 10}, 3, nil)
	if got != "▁▄█" {
		t.Errorf("Sparkline with negatives = %q, want ▁▄█", got)
	}
}

func TestSparklineEmptyAndZeroWidth(t *testing.T) {
	if got := Sparkline(nil, 5, nil); lipgloss.Width(got) != 5 {
		t.Errorf("nil values must still fill the width, got %q", got)
	}
	if got := Sparkline([]float64{1, 2}, 0, nil); got != "" {
		t.Errorf("zero width = %q, want empty", got)
	}
}

func TestSparklineAppliesColorize(t *testing.T) {
	red := lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	got := Sparkline([]float64{1, 2, 3}, 3, func(float64) lipgloss.Style { return red })

	if lipgloss.Width(got) != 3 {
		t.Errorf("colorized width = %d, want 3", lipgloss.Width(got))
	}
	if got == "▁▄█" {
		t.Error("Colorize was not applied")
	}
}

func TestPrecipBandRamp(t *testing.T) {
	got := PrecipBand([]int{0, 15, 40, 65, 90}, 5)
	if got != "·░▒▓█" {
		t.Errorf("PrecipBand = %q, want ·░▒▓█", got)
	}
}

func TestPrecipBandWidthAndClamping(t *testing.T) {
	if got := PrecipBand([]int{50}, 10); lipgloss.Width(got) != 10 {
		t.Errorf("width = %d, want 10", lipgloss.Width(got))
	}
	// Out-of-range probabilities must clamp, not index out of bounds.
	if got := PrecipBand([]int{-20, 250}, 2); lipgloss.Width(got) != 2 {
		t.Errorf("clamped width = %d, want 2 (got %q)", lipgloss.Width(got), got)
	}
}

func TestPrecipBandEmpty(t *testing.T) {
	got := PrecipBand(nil, 4)
	if got != "····" {
		t.Errorf("PrecipBand(nil, 4) = %q, want ····", got)
	}
}
```

- [ ] **Step 2: Run it to confirm it fails**

Run: `go test ./internal/widgets/ -run 'TestSparkline|TestPrecip'`
Expected: FAIL — `undefined: Sparkline`

- [ ] **Step 3: Write the implementation**

Create `internal/widgets/sparkline.go`:

```go
package widgets

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// sparkRamp is the eight-level block ramp used for value series.
var sparkRamp = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// precipRamp shades precipitation probability. The first level is a
// mid-dot rather than a space so an empty forecast still reads as a
// row of data rather than a rendering gap.
var precipRamp = []rune{'·', '░', '▒', '▓', '█'}

// Colorize optionally styles each point of a series by its value.
// Passing nil renders plain text, which is what golden tests do so they
// compare characters instead of ANSI escape sequences.
type Colorize func(value float64) lipgloss.Style

// Sparkline renders vals as a block-character series exactly width
// columns wide, resampling as needed.
//
// The series is normalised against its own min and max, so the ramp
// always spans the data's actual range. That makes shape legible at the
// cost of absolute comparability between rows — which is the right
// trade for a 24-hour temperature curve, where the question is "when
// does it peak", not "how does this compare to Oslo".
func Sparkline(vals []float64, width int, c Colorize) string {
	if width <= 0 {
		return ""
	}
	pts := Resample(vals, width)
	if len(pts) == 0 {
		return strings.Repeat(" ", width)
	}

	min, max := pts[0], pts[0]
	for _, v := range pts {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}

	var b strings.Builder
	span := max - min
	for _, v := range pts {
		var idx int
		if span == 0 {
			// A flat series has no range to normalise against. Render a
			// consistent mid-height line rather than dividing by zero.
			idx = len(sparkRamp) / 2
		} else {
			idx = int((v - min) / span * float64(len(sparkRamp)-1))
		}
		if idx < 0 {
			idx = 0
		}
		if idx >= len(sparkRamp) {
			idx = len(sparkRamp) - 1
		}

		ch := string(sparkRamp[idx])
		if c != nil {
			ch = c(v).Render(ch)
		}
		b.WriteString(ch)
	}
	return b.String()
}

// PrecipBand renders precipitation probabilities as a shaded band
// exactly width columns wide.
//
// Unlike Sparkline this uses ABSOLUTE thresholds, not a normalised
// range: an 80% chance must look the same in every row, or the band
// stops answering "will it rain" and starts answering "is this hour
// wetter than its neighbours", which is not the question.
func PrecipBand(probs []int, width int) string {
	if width <= 0 {
		return ""
	}
	if len(probs) == 0 {
		return strings.Repeat(string(precipRamp[0]), width)
	}

	vals := make([]float64, len(probs))
	for i, p := range probs {
		vals[i] = float64(p)
	}

	var b strings.Builder
	for _, v := range Resample(vals, width) {
		b.WriteRune(precipLevel(v))
	}
	return b.String()
}

// precipLevel maps a probability percentage to a shade. Thresholds are
// absolute and deliberately uneven: the interesting decision boundary
// sits around "might rain" (30%) and "will rain" (60%).
func precipLevel(pct float64) rune {
	switch {
	case pct < 10:
		return precipRamp[0]
	case pct < 30:
		return precipRamp[1]
	case pct < 60:
		return precipRamp[2]
	case pct < 80:
		return precipRamp[3]
	default:
		return precipRamp[4]
	}
}
```

- [ ] **Step 4: Run it to confirm it passes**

Run: `go test ./internal/widgets/ -v -run 'TestSparkline|TestPrecip'`
Expected: PASS, 9 tests

- [ ] **Step 5: Commit**

```bash
git add internal/widgets/
git commit -m "feat: add sparkline and precipitation band renderers"
```

---

### Task 14: Grid cell tiers

**Files:**
- Create: `internal/widgets/cell.go`, `internal/widgets/cell_test.go`

**Interfaces:**
- Consumes: `widgets.Pad` (Task 12); `weather.Day`, `weather.Icon`, `weather.IconSet` (Tasks 2, 7); `units.System` (Task 3)
- Produces: `widgets.Tier` (with `TierNone`, `TierMinimal`, `TierMedium`, `TierRich`), `widgets.TierFor(cellW int) Tier`, `(Tier).MinWidth() int`, `widgets.Cell(d weather.Day, width int, tier Tier, set weather.IconSet, u units.System) string`

> **Why the tier is passed in rather than derived inside `Cell`:** every cell in a row must use the same tier. Deriving per-cell would let one column with a wider temperature silently drop to a narrower tier, producing a ragged grid. The model picks one tier per render from the window width and passes it down.

- [ ] **Step 1: Write the failing test**

Create `internal/widgets/cell_test.go`:

```go
package widgets

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/tsoueid/weatherdash/internal/units"
	"github.com/tsoueid/weatherdash/internal/weather"
)

func sampleDay() weather.Day {
	return weather.Day{Code: 61, HiC: 24, LoC: 12, PrecipProb: 85}
}

// Every cell in a row must be exactly the budgeted width, in every
// tier, in every icon set. A single wrong cell shifts the whole row.
func TestCellWidthIsExact(t *testing.T) {
	sets := []weather.IconSet{weather.IconsASCII, weather.IconsUnicode, weather.IconsNerd}
	for _, set := range sets {
		for _, w := range []int{5, 6, 7, 9, 10, 12, 16} {
			tier := TierFor(w)
			got := Cell(sampleDay(), w, tier, set, units.Metric)
			if lipgloss.Width(got) != w {
				t.Errorf("Cell(w=%d, set=%v) width = %d, want %d (got %q)",
					w, set, lipgloss.Width(got), w, got)
			}
		}
	}
}

func TestTierForThresholds(t *testing.T) {
	tests := []struct {
		w    int
		want Tier
	}{
		{0, TierNone}, {4, TierNone},
		{5, TierMinimal}, {6, TierMinimal},
		{7, TierMedium}, {9, TierMedium},
		{10, TierRich}, {20, TierRich},
	}
	for _, tc := range tests {
		if got := TierFor(tc.w); got != tc.want {
			t.Errorf("TierFor(%d) = %v, want %v", tc.w, got, tc.want)
		}
	}
}

func TestTierMinWidthMatchesTierFor(t *testing.T) {
	for _, tier := range []Tier{TierMinimal, TierMedium, TierRich} {
		if got := TierFor(tier.MinWidth()); got != tier {
			t.Errorf("TierFor(%v.MinWidth()=%d) = %v, want %v",
				tier, tier.MinWidth(), got, tier)
		}
	}
}

func TestCellMinimalShowsHighOnly(t *testing.T) {
	got := Cell(sampleDay(), 5, TierMinimal, weather.IconsASCII, units.Metric)
	if !strings.Contains(got, "24") {
		t.Errorf("minimal cell %q must show the high", got)
	}
	if strings.Contains(got, "12") {
		t.Errorf("minimal cell %q must not show the low", got)
	}
}

func TestCellMediumShowsHighAndLow(t *testing.T) {
	got := Cell(sampleDay(), 9, TierMedium, weather.IconsASCII, units.Metric)
	if !strings.Contains(got, "24") || !strings.Contains(got, "12") {
		t.Errorf("medium cell %q must show both 24 and 12", got)
	}
}

func TestCellRichAddsPrecipGlyph(t *testing.T) {
	got := Cell(sampleDay(), 12, TierRich, weather.IconsASCII, units.Metric)
	if !strings.ContainsRune(got, '█') {
		t.Errorf("rich cell %q must carry a precip glyph for 85%%", got)
	}
}

func TestCellRespectsUnits(t *testing.T) {
	got := Cell(sampleDay(), 12, TierRich, weather.IconsASCII, units.Imperial)
	if !strings.Contains(got, "75") {
		t.Errorf("imperial cell %q must show 75 (24°C)", got)
	}
}

func TestCellTierNoneIsBlank(t *testing.T) {
	got := Cell(sampleDay(), 3, TierNone, weather.IconsASCII, units.Metric)
	if strings.TrimSpace(got) != "" {
		t.Errorf("TierNone cell = %q, want blank", got)
	}
	if lipgloss.Width(got) != 3 {
		t.Errorf("TierNone width = %d, want 3", lipgloss.Width(got))
	}
}

// A city with no data yet must render an empty cell of the right width,
// not a misleading 0°.
func TestCellZeroDayRendersPlaceholder(t *testing.T) {
	got := Cell(weather.Day{}, 9, TierMedium, weather.IconsASCII, units.Metric)
	if lipgloss.Width(got) != 9 {
		t.Errorf("width = %d, want 9", lipgloss.Width(got))
	}
	if strings.Contains(got, "0°") {
		t.Errorf("empty day rendered %q; a zero day must not show 0°", got)
	}
}
```

- [ ] **Step 2: Run it to confirm it fails**

Run: `go test ./internal/widgets/ -run 'TestCell|TestTier'`
Expected: FAIL — `undefined: Tier`

- [ ] **Step 3: Write the implementation**

Create `internal/widgets/cell.go`:

```go
package widgets

import (
	"fmt"
	"strings"

	"github.com/tsoueid/weatherdash/internal/units"
	"github.com/tsoueid/weatherdash/internal/weather"
)

// Tier is how much detail a grid cell shows. Narrow terminals drop to
// lower tiers rather than truncating, because a clipped "24/1" is worse
// than an honest "24".
type Tier int

const (
	// TierNone is too narrow to render anything.
	TierNone Tier = iota
	// TierMinimal is icon plus the daily high: "O24".
	TierMinimal
	// TierMedium adds the low: "O 24/12".
	TierMedium
	// TierRich adds a precipitation glyph: "O 24/12 █".
	TierRich
)

// MinWidth is the narrowest cell that can render this tier.
func (t Tier) MinWidth() int {
	switch t {
	case TierRich:
		return 10
	case TierMedium:
		return 7
	case TierMinimal:
		return 5
	default:
		return 0
	}
}

// TierFor picks the richest tier that fits in cellW columns.
//
// The model calls this once per render and passes the result to every
// cell, so a whole row shares one tier. Deriving it per cell would let
// a column with a three-digit temperature silently drop a tier and make
// the grid ragged.
func TierFor(cellW int) Tier {
	switch {
	case cellW >= TierRich.MinWidth():
		return TierRich
	case cellW >= TierMedium.MinWidth():
		return TierMedium
	case cellW >= TierMinimal.MinWidth():
		return TierMinimal
	default:
		return TierNone
	}
}

// Cell renders one day for one city, padded to exactly width columns.
//
// A zero-valued Day means "no data yet" and renders blank rather than
// 0°, which would be indistinguishable from a genuine freezing day.
func Cell(d weather.Day, width int, tier Tier, set weather.IconSet, u units.System) string {
	if width <= 0 {
		return ""
	}
	if tier == TierNone || d.Date.IsZero() {
		return strings.Repeat(" ", width)
	}

	icon := weather.Icon(d.Code, set)
	hi := u.TempNum(d.HiC)

	var body string
	switch tier {
	case TierMinimal:
		body = fmt.Sprintf("%s%d", icon, hi)
	case TierMedium:
		body = fmt.Sprintf("%s %d/%d", icon, hi, u.TempNum(d.LoC))
	default:
		body = fmt.Sprintf("%s %d/%d %s",
			icon, hi, u.TempNum(d.LoC), string(precipLevel(float64(d.PrecipProb))))
	}
	return Pad(body, width)
}
```

- [ ] **Step 4: Run it to confirm it passes**

Run: `go test ./internal/widgets/ -v -run 'TestCell|TestTier'`
Expected: PASS, 9 tests

- [ ] **Step 5: Commit**

```bash
git add internal/widgets/
git commit -m "feat: add responsive grid cell tiers"
```

---

### Task 15: Wind row and sun arc

**Files:**
- Create: `internal/widgets/wind.go`, `internal/widgets/sunarc.go`, `internal/widgets/wind_test.go`, `internal/widgets/sunarc_test.go`

**Interfaces:**
- Consumes: `widgets.Pad`, `widgets.PadLeft` (Task 12); `weather.Hour` (Task 2); `units.System` (Task 3)
- Produces: `widgets.WindArrow(degrees int) string`, `widgets.WindRow(hours []weather.Hour, width int, u units.System) (arrows, speeds string)`, `widgets.SunArc(rise, set, now time.Time, width int) string`

> **Why an arrow instead of "NNW":** direction is spatial information, and a rotated glyph is read pre-attentively while a compass abbreviation has to be decoded. The arrow points **where the wind is going**, which is the convention weather maps use — the opposite of the meteorological "wind from" reading, so the mapping is rotated 180° deliberately and the test pins it.

- [ ] **Step 1: Write the failing wind test**

Create `internal/widgets/wind_test.go`:

```go
package widgets

import (
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/tsoueid/weatherdash/internal/units"
	"github.com/tsoueid/weatherdash/internal/weather"
)

// Open-Meteo reports the direction the wind blows FROM. The arrow shows
// where it is GOING, matching weather-map convention, so a northerly
// (0°, from the north) points south.
func TestWindArrowDirection(t *testing.T) {
	tests := []struct {
		deg  int
		want string
	}{
		{0, "↓"}, {45, "↙"}, {90, "←"}, {135, "↖"},
		{180, "↑"}, {225, "↗"}, {270, "→"}, {315, "↘"},
		{360, "↓"}, {359, "↓"},
	}
	for _, tc := range tests {
		if got := WindArrow(tc.deg); got != tc.want {
			t.Errorf("WindArrow(%d) = %q, want %q", tc.deg, got, tc.want)
		}
	}
}

func TestWindArrowIsSingleWidth(t *testing.T) {
	for deg := 0; deg < 360; deg += 15 {
		if w := lipgloss.Width(WindArrow(deg)); w != 1 {
			t.Errorf("WindArrow(%d) width = %d, want 1", deg, w)
		}
	}
}

func TestWindArrowHandlesOutOfRange(t *testing.T) {
	for _, deg := range []int{-45, 720, -1} {
		if w := lipgloss.Width(WindArrow(deg)); w != 1 {
			t.Errorf("WindArrow(%d) width = %d, want 1", deg, w)
		}
	}
}

func TestWindRowWidths(t *testing.T) {
	hours := make([]weather.Hour, 24)
	for i := range hours {
		hours[i] = weather.Hour{
			Time:    time.Date(2026, 9, 17, i, 0, 0, 0, time.UTC),
			WindKPH: float64(5 + i),
			WindDir: i * 15,
		}
	}
	for _, w := range []int{20, 40, 60} {
		arrows, speeds := WindRow(hours, w, units.Metric)
		if lipgloss.Width(arrows) != w {
			t.Errorf("arrows width = %d, want %d (%q)", lipgloss.Width(arrows), w, arrows)
		}
		if lipgloss.Width(speeds) != w {
			t.Errorf("speeds width = %d, want %d (%q)", lipgloss.Width(speeds), w, speeds)
		}
	}
}

func TestWindRowEmpty(t *testing.T) {
	arrows, speeds := WindRow(nil, 10, units.Metric)
	if lipgloss.Width(arrows) != 10 || lipgloss.Width(speeds) != 10 {
		t.Errorf("empty wind row widths = %d,%d; want 10,10",
			lipgloss.Width(arrows), lipgloss.Width(speeds))
	}
}
```

- [ ] **Step 2: Run it to confirm it fails**

Run: `go test ./internal/widgets/ -run TestWind`
Expected: FAIL — `undefined: WindArrow`

- [ ] **Step 3: Write the wind implementation**

Create `internal/widgets/wind.go`:

```go
package widgets

import (
	"strings"

	"github.com/tsoueid/weatherdash/internal/units"
	"github.com/tsoueid/weatherdash/internal/weather"
)

// windArrows are indexed by 45° sector, starting at "going north".
var windArrows = []string{"↑", "↗", "→", "↘", "↓", "↙", "←", "↖"}

// WindArrow renders a wind direction as a single-width arrow.
//
// Open-Meteo reports the direction wind blows FROM; the arrow shows
// where it is GOING, which is the convention weather maps use and the
// one people read correctly without a legend. Hence the 180° rotation.
func WindArrow(degrees int) string {
	d := ((degrees+180)%360 + 360) % 360
	idx := int((float64(d)+22.5)/45) % len(windArrows)
	return windArrows[idx]
}

// WindRow renders wind direction and speed as two aligned rows exactly
// width columns wide.
//
// Arrows are sampled rather than averaged: averaging directions across
// a bucket is meaningless when they straddle north (350° and 10° would
// average to 180°, the exact opposite). Speeds are labelled at a
// readable interval instead of on every column.
func WindRow(hours []weather.Hour, width int, u units.System) (arrows, speeds string) {
	if width <= 0 {
		return "", ""
	}
	if len(hours) == 0 {
		blank := strings.Repeat(" ", width)
		return blank, blank
	}

	// One arrow every other column keeps the row from looking like a
	// solid bar while still tracking the shape of the day.
	const arrowEvery = 2

	var ab strings.Builder
	for col := 0; col < width; col++ {
		if col%arrowEvery != 0 {
			ab.WriteByte(' ')
			continue
		}
		idx := col * len(hours) / width
		if idx >= len(hours) {
			idx = len(hours) - 1
		}
		ab.WriteString(WindArrow(hours[idx].WindDir))
	}

	// Speed labels every 8 columns, left-anchored to their sample.
	const labelEvery = 8
	sb := make([]rune, width)
	for i := range sb {
		sb[i] = ' '
	}
	for col := 0; col < width; col += labelEvery {
		idx := col * len(hours) / width
		if idx >= len(hours) {
			idx = len(hours) - 1
		}
		label := []rune(u.Speed(hours[idx].WindKPH))
		for j, r := range label {
			if col+j >= width {
				break
			}
			sb[col+j] = r
		}
	}

	return Pad(ab.String(), width), Pad(string(sb), width)
}
```

- [ ] **Step 4: Write the failing sun arc test**

Create `internal/widgets/sunarc_test.go`:

```go
package widgets

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
)

func TestSunArcWidth(t *testing.T) {
	rise := time.Date(2026, 9, 17, 7, 0, 0, 0, time.UTC)
	set := time.Date(2026, 9, 17, 19, 0, 0, 0, time.UTC)
	now := time.Date(2026, 9, 17, 13, 0, 0, 0, time.UTC)

	for _, w := range []int{10, 20, 40, 60} {
		got := SunArc(rise, set, now, w)
		if lipgloss.Width(got) != w {
			t.Errorf("SunArc width = %d, want %d (%q)", lipgloss.Width(got), w, got)
		}
	}
}

func TestSunArcMarksCurrentPosition(t *testing.T) {
	rise := time.Date(2026, 9, 17, 6, 0, 0, 0, time.UTC)
	set := time.Date(2026, 9, 17, 18, 0, 0, 0, time.UTC)
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC) // exactly midday

	got := SunArc(rise, set, now, 13)
	idx := strings.IndexRune(got, '☉')
	if idx < 0 {
		t.Fatalf("SunArc %q has no sun marker", got)
	}
	// Midday through a 13-column arc lands on the centre column.
	if idx != 6 {
		t.Errorf("sun at column %d, want 6 (%q)", idx, got)
	}
}

// Outside daylight hours there is no position to mark, so the arc
// renders without a marker rather than clamping it to an end.
func TestSunArcOmitsMarkerAtNight(t *testing.T) {
	rise := time.Date(2026, 9, 17, 7, 0, 0, 0, time.UTC)
	set := time.Date(2026, 9, 17, 19, 0, 0, 0, time.UTC)
	night := time.Date(2026, 9, 17, 23, 0, 0, 0, time.UTC)

	got := SunArc(rise, set, night, 20)
	if strings.ContainsRune(got, '☉') {
		t.Errorf("SunArc at night = %q, want no sun marker", got)
	}
	if lipgloss.Width(got) != 20 {
		t.Errorf("width = %d, want 20", lipgloss.Width(got))
	}
}

func TestSunArcZeroTimes(t *testing.T) {
	got := SunArc(time.Time{}, time.Time{}, time.Now(), 10)
	if lipgloss.Width(got) != 10 {
		t.Errorf("width = %d, want 10", lipgloss.Width(got))
	}
}
```

- [ ] **Step 5: Write the sun arc implementation**

Create `internal/widgets/sunarc.go`:

```go
package widgets

import (
	"strings"
	"time"
)

// SunArc renders the daylight span as a track exactly width columns
// wide, marking the sun's current position when it is up.
//
// The marker is omitted rather than clamped outside daylight: pinning
// it to an end would imply the sun is just rising or just setting,
// which is wrong at midnight. An absent marker reads correctly as
// "not up".
func SunArc(rise, set, now time.Time, width int) string {
	if width <= 0 {
		return ""
	}
	if rise.IsZero() || set.IsZero() || !set.After(rise) {
		return strings.Repeat("─", width)
	}

	track := []rune(strings.Repeat("━", width))

	span := set.Sub(rise)
	elapsed := now.Sub(rise)
	if elapsed >= 0 && elapsed <= span {
		pos := int(float64(elapsed) / float64(span) * float64(width-1))
		if pos < 0 {
			pos = 0
		}
		if pos >= width {
			pos = width - 1
		}
		track[pos] = '☉'
	}
	return string(track)
}
```

- [ ] **Step 6: Run it to confirm both pass**

Run: `go test ./internal/widgets/ -v -race`
Expected: PASS, all tests

- [ ] **Step 7: Verify the whole phase**

Run: `make test && make lint`
Expected: PASS, gofmt silent

- [ ] **Step 8: Commit**

```bash
git add internal/widgets/
git commit -m "feat: add wind row and sun arc renderers"
```

---

# Phase 4 — Runnable Grid on Fixture Data

**This phase produces a working application.** By Task 20 the binary launches, draws a real grid, moves a cursor, scrolls, and resizes — entirely on fixture data, with no network code. Everything after this phase adds to something already visible, which is the whole point of ordering it this way.

### Task 16: Key bindings

**Files:**
- Create: `internal/ui/keys.go`, `internal/ui/keys_test.go`

**Interfaces:**
- Consumes: nothing
- Produces: `ui.keyMap` struct with fields `Up`, `Down`, `Left`, `Right`, `Enter`, `Back`, `Add`, `Delete`, `Filter`, `Sort`, `Density`, `Units`, `Refresh`, `MoveUp`, `MoveDown`, `Help`, `Quit` (all `key.Binding`); `ui.defaultKeyMap() keyMap`; `(keyMap).ShortHelp() []key.Binding`; `(keyMap).FullHelp() [][]key.Binding`

> **Why bindings are declared once in a struct rather than matched inline:** the same declarations feed both the `Update` switch and the `bubbles/help` component. Matching on raw key strings inside `Update` would let the footer drift out of sync with the actual behaviour — the classic TUI bug where the help lies.

- [ ] **Step 1: Write the failing test**

Create `internal/ui/keys_test.go`:

```go
package ui

import (
	"testing"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

func TestKeyMapMatchesVimAndArrows(t *testing.T) {
	k := defaultKeyMap()
	tests := []struct {
		name    string
		binding key.Binding
		keys    []string
	}{
		{"up", k.Up, []string{"up", "k"}},
		{"down", k.Down, []string{"down", "j"}},
		{"left", k.Left, []string{"left", "h"}},
		{"right", k.Right, []string{"right", "l"}},
	}
	for _, tc := range tests {
		for _, s := range tc.keys {
			msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
			if len(s) > 1 {
				msg = tea.KeyMsg{Type: keyTypeFor(s)}
			}
			if !key.Matches(msg, tc.binding) {
				t.Errorf("%s binding does not match %q", tc.name, s)
			}
		}
	}
}

func TestDistinctBindingsForAddAndFilter(t *testing.T) {
	k := defaultKeyMap()
	add := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")}
	filter := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")}

	if !key.Matches(add, k.Add) {
		t.Error("a must trigger Add")
	}
	if key.Matches(add, k.Filter) {
		t.Error("a must NOT also trigger Filter — searching and filtering are different operations")
	}
	if !key.Matches(filter, k.Filter) {
		t.Error("/ must trigger Filter")
	}
}

func TestEveryBindingHasHelpText(t *testing.T) {
	k := defaultKeyMap()
	for _, row := range k.FullHelp() {
		for _, b := range row {
			h := b.Help()
			if h.Key == "" || h.Desc == "" {
				t.Errorf("binding %v has incomplete help text %+v", b.Keys(), h)
			}
		}
	}
}

func TestShortHelpIsNonEmpty(t *testing.T) {
	if len(defaultKeyMap().ShortHelp()) == 0 {
		t.Error("ShortHelp must list the footer bindings")
	}
}
```

Add this helper at the bottom of the test file:

```go
// keyTypeFor maps the named keys used in the table to their tea types.
func keyTypeFor(s string) tea.KeyType {
	switch s {
	case "up":
		return tea.KeyUp
	case "down":
		return tea.KeyDown
	case "left":
		return tea.KeyLeft
	case "right":
		return tea.KeyRight
	default:
		return tea.KeyRunes
	}
}
```

- [ ] **Step 2: Run it to confirm it fails**

Run: `go test ./internal/ui/`
Expected: FAIL — `undefined: defaultKeyMap`

- [ ] **Step 3: Write the implementation**

Create `internal/ui/keys.go`:

```go
// Package ui holds the Bubble Tea model. A single shared {city, day}
// cursor drives both the grid and the detail screen, so zooming in and
// out never loses position.
package ui

import "github.com/charmbracelet/bubbles/key"

// keyMap declares every binding once, so the Update switch and the help
// footer read from the same source. Matching raw key strings inside
// Update instead would let the displayed help drift from the actual
// behaviour.
type keyMap struct {
	Up       key.Binding
	Down     key.Binding
	Left     key.Binding
	Right    key.Binding
	Enter    key.Binding
	Back     key.Binding
	Add      key.Binding
	Delete   key.Binding
	Filter   key.Binding
	Sort     key.Binding
	Density  key.Binding
	Units    key.Binding
	Refresh  key.Binding
	MoveUp   key.Binding
	MoveDown key.Binding
	Help     key.Binding
	Quit     key.Binding
}

func defaultKeyMap() keyMap {
	return keyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "city up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "city down"),
		),
		Left: key.NewBinding(
			key.WithKeys("left", "h"),
			key.WithHelp("←/h", "previous day"),
		),
		Right: key.NewBinding(
			key.WithKeys("right", "l"),
			key.WithHelp("→/l", "next day"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("⏎", "zoom in"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),
		// Add and Filter are deliberately separate: searching the world
		// for a new city and narrowing the rows you already have are
		// different operations, and merging them creates mode confusion.
		Add: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "add city"),
		),
		Delete: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "delete city"),
		),
		Filter: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "filter"),
		),
		Sort: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "sort"),
		),
		Density: key.NewBinding(
			key.WithKeys("z"),
			key.WithHelp("z", "density"),
		),
		Units: key.NewBinding(
			key.WithKeys("u"),
			key.WithHelp("u", "units"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		MoveUp: key.NewBinding(
			key.WithKeys("K"),
			key.WithHelp("K", "move city up"),
		),
		MoveDown: key.NewBinding(
			key.WithKeys("J"),
			key.WithHelp("J", "move city down"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
	}
}

// ShortHelp is the one-line footer hint.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Right, k.Enter, k.Add, k.Filter, k.Sort, k.Help, k.Quit}
}

// FullHelp is the ? overlay, grouped by purpose.
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Left, k.Right, k.Enter, k.Back},
		{k.Add, k.Delete, k.Filter, k.Sort, k.MoveUp, k.MoveDown},
		{k.Density, k.Units, k.Refresh, k.Help, k.Quit},
	}
}
```

- [ ] **Step 4: Run it to confirm it passes**

Run: `go test ./internal/ui/ -v`
Expected: PASS, 4 tests

- [ ] **Step 5: Commit**

```bash
git add internal/ui/
git commit -m "feat: add key bindings shared by update and help"
```

---

### Task 17: Model, cursor, and clamping

**Files:**
- Create: `internal/ui/model.go`, `internal/ui/model_test.go`

**Interfaces:**
- Consumes: `keyMap` (Task 16); `weather.City`, `weather.Forecast`, `weather.IconSet` (Tasks 2, 7); `units.System` (Task 3); `store.Config`, `store.Cache` (Tasks 5, 6)
- Produces: `ui.Model` struct; `ui.New(cfg store.Config, cache store.Cache) Model`; `ui.screen` constants `screenGrid`, `screenDetail`, `screenAddCity`, `screenHelp`; `ui.density` constants `densityCompact`, `densityExpanded` with `(density).rowHeight() int`; methods `(*Model).moveCity(delta int)`, `(*Model).moveDay(delta int)`, `(*Model).clampCursor()`, `(Model).selectedCity() (weather.City, bool)`, `(Model).selectedForecast() (weather.Forecast, bool)`, `(Model).dayAt(cityIdx, dayIdx int) (weather.Day, bool)`

> **Why the cursor is two plain ints on the model rather than per-screen state:** the grid and the detail screen are two renderers of one position. Giving each its own selection is how TUIs end up with two cursors that disagree — you zoom into Tokyo/Friday, press escape, and the grid highlights Paris/Monday.

- [ ] **Step 1: Write the failing test**

Create `internal/ui/model_test.go`:

```go
package ui

import (
	"testing"
	"time"

	"github.com/tsoueid/weatherdash/internal/store"
	"github.com/tsoueid/weatherdash/internal/weather"
)

// testModel builds a model with n cities, each with 7 days of data.
func testModel(t *testing.T, n int) Model {
	t.Helper()

	cfg := store.DefaultConfig()
	cache := store.NewCache()
	names := []string{"Paris", "Beirut", "Tokyo", "Oslo", "Lisbon", "Berlin", "Cairo", "Lima"}

	for i := 0; i < n; i++ {
		name := names[i%len(names)]
		id := name + string(rune('0'+i))
		cfg.Cities = append(cfg.Cities, weather.City{
			ID: id, Name: name, Country: "X", TZ: "UTC",
		})

		days := make([]weather.Day, 7)
		for d := range days {
			days[d] = weather.Day{
				Date: time.Date(2026, 9, 17+d, 0, 0, 0, 0, time.UTC),
				HiC:  float64(10 + i + d),
				LoC:  float64(2 + i),
				Code: 0,
			}
		}
		cache.Forecasts[id] = weather.Forecast{CityID: id, Days: days}
	}

	m := New(cfg, cache)
	m.w, m.h = 100, 30
	return m
}

func TestNewLoadsCitiesAndCache(t *testing.T) {
	m := testModel(t, 3)
	if len(m.cities) != 3 {
		t.Fatalf("got %d cities, want 3", len(m.cities))
	}
	if len(m.data) != 3 {
		t.Fatalf("got %d forecasts, want 3", len(m.data))
	}
	if m.screen != screenGrid {
		t.Errorf("screen = %v, want screenGrid", m.screen)
	}
	if m.cursorCity != 0 || m.cursorDay != 0 {
		t.Errorf("cursor = %d,%d; want 0,0", m.cursorCity, m.cursorDay)
	}
}

func TestMoveCityClampsAtBothEnds(t *testing.T) {
	m := testModel(t, 4)

	m.moveCity(-1)
	if m.cursorCity != 0 {
		t.Errorf("moving up from the top gave %d, want 0", m.cursorCity)
	}

	m.moveCity(99)
	if m.cursorCity != 3 {
		t.Errorf("moving past the bottom gave %d, want 3", m.cursorCity)
	}
}

func TestMoveDayClampsAtBothEnds(t *testing.T) {
	m := testModel(t, 2)

	m.moveDay(-5)
	if m.cursorDay != 0 {
		t.Errorf("moving left from day 0 gave %d, want 0", m.cursorDay)
	}

	m.moveDay(99)
	if m.cursorDay != 6 {
		t.Errorf("moving past the last day gave %d, want 6", m.cursorDay)
	}
}

func TestCursorSurvivesEmptyCityList(t *testing.T) {
	m := New(store.DefaultConfig(), store.NewCache())
	m.w, m.h = 100, 30

	m.moveCity(1)
	m.moveDay(1)
	m.clampCursor()

	if m.cursorCity != 0 || m.cursorDay != 0 {
		t.Errorf("cursor on an empty list = %d,%d; want 0,0", m.cursorCity, m.cursorDay)
	}
	if _, ok := m.selectedCity(); ok {
		t.Error("selectedCity must report false with no cities")
	}
}

// Deleting the last city while the cursor is on it must not leave the
// cursor pointing past the end.
func TestClampAfterCityListShrinks(t *testing.T) {
	m := testModel(t, 4)
	m.cursorCity = 3

	m.cities = m.cities[:2]
	m.clampCursor()

	if m.cursorCity != 1 {
		t.Errorf("cursor = %d, want 1 after shrinking to 2 cities", m.cursorCity)
	}
}

func TestSelectedForecastAndDay(t *testing.T) {
	m := testModel(t, 3)
	m.cursorCity, m.cursorDay = 1, 4

	c, ok := m.selectedCity()
	if !ok || c.Name != "Beirut" {
		t.Fatalf("selectedCity = %+v, %v", c, ok)
	}

	f, ok := m.selectedForecast()
	if !ok {
		t.Fatal("selectedForecast reported false")
	}
	if f.CityID != c.ID {
		t.Errorf("forecast CityID = %q, want %q", f.CityID, c.ID)
	}

	d, ok := m.dayAt(1, 4)
	if !ok {
		t.Fatal("dayAt reported false")
	}
	if want := float64(10 + 1 + 4); d.HiC != want {
		t.Errorf("dayAt HiC = %v, want %v", d.HiC, want)
	}
}

// A city with no cached forecast must report cleanly, not panic.
func TestSelectedForecastMissingData(t *testing.T) {
	m := testModel(t, 2)
	m.data = map[string]weather.Forecast{}

	if _, ok := m.selectedForecast(); ok {
		t.Error("selectedForecast must report false when the city has no data")
	}
	if _, ok := m.dayAt(0, 0); ok {
		t.Error("dayAt must report false when the city has no data")
	}
}

func TestDensityRowHeight(t *testing.T) {
	if got := densityCompact.rowHeight(); got != 1 {
		t.Errorf("compact rowHeight = %d, want 1", got)
	}
	if got := densityExpanded.rowHeight(); got != 3 {
		t.Errorf("expanded rowHeight = %d, want 3", got)
	}
}
```

- [ ] **Step 2: Run it to confirm it fails**

Run: `go test ./internal/ui/ -run TestNew`
Expected: FAIL — `undefined: New`

- [ ] **Step 3: Write the implementation**

Create `internal/ui/model.go`:

```go
package ui

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"

	"github.com/tsoueid/weatherdash/internal/store"
	"github.com/tsoueid/weatherdash/internal/units"
	"github.com/tsoueid/weatherdash/internal/weather"
)

// screen is the active view. Screens share one cursor; they are
// renderers of a position, not separate places.
type screen int

const (
	screenGrid screen = iota
	screenDetail
	screenAddCity
	screenHelp
)

// density controls how many terminal lines each city row occupies.
type density int

const (
	// densityCompact is one line per city: icon and high only.
	densityCompact density = iota
	// densityExpanded is three lines: icons, hi/lo, precipitation band.
	densityExpanded
)

// rowHeight is the number of terminal lines one city occupies.
func (d density) rowHeight() int {
	if d == densityExpanded {
		return 3
	}
	return 1
}

func (d density) String() string {
	if d == densityExpanded {
		return "expanded"
	}
	return "compact"
}

func parseDensity(s string) density {
	if s == "expanded" {
		return densityExpanded
	}
	return densityCompact
}

// Model is the complete application state.
type Model struct {
	screen screen

	// cities is the DISPLAY order, which sorting rewrites freely.
	// The saved order lives in cfg.Cities and is never touched by a
	// sort, so manual order is always recoverable.
	cities []weather.City
	data   map[string]weather.Forecast

	// The shared cursor. Both the grid and the detail screen render
	// from these two ints, which is what makes zooming in and out
	// position-preserving.
	cursorCity int
	cursorDay  int

	// scroll is the index of the first visible city row.
	scroll int

	days    int
	density density
	units   units.System
	icons   weather.IconSet

	w, h int

	cfg     store.Config
	keys    keyMap
	help    help.Model
	spinner spinner.Model
	filter  textinput.Model
}

// New builds the initial model from config and whatever the cache holds.
//
// Cache data is loaded here, before the program starts, so the very
// first View() draws a populated grid rather than an empty one. That is
// what makes startup feel instant instead of one network round-trip.
func New(cfg store.Config, cache store.Cache) Model {
	u, err := units.Parse(cfg.Units)
	if err != nil {
		u = units.Metric
	}
	icons, err := weather.ParseIconSet(cfg.Icons)
	if err != nil {
		icons = weather.IconsUnicode
	}

	sp := spinner.New()
	sp.Spinner = spinner.Dot

	fi := textinput.New()
	fi.Placeholder = "filter cities"
	fi.Prompt = "/"

	data := make(map[string]weather.Forecast, len(cache.Forecasts))
	for id, f := range cache.Forecasts {
		data[id] = f
	}

	cities := make([]weather.City, len(cfg.Cities))
	copy(cities, cfg.Cities)

	return Model{
		screen:  screenGrid,
		cities:  cities,
		data:    data,
		days:    cfg.ForecastDays,
		density: parseDensity(cfg.Density),
		units:   u,
		icons:   icons,
		cfg:     cfg,
		keys:    defaultKeyMap(),
		help:    help.New(),
		spinner: sp,
		filter:  fi,
	}
}

// moveCity shifts the city cursor and clamps it.
func (m *Model) moveCity(delta int) {
	m.cursorCity += delta
	m.clampCursor()
}

// moveDay shifts the day cursor and clamps it.
func (m *Model) moveDay(delta int) {
	m.cursorDay += delta
	m.clampCursor()
}

// clampCursor keeps both cursor axes inside the data.
//
// It is called after every mutation of cities or days — not just after
// a keypress — because a delete or a shorter forecast window can strand
// the cursor past the end just as easily as an arrow key can.
func (m *Model) clampCursor() {
	if m.cursorCity < 0 || len(m.cities) == 0 {
		m.cursorCity = 0
	} else if m.cursorCity >= len(m.cities) {
		m.cursorCity = len(m.cities) - 1
	}

	maxDay := m.days - 1
	if f, ok := m.selectedForecast(); ok && len(f.Days) > 0 && len(f.Days)-1 < maxDay {
		maxDay = len(f.Days) - 1
	}
	if maxDay < 0 {
		maxDay = 0
	}
	if m.cursorDay < 0 {
		m.cursorDay = 0
	} else if m.cursorDay > maxDay {
		m.cursorDay = maxDay
	}
}

// selectedCity returns the city under the cursor.
func (m Model) selectedCity() (weather.City, bool) {
	if m.cursorCity < 0 || m.cursorCity >= len(m.cities) {
		return weather.City{}, false
	}
	return m.cities[m.cursorCity], true
}

// selectedForecast returns the forecast for the city under the cursor.
func (m Model) selectedForecast() (weather.Forecast, bool) {
	c, ok := m.selectedCity()
	if !ok {
		return weather.Forecast{}, false
	}
	f, ok := m.data[c.ID]
	return f, ok
}

// dayAt returns one cell of the matrix by its coordinates.
func (m Model) dayAt(cityIdx, dayIdx int) (weather.Day, bool) {
	if cityIdx < 0 || cityIdx >= len(m.cities) {
		return weather.Day{}, false
	}
	f, ok := m.data[m.cities[cityIdx].ID]
	if !ok || dayIdx < 0 || dayIdx >= len(f.Days) {
		return weather.Day{}, false
	}
	return f.Days[dayIdx], true
}
```

- [ ] **Step 4: Run it to confirm it passes**

Run: `go test ./internal/ui/ -v -race`
Expected: PASS, 12 tests

- [ ] **Step 5: Commit**

```bash
git add internal/ui/
git commit -m "feat: add UI model with shared cursor and clamping"
```

---

### Task 18: Responsive layout and scrolling

**Files:**
- Create: `internal/ui/layout.go`, `internal/ui/layout_test.go`

**Interfaces:**
- Consumes: `Model` (Task 17); `widgets.Tier`, `widgets.TierFor` (Task 14)
- Produces: `ui.layout` struct with fields `labelW`, `days`, `cellW`, `tier`, `rows`; `(Model).layout() layout`; `(Model).tooSmall() bool`; `(*Model).ensureCursorVisible()`; constants `minTermWidth`, `minTermHeight`

> **Why columns are dropped rather than narrowed past `TierMinimal`:** a cell too narrow for its content truncates to something actively misleading — "24/1" instead of "24/12". Showing five honest days beats seven lying ones. The loop drops days from the right until the remaining cells fit.

- [ ] **Step 1: Write the failing test**

Create `internal/ui/layout_test.go`:

```go
package ui

import (
	"testing"

	"github.com/tsoueid/weatherdash/internal/widgets"
)

func TestLayoutTiersByWidth(t *testing.T) {
	tests := []struct {
		w        int
		wantTier widgets.Tier
	}{
		{120, widgets.TierRich},
		{100, widgets.TierRich},
		{80, widgets.TierMedium},
		{60, widgets.TierMinimal},
	}
	for _, tc := range tests {
		m := testModel(t, 3)
		m.w = tc.w
		if got := m.layout().tier; got != tc.wantTier {
			t.Errorf("width %d: tier = %v, want %v (cellW %d)",
				tc.w, got, tc.wantTier, m.layout().cellW)
		}
	}
}

// With a long forecast window there is not enough width for every day,
// so columns are dropped rather than truncated into nonsense.
func TestLayoutDropsDaysWhenTooNarrow(t *testing.T) {
	m := testModel(t, 3)
	m.days = 16
	m.w = 80

	l := m.layout()
	if l.days >= 16 {
		t.Errorf("days = %d, want fewer than 16 at width 80", l.days)
	}
	if l.cellW < widgets.TierMinimal.MinWidth() {
		t.Errorf("cellW = %d, want at least %d after dropping days",
			l.cellW, widgets.TierMinimal.MinWidth())
	}
	if l.days < 3 {
		t.Errorf("days = %d, want at least the 3-day floor", l.days)
	}
}

func TestLayoutRowsRespectDensity(t *testing.T) {
	m := testModel(t, 20)
	m.h = 24

	compact := m.layout().rows
	m.density = densityExpanded
	expanded := m.layout().rows

	if expanded >= compact {
		t.Errorf("expanded rows = %d, compact = %d; expanded must fit fewer",
			expanded, compact)
	}
	if expanded < 1 {
		t.Errorf("expanded rows = %d, want at least 1", expanded)
	}
}

func TestTooSmall(t *testing.T) {
	m := testModel(t, 3)

	m.w, m.h = 100, 30
	if m.tooSmall() {
		t.Error("100x30 must not be too small")
	}
	m.w, m.h = 40, 30
	if !m.tooSmall() {
		t.Error("width 40 must be too small")
	}
	m.w, m.h = 100, 8
	if !m.tooSmall() {
		t.Error("height 8 must be too small")
	}
}

func TestEnsureCursorVisibleScrollsDown(t *testing.T) {
	m := testModel(t, 20)
	m.w, m.h = 100, 14 // a small viewport so scrolling is forced

	rows := m.layout().rows
	m.cursorCity = 19
	m.ensureCursorVisible()

	if m.cursorCity < m.scroll || m.cursorCity >= m.scroll+rows {
		t.Errorf("cursor %d outside window [%d,%d)",
			m.cursorCity, m.scroll, m.scroll+rows)
	}
}

func TestEnsureCursorVisibleScrollsUp(t *testing.T) {
	m := testModel(t, 20)
	m.w, m.h = 100, 14
	m.scroll = 10
	m.cursorCity = 0
	m.ensureCursorVisible()

	if m.scroll != 0 {
		t.Errorf("scroll = %d, want 0 when the cursor is at the top", m.scroll)
	}
}

func TestScrollNeverExceedsContent(t *testing.T) {
	m := testModel(t, 4)
	m.w, m.h = 100, 30
	m.scroll = 99
	m.cursorCity = 0
	m.ensureCursorVisible()

	if m.scroll != 0 {
		t.Errorf("scroll = %d, want 0 when all rows fit", m.scroll)
	}
}

func TestEnsureCursorVisibleEmptyList(t *testing.T) {
	m := New(defaultTestConfig(), emptyTestCache())
	m.w, m.h = 100, 30
	m.ensureCursorVisible() // must not panic or divide by zero

	if m.scroll != 0 {
		t.Errorf("scroll = %d, want 0", m.scroll)
	}
}
```

Add these helpers to `model_test.go`:

```go
func defaultTestConfig() store.Config { return store.DefaultConfig() }
func emptyTestCache() store.Cache     { return store.NewCache() }
```

- [ ] **Step 2: Run it to confirm it fails**

Run: `go test ./internal/ui/ -run TestLayout`
Expected: FAIL — `m.layout undefined`

- [ ] **Step 3: Write the implementation**

Create `internal/ui/layout.go`:

```go
package ui

import "github.com/tsoueid/weatherdash/internal/widgets"

const (
	// minTermWidth is the narrowest terminal the matrix is usable in.
	// Below this the grid is replaced by a message rather than drawn
	// as garbage.
	minTermWidth  = 50
	minTermHeight = 15

	// labelWidth is the city-name column.
	labelWidth = 12

	// gridChromeLines is the day header plus the footer.
	gridChromeLines = 3

	// minDayColumns is the fewest day columns worth showing. Below
	// three the matrix stops being a comparison view.
	minDayColumns = 3

	// scrollOff keeps this many rows visible past the cursor, so the
	// selected city is never flush against the edge.
	scrollOff = 1
)

// layout is the geometry for one render, derived from the window size.
// It is computed once per frame and passed down, so every cell in the
// grid shares one tier and one width.
type layout struct {
	labelW int
	days   int
	cellW  int
	tier   widgets.Tier
	rows   int
}

// tooSmall reports whether the terminal is below the usable minimum.
func (m Model) tooSmall() bool {
	return m.w < minTermWidth || m.h < minTermHeight
}

// layout computes the grid geometry for the current window size.
//
// When the configured forecast window does not fit, day columns are
// dropped from the right rather than narrowed past TierMinimal: a cell
// too small for its content truncates "24/12" into "24/1", which is
// worse than not showing the day at all.
func (m Model) layout() layout {
	days := m.days
	if days < 1 {
		days = 1
	}

	avail := m.w - labelWidth
	if avail < 0 {
		avail = 0
	}

	cellW := 0
	for days >= minDayColumns {
		cellW = avail / days
		if cellW >= widgets.TierMinimal.MinWidth() {
			break
		}
		days--
	}
	if days < minDayColumns {
		days = minDayColumns
		cellW = avail / days
	}

	rows := (m.h - gridChromeLines) / m.density.rowHeight()
	if rows < 1 {
		rows = 1
	}

	return layout{
		labelW: labelWidth,
		days:   days,
		cellW:  cellW,
		tier:   widgets.TierFor(cellW),
		rows:   rows,
	}
}

// ensureCursorVisible adjusts scroll so the selected city stays in the
// viewport, keeping scrollOff rows of context where possible.
func (m *Model) ensureCursorVisible() {
	if len(m.cities) == 0 {
		m.scroll = 0
		return
	}

	rows := m.layout().rows

	// All rows fit: never scroll.
	if len(m.cities) <= rows {
		m.scroll = 0
		return
	}

	top := m.cursorCity - scrollOff
	if m.scroll > top {
		m.scroll = top
	}
	bottom := m.cursorCity - rows + 1 + scrollOff
	if m.scroll < bottom {
		m.scroll = bottom
	}

	if max := len(m.cities) - rows; m.scroll > max {
		m.scroll = max
	}
	if m.scroll < 0 {
		m.scroll = 0
	}
}
```

- [ ] **Step 4: Run it to confirm it passes**

Run: `go test ./internal/ui/ -v -race`
Expected: PASS, all tests

- [ ] **Step 5: Commit**

```bash
git add internal/ui/
git commit -m "feat: add responsive grid layout and cursor scrolling"
```

---

### Task 19: Theme and grid renderer

**Files:**
- Create: `internal/ui/theme.go`, `internal/ui/grid.go`, `internal/ui/grid_test.go`

**Interfaces:**
- Consumes: `layout` (Task 18); `Model` (Task 17); `widgets.Cell`, `widgets.Pad`, `widgets.PrecipBand` (Tasks 12-14); `weather.Icon` (Task 7)
- Produces: `ui.theme` struct with styles `header`, `label`, `labelSel`, `cursor`, `dim`, `warn`; `ui.newTheme() theme`; `ui.tempColor(c float64) lipgloss.Color`; `(Model).renderGrid() string`; `(Model).dayHeader(l layout) string`; `(Model).cityRow(idx int, l layout) []string`

> **Why the column header is labelled from the user's own clock:** each city's day 0 is its own local today, so no single weekday label is literally correct for every row at once. Labelling from the viewer's local day matches the planning frame ("what is Friday like *for me*, over there") and the detail screen shows the exact local date for the selected city, which is where precision actually matters.

- [ ] **Step 1: Write the failing test**

Create `internal/ui/grid_test.go`:

```go
package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// Every line of the grid must be exactly the terminal width, or the
// right-hand border tears.
func TestRenderGridLineWidths(t *testing.T) {
	for _, size := range []struct{ w, h int }{
		{80, 24}, {120, 40}, {60, 20}, {100, 16},
	} {
		m := testModel(t, 8)
		m.w, m.h = size.w, size.h

		out := m.renderGrid()
		for i, line := range strings.Split(out, "\n") {
			if got := lipgloss.Width(line); got != size.w {
				t.Errorf("%dx%d line %d width = %d, want %d: %q",
					size.w, size.h, i, got, size.w, line)
			}
		}
	}
}

func TestRenderGridShowsCityNames(t *testing.T) {
	m := testModel(t, 3)
	out := m.renderGrid()

	for _, want := range []string{"Paris", "Beirut", "Tokyo"} {
		if !strings.Contains(out, want) {
			t.Errorf("grid is missing %q:\n%s", want, out)
		}
	}
}

func TestRenderGridHeaderLabelsToday(t *testing.T) {
	m := testModel(t, 2)
	if !strings.Contains(m.renderGrid(), "Today") {
		t.Errorf("grid header must label the first column Today:\n%s", m.renderGrid())
	}
}

func TestRenderGridScrollsToWindow(t *testing.T) {
	m := testModel(t, 20)
	m.w, m.h = 100, 14
	m.cursorCity = 19
	m.ensureCursorVisible()

	out := m.renderGrid()
	if strings.Contains(out, "Paris0") {
		t.Error("scrolled grid must not show the first city")
	}
}

func TestExpandedDensityAddsRows(t *testing.T) {
	m := testModel(t, 3)
	m.w, m.h = 120, 40

	compact := len(strings.Split(m.renderGrid(), "\n"))
	m.density = densityExpanded
	expanded := len(strings.Split(m.renderGrid(), "\n"))

	if expanded <= compact {
		t.Errorf("expanded grid has %d lines, compact %d; expanded must be taller",
			expanded, compact)
	}
}

func TestRenderGridEmptyCityList(t *testing.T) {
	m := New(defaultTestConfig(), emptyTestCache())
	m.w, m.h = 100, 30

	out := m.renderGrid()
	if strings.TrimSpace(out) == "" {
		t.Error("an empty city list must still render chrome, not nothing")
	}
	for i, line := range strings.Split(out, "\n") {
		if got := lipgloss.Width(line); got != 100 {
			t.Errorf("line %d width = %d, want 100", i, got)
		}
	}
}

func TestTempColorRampIsMonotonic(t *testing.T) {
	// Distinct temperatures across the range must not all collapse to
	// one colour, or the grid stops being scannable.
	seen := map[lipgloss.Color]bool{}
	for _, c := range []float64{-20, -5, 5, 15, 25, 35, 45} {
		seen[tempColor(c)] = true
	}
	if len(seen) < 5 {
		t.Errorf("temp ramp produced only %d distinct colours, want at least 5", len(seen))
	}
}
```

- [ ] **Step 2: Run it to confirm it fails**

Run: `go test ./internal/ui/ -run TestRenderGrid`
Expected: FAIL — `m.renderGrid undefined`

- [ ] **Step 3: Write the theme**

Create `internal/ui/theme.go`:

```go
package ui

import "github.com/charmbracelet/lipgloss"

// theme holds the styles used across screens. Styles live here rather
// than inline so the palette can be changed in one place.
type theme struct {
	header   lipgloss.Style
	label    lipgloss.Style
	labelSel lipgloss.Style
	cursor   lipgloss.Style
	dim      lipgloss.Style
	warn     lipgloss.Style
}

func newTheme() theme {
	return theme{
		header:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("252")),
		label:    lipgloss.NewStyle().Foreground(lipgloss.Color("250")),
		labelSel: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")),
		cursor:   lipgloss.NewStyle().Reverse(true),
		dim:      lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		warn:     lipgloss.NewStyle().Foreground(lipgloss.Color("214")),
	}
}

// tempStop is one anchor of the temperature colour ramp.
type tempStop struct {
	c     float64
	color lipgloss.Color
}

// tempRamp runs cold blue through to hot red. The stops are spaced by
// perceived temperature rather than evenly, so the interesting band
// (roughly 0-30°C, where most weather people care about lives) gets
// most of the resolution.
var tempRamp = []tempStop{
	{-20, lipgloss.Color("57")},  // deep blue
	{-5, lipgloss.Color("39")},   // blue
	{5, lipgloss.Color("51")},    // cyan
	{12, lipgloss.Color("42")},   // green
	{18, lipgloss.Color("227")},  // yellow
	{25, lipgloss.Color("214")},  // orange
	{32, lipgloss.Color("202")},  // deep orange
	{40, lipgloss.Color("196")},  // red
}

// tempColor maps a Celsius value to a ramp colour.
//
// Colour is what makes the matrix scannable before it is readable: the
// eye finds the warm row without parsing any digits. It is applied to
// the temperature text only, never as a cell background, so the grid
// stays legible in terminals with unusual palettes.
func tempColor(c float64) lipgloss.Color {
	if c <= tempRamp[0].c {
		return tempRamp[0].color
	}
	for i := 1; i < len(tempRamp); i++ {
		if c <= tempRamp[i].c {
			return tempRamp[i].color
		}
	}
	return tempRamp[len(tempRamp)-1].color
}
```

- [ ] **Step 4: Write the grid renderer**

Create `internal/ui/grid.go`:

```go
package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/tsoueid/weatherdash/internal/weather"
	"github.com/tsoueid/weatherdash/internal/widgets"
)

// renderGrid draws the matrix screen: a pinned day header above a
// scrolling band of city rows.
//
// The header renders outside the scroll region so the day labels stay
// visible at any scroll position — with twenty cities, a header that
// scrolls away makes the grid unreadable.
func (m Model) renderGrid() string {
	l := m.layout()
	th := newTheme()

	lines := make([]string, 0, m.h)
	lines = append(lines, m.dayHeader(l))
	lines = append(lines, widgets.Pad(strings.Repeat("─", m.w), m.w))

	body := 0
	for i := m.scroll; i < len(m.cities) && body < l.rows; i++ {
		lines = append(lines, m.cityRow(i, l)...)
		body++
	}

	// Pad the body so the footer always sits on the same line.
	used := len(lines)
	for used < m.h-1 {
		lines = append(lines, strings.Repeat(" ", m.w))
		used++
	}

	lines = append(lines, th.dim.Render(widgets.Pad(m.footerHint(), m.w)))
	return strings.Join(lines[:m.h], "\n")
}

// footerHint is the one-line key reminder. Task 23 replaces this with a
// version that also reports data freshness.
func (m Model) footerHint() string {
	return " ↑↓ city · ←→ day · ⏎ zoom · a add · / filter · s sort · r refresh · ? help"
}

// dayHeader renders the pinned column labels.
//
// Labels come from the viewer's local clock. No single weekday label
// can be literally correct for every row at once, because each city's
// day 0 is its own local today; the viewer's frame is the one that
// matches how people plan, and the detail screen shows the selected
// city's exact local date where precision matters.
func (m Model) dayHeader(l layout) string {
	th := newTheme()
	now := time.Now()

	var b strings.Builder
	b.WriteString(strings.Repeat(" ", l.labelW))
	for d := 0; d < l.days; d++ {
		label := "Today"
		if d > 0 {
			label = now.AddDate(0, 0, d).Format("Mon 2")
		}
		b.WriteString(widgets.Pad(label, l.cellW))
	}
	return th.header.Render(widgets.Pad(b.String(), m.w))
}

// cityRow renders one city as one line (compact) or three (expanded).
func (m Model) cityRow(idx int, l layout) []string {
	th := newTheme()
	city := m.cities[idx]
	f, hasData := m.data[city.ID]

	labelStyle := th.label
	if idx == m.cursorCity {
		labelStyle = th.labelSel
	}
	label := labelStyle.Render(widgets.Pad(city.Name, l.labelW))

	if m.density == densityCompact {
		return []string{widgets.Pad(label+m.cellsFor(idx, f, hasData, l), m.w)}
	}

	// Expanded: icons, temperatures, precipitation band.
	var icons, temps, precip strings.Builder
	icons.WriteString(widgets.Pad(city.Name, l.labelW))
	temps.WriteString(widgets.Pad(strings.ToUpper(city.CC), l.labelW))
	precip.WriteString(strings.Repeat(" ", l.labelW))

	for d := 0; d < l.days; d++ {
		day, ok := m.dayAt(idx, d)
		if !ok {
			icons.WriteString(strings.Repeat(" ", l.cellW))
			temps.WriteString(strings.Repeat(" ", l.cellW))
			precip.WriteString(strings.Repeat(" ", l.cellW))
			continue
		}
		icons.WriteString(widgets.Pad(weather.Icon(day.Code, m.icons), l.cellW))
		temps.WriteString(widgets.Pad(
			fmt.Sprintf("%d/%d", m.units.TempNum(day.HiC), m.units.TempNum(day.LoC)),
			l.cellW))
		precip.WriteString(widgets.Pad(
			widgets.PrecipBand([]int{day.PrecipProb}, 2), l.cellW))
	}

	return []string{
		widgets.Pad(labelStyle.Render(icons.String()), m.w),
		widgets.Pad(temps.String(), m.w),
		widgets.Pad(th.dim.Render(precip.String()), m.w),
	}
}

// cellsFor renders the day cells of one compact row, highlighting the
// cell under the cursor.
func (m Model) cellsFor(idx int, f weather.Forecast, hasData bool, l layout) string {
	th := newTheme()
	var b strings.Builder

	for d := 0; d < l.days; d++ {
		var cell string
		if !hasData || d >= len(f.Days) {
			cell = strings.Repeat(" ", l.cellW)
		} else {
			day := f.Days[d]
			cell = widgets.Cell(day, l.cellW, l.tier, m.icons, m.units)
			cell = lipglossColorTemp(cell, day.HiC)
		}
		if idx == m.cursorCity && d == m.cursorDay {
			cell = th.cursor.Render(cell)
		}
		b.WriteString(cell)
	}
	return b.String()
}

// lipglossColorTemp tints a rendered cell by its daily high. Colour is
// applied to the whole cell string rather than to the digits alone
// because splitting a styled string by character would break the width
// accounting the grid depends on.
func lipglossColorTemp(cell string, hiC float64) string {
	return lipgloss.NewStyle().Foreground(tempColor(hiC)).Render(cell)
}
```

Add `"github.com/charmbracelet/lipgloss"` to the import block.

- [ ] **Step 5: Run it to confirm it passes**

Run: `go test ./internal/ui/ -v -race`
Expected: PASS. **If a width assertion fails, the bug is in the renderer, not the test** — find the line that pads with `len` or forgets a `Pad` call.

- [ ] **Step 6: Commit**

```bash
git add internal/ui/
git commit -m "feat: add theme, temperature ramp, and grid renderer"
```

---

### Task 20: Update loop, View, and a runnable binary

**Files:**
- Create: `internal/ui/update.go`, `internal/ui/view.go`, `internal/ui/demo.go`, `internal/ui/update_test.go`
- Modify: `cmd/weatherdash/main.go` (replace the placeholder)

**Interfaces:**
- Consumes: everything from Tasks 16-19
- Produces: `(Model).Init() tea.Cmd`, `(Model).Update(tea.Msg) (tea.Model, tea.Cmd)`, `(Model).View() string`, `ui.DemoModel() Model`

> **This is the milestone.** After this task `./weatherdash --demo` launches a real grid you can drive with the keyboard, resize, and scroll — with no network code in the binary. Every later phase modifies something you can already see, which is what keeps the remaining work verifiable at each step instead of only at the end.

- [ ] **Step 1: Write the failing test**

Create `internal/ui/update_test.go`:

```go
package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func keyMsg(s string) tea.KeyMsg {
	switch s {
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

// send applies a sequence of keys and returns the resulting model.
func send(m Model, keys ...string) Model {
	for _, k := range keys {
		next, _ := m.Update(keyMsg(k))
		m = next.(Model)
	}
	return m
}

func TestUpdateMovesCursor(t *testing.T) {
	m := send(testModel(t, 5), "down", "down", "right")

	if m.cursorCity != 2 {
		t.Errorf("cursorCity = %d, want 2", m.cursorCity)
	}
	if m.cursorDay != 1 {
		t.Errorf("cursorDay = %d, want 1", m.cursorDay)
	}
}

func TestUpdateVimKeysMatchArrows(t *testing.T) {
	arrows := send(testModel(t, 5), "down", "down", "right")
	vim := send(testModel(t, 5), "j", "j", "l")

	if arrows.cursorCity != vim.cursorCity || arrows.cursorDay != vim.cursorDay {
		t.Errorf("vim gave %d,%d; arrows gave %d,%d",
			vim.cursorCity, vim.cursorDay, arrows.cursorCity, arrows.cursorDay)
	}
}

func TestUpdateClampsAtEdges(t *testing.T) {
	m := send(testModel(t, 3), "up", "up", "left", "left")
	if m.cursorCity != 0 || m.cursorDay != 0 {
		t.Errorf("cursor = %d,%d; want 0,0", m.cursorCity, m.cursorDay)
	}
}

func TestUpdateWindowSizeUpdatesLayout(t *testing.T) {
	m := testModel(t, 3)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 50})
	m = next.(Model)

	if m.w != 140 || m.h != 50 {
		t.Errorf("size = %dx%d, want 140x50", m.w, m.h)
	}
}

// Shrinking the window must not strand the cursor outside the viewport.
func TestUpdateResizeKeepsCursorVisible(t *testing.T) {
	m := testModel(t, 20)
	m.w, m.h = 100, 40
	m.cursorCity = 19
	m.ensureCursorVisible()

	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 16})
	m = next.(Model)

	rows := m.layout().rows
	if m.cursorCity < m.scroll || m.cursorCity >= m.scroll+rows {
		t.Errorf("after resize cursor %d is outside [%d,%d)",
			m.cursorCity, m.scroll, m.scroll+rows)
	}
}

func TestUpdateQuitReturnsQuitCmd(t *testing.T) {
	m := testModel(t, 3)
	_, cmd := m.Update(keyMsg("q"))
	if cmd == nil {
		t.Fatal("q must return a command")
	}
	if msg := cmd(); msg == nil {
		t.Fatal("q must produce a quit message")
	}
}

func TestViewTooSmall(t *testing.T) {
	m := testModel(t, 3)
	m.w, m.h = 30, 10

	out := m.View()
	if !strings.Contains(strings.ToLower(out), "too small") {
		t.Errorf("small terminal view = %q, want a 'too small' message", out)
	}
}

func TestViewRendersGridAtNormalSize(t *testing.T) {
	m := testModel(t, 4)
	if !strings.Contains(m.View(), "Paris") {
		t.Error("View must render the grid at a normal size")
	}
}

// View must never panic before the first WindowSizeMsg arrives, when
// width and height are still zero.
func TestViewBeforeFirstWindowSize(t *testing.T) {
	m := New(defaultTestConfig(), emptyTestCache())
	if out := m.View(); out == "" {
		t.Error("View at zero size must render something, not empty")
	}
}

func TestDemoModelHasData(t *testing.T) {
	m := DemoModel()
	if len(m.cities) < 5 {
		t.Errorf("demo has %d cities, want at least 5", len(m.cities))
	}
	for _, c := range m.cities {
		if _, ok := m.data[c.ID]; !ok {
			t.Errorf("demo city %q has no forecast", c.Name)
		}
	}
}
```

- [ ] **Step 2: Run it to confirm it fails**

Run: `go test ./internal/ui/ -run TestUpdate`
Expected: FAIL — `m.Update undefined`

- [ ] **Step 3: Write the update loop**

Create `internal/ui/update.go`:

```go
package ui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// Init starts the spinner. Phase 5 adds the initial fetch and the
// auto-refresh ticker here.
func (m Model) Init() tea.Cmd {
	return m.spinner.Tick
}

// Update is a pure function of (model, message). No I/O happens here —
// network work is returned as a tea.Cmd and arrives back as a message,
// which is what makes the entire loop testable without mocks.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		m.help.Width = msg.Width
		// A resize changes how many rows fit, which can strand the
		// cursor outside the viewport.
		m.ensureCursorVisible()
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

// handleKey routes a keypress to the active screen. Phases 6 to 8 add
// the other screens' branches.
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keys.Quit) {
		return m, tea.Quit
	}

	switch m.screen {
	case screenGrid:
		return m.handleGridKey(msg)
	}
	return m, nil
}

func (m Model) handleGridKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Up):
		m.moveCity(-1)
		m.ensureCursorVisible()
	case key.Matches(msg, m.keys.Down):
		m.moveCity(1)
		m.ensureCursorVisible()
	case key.Matches(msg, m.keys.Left):
		m.moveDay(-1)
	case key.Matches(msg, m.keys.Right):
		m.moveDay(1)
	}
	return m, nil
}
```

- [ ] **Step 4: Write the view dispatcher**

Create `internal/ui/view.go`:

```go
package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// View dispatches to the active screen's renderer.
//
// It renders a message rather than a grid when the terminal is too
// small, because a matrix squeezed below its minimum produces torn
// borders that look like a crash.
func (m Model) View() string {
	if m.w == 0 || m.h == 0 {
		// Before the first WindowSizeMsg there is no geometry to render
		// against. Returning empty would flash a blank frame.
		return "Loading…"
	}
	if m.tooSmall() {
		return lipgloss.Place(m.w, m.h, lipgloss.Center, lipgloss.Center,
			fmt.Sprintf("Terminal too small\nneed at least %d×%d, have %d×%d",
				minTermWidth, minTermHeight, m.w, m.h))
	}

	switch m.screen {
	case screenGrid:
		return m.renderGrid()
	}
	return m.renderGrid()
}
```

- [ ] **Step 5: Write the demo fixture**

Create `internal/ui/demo.go`:

```go
package ui

import (
	"math"
	"time"

	"github.com/tsoueid/weatherdash/internal/store"
	"github.com/tsoueid/weatherdash/internal/weather"
)

// demoCity is one synthetic location plus the shape of its weather.
type demoCity struct {
	name, cc, country, tz string
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
		{"Paris", "FR", "France", "Europe/Paris", 16, 3},
		{"Beirut", "LB", "Lebanon", "Asia/Beirut", 27, 0},
		{"Tokyo", "JP", "Japan", "Asia/Tokyo", 12, 61},
		{"Oslo", "NO", "Norway", "Europe/Oslo", 4, 71},
		{"Lisbon", "PT", "Portugal", "Europe/Lisbon", 23, 0},
		{"Berlin", "DE", "Germany", "Europe/Berlin", 15, 3},
		{"Cairo", "EG", "Egypt", "Africa/Cairo", 31, 0},
		{"Reykjavík", "IS", "Iceland", "Atlantic/Reykjavik", 6, 51},
	}

	cfg := store.DefaultConfig()
	cache := store.NewCache()
	start := time.Now().Truncate(24 * time.Hour)

	for i, d := range demo {
		city := weather.City{
			ID: "demo-" + d.name, Name: d.name, CC: d.cc,
			Country: d.country, TZ: d.tz,
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

	return New(cfg, cache)
}
```

- [ ] **Step 6: Replace `cmd/weatherdash/main.go`**

```go
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

	"github.com/tsoueid/weatherdash/internal/store"
	"github.com/tsoueid/weatherdash/internal/ui"
	"github.com/tsoueid/weatherdash/internal/version"
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
	cfg, _, err := store.LoadConfig(cfgPath)
	if err != nil {
		return ui.Model{}, err
	}

	cachePath, err := store.CachePath()
	if err != nil {
		return ui.Model{}, err
	}

	// LoadCache never fails: a missing or corrupt cache is an empty
	// one, and the first fetch repopulates it.
	return ui.New(cfg, store.LoadCache(cachePath)), nil
}
```

- [ ] **Step 7: Run the tests**

Run: `go test ./... -race`
Expected: PASS, every package

- [ ] **Step 8: Run the application — the milestone check**

Run: `make build && ./weatherdash --demo`

Verify by hand:
1. A grid of eight cities with seven day columns renders.
2. `j`/`k` and `↑`/`↓` move the highlight down and up the city column.
3. `h`/`l` and `←`/`→` move it across the day columns.
4. The highlight stops at all four edges instead of wrapping or panicking.
5. Resizing the terminal reflows the grid; narrowing it drops to a
   simpler cell tier, and going very narrow shows the "too small"
   message rather than torn output.
6. `q` exits cleanly and restores the terminal.

- [ ] **Step 9: Commit**

```bash
git add .
git commit -m "feat: runnable grid TUI with demo data"
```

---

# Phase 5 — Live Data and Cache-First Startup

### Task 21: Fetch command with request-ID discipline

**Files:**
- Create: `internal/ui/msgs.go`, `internal/ui/commands.go`, `internal/ui/commands_test.go`
- Modify: `internal/ui/model.go` (add dependency fields and `WithDeps`)

**Interfaces:**
- Consumes: `weather.Provider` (Task 2), `geocode.Client` (Task 11), `Model` (Task 17)
- Produces: `ui.Deps` struct with fields `Provider`, `Geocoder`, `ConfigPath`, `CachePath`; `(Model).WithDeps(Deps) Model`; message types `forecastsMsg` (fields `reqID`, `cityIDs`, `forecasts`, `err`), `tickMsg`; `ui.fetchCmd(p weather.Provider, cities []weather.City, reqID int) tea.Cmd`

> **The closure trap this task exists to avoid.** `fetchCmd` runs on another goroutine, after `Update` has returned. If it read `m.cities` from inside the closure it would see whatever the list is *when the request finishes*, not when it started — so deleting a city mid-refresh would shift every result by one and put Oslo's weather under Beirut. Copying the slice into the closure makes the mapping immutable for the request's lifetime.

- [ ] **Step 1: Write the failing test**

Create `internal/ui/commands_test.go`:

```go
package ui

import (
	"context"
	"errors"
	"testing"

	"github.com/tsoueid/weatherdash/internal/weather"
)

// fakeProvider records what it was asked for and returns canned data.
type fakeProvider struct {
	seen []weather.City
	err  error
}

func (f *fakeProvider) FetchAll(_ context.Context, cities []weather.City) ([]weather.Forecast, error) {
	f.seen = append([]weather.City(nil), cities...)
	if f.err != nil {
		return nil, f.err
	}
	out := make([]weather.Forecast, len(cities))
	for i, c := range cities {
		out[i] = weather.Forecast{
			CityID: c.ID,
			Days:   []weather.Day{{HiC: float64(20 + i)}},
		}
	}
	return out, nil
}

func TestFetchCmdCarriesReqIDAndIDs(t *testing.T) {
	p := &fakeProvider{}
	cities := []weather.City{{ID: "a"}, {ID: "b"}}

	msg := fetchCmd(p, cities, 7)().(forecastsMsg)

	if msg.reqID != 7 {
		t.Errorf("reqID = %d, want 7", msg.reqID)
	}
	if len(msg.cityIDs) != 2 || msg.cityIDs[0] != "a" || msg.cityIDs[1] != "b" {
		t.Errorf("cityIDs = %v, want [a b]", msg.cityIDs)
	}
	if len(msg.forecasts) != 2 {
		t.Errorf("got %d forecasts, want 2", len(msg.forecasts))
	}
}

// The command must operate on a snapshot. Mutating the caller's slice
// after fetchCmd is built must not change what the request sends.
func TestFetchCmdSnapshotsTheCityList(t *testing.T) {
	p := &fakeProvider{}
	cities := []weather.City{{ID: "a"}, {ID: "b"}, {ID: "c"}}

	cmd := fetchCmd(p, cities, 1)

	// Simulate the user deleting a city before the response lands.
	cities[0] = weather.City{ID: "MUTATED"}
	cities = cities[:1]
	_ = cities

	msg := cmd().(forecastsMsg)

	if len(p.seen) != 3 {
		t.Fatalf("provider saw %d cities, want the 3 from the snapshot", len(p.seen))
	}
	if p.seen[0].ID != "a" {
		t.Errorf("provider saw %q, want the snapshotted \"a\"", p.seen[0].ID)
	}
	if len(msg.cityIDs) != 3 {
		t.Errorf("cityIDs = %v, want 3 snapshotted ids", msg.cityIDs)
	}
}

func TestFetchCmdPropagatesError(t *testing.T) {
	p := &fakeProvider{err: errors.New("network down")}

	msg := fetchCmd(p, []weather.City{{ID: "a"}}, 1)().(forecastsMsg)

	if msg.err == nil {
		t.Fatal("fetch error must reach the message")
	}
}

func TestFetchCmdWithNoProvider(t *testing.T) {
	msg := fetchCmd(nil, []weather.City{{ID: "a"}}, 1)().(forecastsMsg)
	if msg.err == nil {
		t.Error("a nil provider must produce an error, not a panic")
	}
}
```

- [ ] **Step 2: Run it to confirm it fails**

Run: `go test ./internal/ui/ -run TestFetchCmd`
Expected: FAIL — `undefined: fetchCmd`

- [ ] **Step 3: Write the message types**

Create `internal/ui/msgs.go`:

```go
package ui

import (
	"time"

	"github.com/tsoueid/weatherdash/internal/geocode"
	"github.com/tsoueid/weatherdash/internal/weather"
)

// forecastsMsg is the result of one batched fetch.
//
// cityIDs is the snapshot the positional response maps onto. Carrying
// it in the message is what lets Update convert positional results into
// an ID-keyed map without ever consulting the live city list, which may
// have changed while the request was in flight.
type forecastsMsg struct {
	reqID     int
	cityIDs   []string
	forecasts []weather.Forecast
	err       error
}

// tickMsg fires the auto-refresh (Task 33).
type tickMsg time.Time

// geocodeMsg carries city-search results (Task 26).
type geocodeMsg struct {
	seq     int
	results []geocode.Place
	err     error
}

// debounceMsg fires after the search input has been idle (Task 26).
type debounceMsg struct {
	seq   int
	query string
}
```

- [ ] **Step 4: Write the command**

Create `internal/ui/commands.go`:

```go
package ui

import (
	"context"
	"errors"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/tsoueid/weatherdash/internal/weather"
)

// fetchTimeout bounds one batched request. Twenty cities is a single
// call, so a slow response means a slow network, not a large payload.
const fetchTimeout = 20 * time.Second

// fetchCmd builds the command that retrieves every city's forecast in
// one request.
//
// The city slice is COPIED into the closure rather than read from the
// model. The command runs on another goroutine after Update returns, so
// reading live model state here would see the list as it is when the
// response lands — and a city deleted meanwhile would shift every
// result by one, silently pairing each city with its neighbour's
// weather.
func fetchCmd(p weather.Provider, cities []weather.City, reqID int) tea.Cmd {
	snapshot := make([]weather.City, len(cities))
	copy(snapshot, cities)

	ids := make([]string, len(snapshot))
	for i, c := range snapshot {
		ids[i] = c.ID
	}

	return func() tea.Msg {
		if p == nil {
			return forecastsMsg{reqID: reqID, cityIDs: ids,
				err: errors.New("no weather provider configured")}
		}

		ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
		defer cancel()

		forecasts, err := p.FetchAll(ctx, snapshot)
		return forecastsMsg{
			reqID:     reqID,
			cityIDs:   ids,
			forecasts: forecasts,
			err:       err,
		}
	}
}
```

- [ ] **Step 5: Add dependency fields to `internal/ui/model.go`**

Append to the file, and add the listed fields to the `Model` struct:

```go
// Add to the Model struct:
//
//	provider   weather.Provider
//	geocoder   *geocode.Client
//	configPath string
//	cachePath  string
//	reqID      int
//	loading    bool
//	lastOK     time.Time
//	lastErr    error
//	backoffTil time.Time

// Deps are the collaborators the model needs to do real work. They are
// injected after New so that tests and the demo mode can build a fully
// functional model with no network at all.
type Deps struct {
	Provider   weather.Provider
	Geocoder   *geocode.Client
	ConfigPath string
	CachePath  string
}

// WithDeps returns a copy of the model wired to its collaborators.
func (m Model) WithDeps(d Deps) Model {
	m.provider = d.Provider
	m.geocoder = d.Geocoder
	m.configPath = d.ConfigPath
	m.cachePath = d.CachePath
	return m
}
```

Add `"time"`, `"github.com/tsoueid/weatherdash/internal/geocode"` to the imports.

- [ ] **Step 6: Run it to confirm it passes**

Run: `go test ./internal/ui/ -v -race -run TestFetch`
Expected: PASS, 4 tests

- [ ] **Step 7: Commit**

```bash
git add internal/ui/
git commit -m "feat: add batched fetch command with snapshot isolation"
```

---

### Task 22: Handling results, cache-first startup, persistence

**Files:**
- Modify: `internal/ui/update.go` (handle `forecastsMsg`, start the initial fetch in `Init`)
- Modify: `cmd/weatherdash/main.go` (wire `Deps`)
- Create: `internal/ui/results_test.go`

**Interfaces:**
- Consumes: `forecastsMsg`, `fetchCmd`, `Deps` (Task 21); `store.SaveCache` (Task 6)
- Produces: `(Model).applyForecasts(msg forecastsMsg) Model`, `(Model).refreshCmd() (Model, tea.Cmd)`, `ui.saveCacheCmd(path string, data map[string]weather.Forecast) tea.Cmd`

> **Why a stale `reqID` is discarded rather than merged:** pressing `r` while the auto-refresh ticker's request is already in flight produces two responses. Merging both would let the older one overwrite the newer, so the grid would show data that is one refresh cycle out of date — intermittently, and only under timing that is nearly impossible to reproduce by hand.

- [ ] **Step 1: Write the failing test**

Create `internal/ui/results_test.go`:

```go
package ui

import (
	"errors"
	"testing"
	"time"

	"github.com/tsoueid/weatherdash/internal/weather"
)

func TestApplyForecastsStoresByCityID(t *testing.T) {
	m := testModel(t, 3)
	m.reqID = 5
	m.loading = true

	m = m.applyForecasts(forecastsMsg{
		reqID:   5,
		cityIDs: []string{m.cities[0].ID, m.cities[1].ID},
		forecasts: []weather.Forecast{
			{CityID: m.cities[0].ID, Days: []weather.Day{{HiC: 99}}},
			{CityID: m.cities[1].ID, Days: []weather.Day{{HiC: 88}}},
		},
	})

	if m.loading {
		t.Error("a completed fetch must clear the loading flag")
	}
	if m.data[m.cities[0].ID].Days[0].HiC != 99 {
		t.Errorf("city 0 HiC = %v, want 99", m.data[m.cities[0].ID].Days[0].HiC)
	}
	if m.lastOK.IsZero() {
		t.Error("a successful fetch must stamp lastOK")
	}
	if m.lastErr != nil {
		t.Errorf("lastErr = %v, want nil", m.lastErr)
	}
}

// A response from a superseded request must be ignored entirely.
func TestApplyForecastsIgnoresStaleReqID(t *testing.T) {
	m := testModel(t, 2)
	m.reqID = 9
	before := m.data[m.cities[0].ID].Days[0].HiC

	m = m.applyForecasts(forecastsMsg{
		reqID:   3, // superseded
		cityIDs: []string{m.cities[0].ID},
		forecasts: []weather.Forecast{
			{CityID: m.cities[0].ID, Days: []weather.Day{{HiC: 12345}}},
		},
	})

	if got := m.data[m.cities[0].ID].Days[0].HiC; got != before {
		t.Errorf("stale response was applied: HiC = %v, want %v", got, before)
	}
}

// A failed fetch must leave cached data intact and record the error.
func TestApplyForecastsErrorKeepsCache(t *testing.T) {
	m := testModel(t, 2)
	m.reqID = 1
	before := m.data[m.cities[0].ID].Days[0].HiC

	m = m.applyForecasts(forecastsMsg{reqID: 1, err: errors.New("offline")})

	if m.lastErr == nil {
		t.Error("lastErr must record the failure")
	}
	if got := m.data[m.cities[0].ID].Days[0].HiC; got != before {
		t.Errorf("cached data was lost on error: %v, want %v", got, before)
	}
	if m.loading {
		t.Error("a failed fetch must still clear loading")
	}
}

// A result for a city deleted mid-flight must be dropped, not stored.
func TestApplyForecastsDropsDeletedCities(t *testing.T) {
	m := testModel(t, 3)
	m.reqID = 1
	deletedID := m.cities[2].ID
	m.cities = m.cities[:2]
	delete(m.data, deletedID)

	m = m.applyForecasts(forecastsMsg{
		reqID:   1,
		cityIDs: []string{m.cities[0].ID, m.cities[1].ID, deletedID},
		forecasts: []weather.Forecast{
			{CityID: m.cities[0].ID, Days: []weather.Day{{HiC: 1}}},
			{CityID: m.cities[1].ID, Days: []weather.Day{{HiC: 2}}},
			{CityID: deletedID, Days: []weather.Day{{HiC: 3}}},
		},
	})

	if _, ok := m.data[deletedID]; ok {
		t.Error("a forecast for a deleted city must not be stored")
	}
}

func TestApplyForecastsRateLimitSetsBackoff(t *testing.T) {
	m := testModel(t, 2)
	m.reqID = 1

	m = m.applyForecasts(forecastsMsg{
		reqID: 1,
		err:   &weather.RateLimitError{RetryAfter: 45 * time.Second},
	})

	if m.backoffTil.IsZero() {
		t.Fatal("a rate-limit error must set a backoff deadline")
	}
	if time.Until(m.backoffTil) < 30*time.Second {
		t.Errorf("backoff is %v away, want roughly 45s", time.Until(m.backoffTil))
	}
}

func TestRefreshCmdIncrementsReqID(t *testing.T) {
	m := testModel(t, 2)
	m = m.WithDeps(Deps{Provider: &fakeProvider{}})

	before := m.reqID
	m, cmd := m.refreshCmd()

	if m.reqID != before+1 {
		t.Errorf("reqID = %d, want %d", m.reqID, before+1)
	}
	if !m.loading {
		t.Error("refresh must set the loading flag")
	}
	if cmd == nil {
		t.Fatal("refresh must return a command")
	}
}

func TestRefreshCmdRespectsBackoff(t *testing.T) {
	m := testModel(t, 2)
	m = m.WithDeps(Deps{Provider: &fakeProvider{}})
	m.backoffTil = time.Now().Add(time.Minute)

	m2, cmd := m.refreshCmd()
	if cmd != nil {
		t.Error("refresh during backoff must not issue a request")
	}
	if m2.reqID != m.reqID {
		t.Error("refresh during backoff must not consume a reqID")
	}
}

func TestRefreshCmdWithNoCities(t *testing.T) {
	m := New(defaultTestConfig(), emptyTestCache())
	m = m.WithDeps(Deps{Provider: &fakeProvider{}})

	_, cmd := m.refreshCmd()
	if cmd != nil {
		t.Error("refresh with no cities must not issue a request")
	}
}
```

- [ ] **Step 2: Run it to confirm it fails**

Run: `go test ./internal/ui/ -run TestApplyForecasts`
Expected: FAIL — `m.applyForecasts undefined`

- [ ] **Step 3: Extend `internal/ui/update.go`**

Replace `Init` and add the handlers:

```go
// Init renders from cache first, then starts a background refresh.
//
// The model already holds cached forecasts when this runs, so View()
// draws a populated grid before the request is even sent. That is what
// makes startup feel instant rather than one network round-trip.
func (m Model) Init() tea.Cmd {
	next, fetch := m.refreshCmd()
	*(&m) = next
	cmds := []tea.Cmd{m.spinner.Tick}
	if fetch != nil {
		cmds = append(cmds, fetch)
	}
	return tea.Batch(cmds...)
}

// refreshCmd starts a fetch, returning the updated model and command.
//
// It consumes a request ID so that any response from an earlier request
// can be recognised and discarded.
func (m Model) refreshCmd() (Model, tea.Cmd) {
	if m.provider == nil || len(m.cities) == 0 {
		return m, nil
	}
	if time.Now().Before(m.backoffTil) {
		// Rate limited. Refusing here rather than at the transport keeps
		// us from spending the user's quota on requests we know will fail.
		return m, nil
	}

	m.reqID++
	m.loading = true
	return m, fetchCmd(m.provider, m.cities, m.reqID)
}

// applyForecasts merges a fetch result into the model.
func (m Model) applyForecasts(msg forecastsMsg) Model {
	// Discard superseded responses. Without this, a manual r landing
	// after the ticker's request would let the older result overwrite
	// the newer one.
	if msg.reqID != m.reqID {
		return m
	}
	m.loading = false

	if msg.err != nil {
		m.lastErr = msg.err
		var rl *weather.RateLimitError
		if errors.As(msg.err, &rl) {
			m.backoffTil = time.Now().Add(rl.RetryAfter)
		}
		// Cached data is deliberately left untouched: stale numbers with
		// an honest staleness marker beat an empty grid.
		return m
	}

	m.lastErr = nil
	m.lastOK = time.Now()
	m.backoffTil = time.Time{}

	// Build the set of cities that still exist, so a result for one
	// deleted mid-flight is dropped rather than resurrected.
	live := make(map[string]bool, len(m.cities))
	for _, c := range m.cities {
		live[c.ID] = true
	}

	for i, f := range msg.forecasts {
		id := f.CityID
		if id == "" && i < len(msg.cityIDs) {
			id = msg.cityIDs[i]
		}
		if !live[id] {
			continue
		}
		f.CityID = id
		m.data[id] = f
	}

	m.clampCursor()
	return m
}
```

Add `"errors"`, `"time"`, and `"github.com/tsoueid/weatherdash/internal/weather"` to the imports.

- [ ] **Step 4: Route the message in `Update`**

Add these cases to the `Update` type switch, before the default spinner handling:

```go
	case forecastsMsg:
		m = m.applyForecasts(msg)
		return m, saveCacheCmd(m.cachePath, m.data)
```

And add the `Refresh` key to `handleGridKey`:

```go
	case key.Matches(msg, m.keys.Refresh):
		return m.refreshCmd()
```

- [ ] **Step 5: Add the cache-persist command to `internal/ui/commands.go`**

```go
// saveCacheCmd persists forecasts in the background.
//
// The error is swallowed on purpose: a cache that fails to write costs
// one slow startup and nothing else, so interrupting the user with it
// would be noise. Config write failures, which lose saved cities, are
// surfaced — see Task 28.
func saveCacheCmd(path string, data map[string]weather.Forecast) tea.Cmd {
	if path == "" || len(data) == 0 {
		return nil
	}
	snapshot := make(map[string]weather.Forecast, len(data))
	for k, v := range data {
		snapshot[k] = v
	}
	return func() tea.Msg {
		c := store.NewCache()
		c.Forecasts = snapshot
		_ = store.SaveCache(path, c)
		return nil
	}
}
```

Add `"github.com/tsoueid/weatherdash/internal/store"` to the imports.

- [ ] **Step 6: Wire `Deps` in `cmd/weatherdash/main.go`**

Replace the non-demo branch of `buildModel`:

```go
	model := ui.New(cfg, store.LoadCache(cachePath))
	return model.WithDeps(ui.Deps{
		Provider:   weather.NewClient(cfg.ForecastDays),
		Geocoder:   geocode.NewClient(),
		ConfigPath: cfgPath,
		CachePath:  cachePath,
	}), nil
```

Add the `geocode` and `weather` imports.

- [ ] **Step 7: Run the tests**

Run: `go test ./... -race`
Expected: PASS

- [ ] **Step 8: Verify live data end to end**

Run: `make build && ./weatherdash`

On a first run the grid is empty (no cities yet) — that is expected and Task 34 gives it a proper empty state. To verify fetching now, temporarily seed a city by hand:

```bash
mkdir -p ~/.config/weatherdash
cat > ~/.config/weatherdash/config.toml <<'EOF'
units = "metric"
refresh_minutes = 15
forecast_days = 7

[[city]]
id = "om-2988507"
name = "Paris"
admin = "Île-de-France"
country = "France"
cc = "FR"
lat = 48.8566
lon = 2.3522
tz = "Europe/Paris"
EOF
./weatherdash
```

Verify: real Paris weather appears; `r` refetches; quitting and relaunching paints the grid instantly from `~/.cache/weatherdash/cache.json` before the network responds.

- [ ] **Step 9: Commit**

```bash
git add .
git commit -m "feat: wire live fetching with cache-first startup and persistence"
```

---

### Task 23: Freshness footer

**Files:**
- Modify: `internal/ui/grid.go` (replace `footerHint`)
- Create: `internal/ui/footer.go`, `internal/ui/footer_test.go`

**Interfaces:**
- Consumes: `Model.loading`, `Model.lastOK`, `Model.lastErr` (Task 22); `theme` (Task 19)
- Produces: `(Model).footer(width int) string`, `ui.humanAge(d time.Duration) string`

> **Why staleness is always on screen rather than shown only on failure:** a dashboard that silently displays this morning's weather at 6pm is worse than one that shows nothing, because the user has no way to know. The age string is the cheapest possible honesty.

- [ ] **Step 1: Write the failing test**

Create `internal/ui/footer_test.go`:

```go
package ui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
)

func TestFooterWidthIsExact(t *testing.T) {
	m := testModel(t, 3)
	m.lastOK = time.Now().Add(-3 * time.Minute)

	for _, w := range []int{60, 80, 120} {
		if got := lipgloss.Width(m.footer(w)); got != w {
			t.Errorf("footer width = %d, want %d", got, w)
		}
	}
}

func TestFooterShowsAge(t *testing.T) {
	m := testModel(t, 3)
	m.lastOK = time.Now().Add(-3 * time.Minute)

	if !strings.Contains(m.footer(120), "3m ago") {
		t.Errorf("footer = %q, want it to mention 3m ago", m.footer(120))
	}
}

func TestFooterShowsRefreshing(t *testing.T) {
	m := testModel(t, 3)
	m.loading = true

	if !strings.Contains(strings.ToLower(m.footer(120)), "refreshing") {
		t.Errorf("footer = %q, want a refreshing indicator", m.footer(120))
	}
}

func TestFooterShowsStalenessOnError(t *testing.T) {
	m := testModel(t, 3)
	m.lastErr = errors.New("network down")
	m.lastOK = time.Now().Add(-2 * time.Hour)

	got := m.footer(120)
	if !strings.Contains(got, "⚠") {
		t.Errorf("footer = %q, want a warning marker", got)
	}
	if !strings.Contains(got, "2h") {
		t.Errorf("footer = %q, want the staleness age", got)
	}
}

func TestFooterNeverFetched(t *testing.T) {
	m := testModel(t, 3)
	if got := lipgloss.Width(m.footer(80)); got != 80 {
		t.Errorf("width = %d, want 80", got)
	}
}

func TestHumanAge(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Second, "just now"},
		{90 * time.Second, "1m ago"},
		{45 * time.Minute, "45m ago"},
		{3 * time.Hour, "3h ago"},
		{50 * time.Hour, "2d ago"},
	}
	for _, tc := range tests {
		if got := humanAge(tc.d); got != tc.want {
			t.Errorf("humanAge(%v) = %q, want %q", tc.d, got, tc.want)
		}
	}
}
```

- [ ] **Step 2: Run it to confirm it fails**

Run: `go test ./internal/ui/ -run TestFooter`
Expected: FAIL — `m.footer undefined`

- [ ] **Step 3: Write the implementation**

Create `internal/ui/footer.go`:

```go
package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/tsoueid/weatherdash/internal/widgets"
)

// humanAge renders a duration the way a person would say it.
func humanAge(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

// footer renders the key hints plus a data-freshness indicator.
//
// Freshness is shown at all times, not only on failure: a dashboard
// quietly displaying this morning's weather at six in the evening is
// worse than one showing nothing, because nothing on screen says so.
func (m Model) footer(width int) string {
	th := newTheme()

	hints := " ↑↓ city · ←→ day · ⏎ zoom · a add · / filter · s sort · ? help"

	var status string
	switch {
	case m.loading:
		status = m.spinner.View() + " refreshing…"
	case m.lastErr != nil && !m.lastOK.IsZero():
		status = th.warn.Render(
			fmt.Sprintf("⚠ stale %s · %s", humanAge(time.Since(m.lastOK)), errSummary(m.lastErr)))
	case m.lastErr != nil:
		status = th.warn.Render("⚠ " + errSummary(m.lastErr))
	case !m.lastOK.IsZero():
		status = th.dim.Render("updated " + humanAge(time.Since(m.lastOK)))
	default:
		status = th.dim.Render("no data yet")
	}

	gap := width - lipglossWidth(hints) - lipglossWidth(status) - 2
	if gap < 1 {
		// Too narrow for both: the status wins, because it is the part
		// the user cannot get any other way.
		return widgets.Pad(" "+status, width)
	}
	return widgets.Pad(hints+strings.Repeat(" ", gap)+status+" ", width)
}

// errSummary trims an error to something that fits on one line.
func errSummary(err error) string {
	s := err.Error()
	if len(s) > 40 {
		s = s[:37] + "…"
	}
	return s
}
```

Add a small helper at the bottom of `internal/ui/theme.go`:

```go
// lipglossWidth is a local alias so callers do not each import lipgloss
// just to measure a string.
func lipglossWidth(s string) int { return lipgloss.Width(s) }
```

- [ ] **Step 4: Use it in `renderGrid`**

In `internal/ui/grid.go`, replace the footer line:

```go
	lines = append(lines, m.footer(m.w))
```

and delete the now-unused `footerHint` method.

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/ui/ -v -race`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/ui/
git commit -m "feat: add freshness and error indicators to the footer"
```

---

# Phase 6 — Detail Screen

### Task 24: Detail renderer

**Files:**
- Create: `internal/ui/detail.go`, `internal/ui/detail_test.go`

**Interfaces:**
- Consumes: `Model`, `dayAt`, `selectedCity` (Task 17); all widgets (Phase 3); `weather.Label` (Task 7)
- Produces: `(Model).renderDetail() string`, `(Model).isToday(d weather.Day, city weather.City) bool`

> **Why the "now" block is conditional:** a future day has no current conditions. Rendering the hero unconditionally would show today's live temperature under Friday's date, which is the kind of wrong that looks right.

- [ ] **Step 1: Write the failing test**

Create `internal/ui/detail_test.go`:

```go
package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/tsoueid/weatherdash/internal/weather"
)

func TestRenderDetailLineWidths(t *testing.T) {
	for _, size := range []struct{ w, h int }{{80, 24}, {120, 40}, {60, 20}} {
		m := testModel(t, 4)
		m.w, m.h = size.w, size.h
		m.screen = screenDetail
		m.cursorCity, m.cursorDay = 1, 2

		for i, line := range strings.Split(m.renderDetail(), "\n") {
			if got := lipgloss.Width(line); got != size.w {
				t.Errorf("%dx%d line %d width = %d, want %d: %q",
					size.w, size.h, i, got, size.w, line)
			}
		}
	}
}

func TestRenderDetailShowsCityAndPosition(t *testing.T) {
	m := testModel(t, 4)
	m.cursorCity, m.cursorDay = 2, 3

	out := m.renderDetail()
	if !strings.Contains(out, "Tokyo") {
		t.Errorf("detail must name the city:\n%s", out)
	}
	if !strings.Contains(out, "4 of 7") {
		t.Errorf("detail must show 'day 4 of 7':\n%s", out)
	}
}

func TestRenderDetailShowsConditionLabel(t *testing.T) {
	m := testModel(t, 2)
	f := m.data[m.cities[0].ID]
	f.Days[0].Code = 65 // Heavy rain
	m.data[m.cities[0].ID] = f

	if !strings.Contains(m.renderDetail(), "Heavy rain") {
		t.Errorf("detail must show the condition label:\n%s", m.renderDetail())
	}
}

func TestRenderDetailWithoutData(t *testing.T) {
	m := testModel(t, 2)
	m.data = map[string]weather.Forecast{}

	out := m.renderDetail()
	if strings.TrimSpace(out) == "" {
		t.Error("detail with no data must render a message, not nothing")
	}
	for i, line := range strings.Split(out, "\n") {
		if got := lipgloss.Width(line); got != m.w {
			t.Errorf("line %d width = %d, want %d", i, got, m.w)
		}
	}
}

func TestRenderDetailEmptyCityList(t *testing.T) {
	m := New(defaultTestConfig(), emptyTestCache())
	m.w, m.h = 100, 30
	m.screen = screenDetail

	if out := m.renderDetail(); strings.TrimSpace(out) == "" {
		t.Error("detail with no cities must render something")
	}
}

func TestIsTodayComparesLocalDate(t *testing.T) {
	m := testModel(t, 1)
	city := m.cities[0]

	today := weather.Day{Date: time.Now().Truncate(24 * time.Hour)}
	if !m.isToday(today, city) {
		t.Error("today's date must be recognised as today")
	}

	future := weather.Day{Date: time.Now().AddDate(0, 0, 3).Truncate(24 * time.Hour)}
	if m.isToday(future, city) {
		t.Error("a date three days out must not be today")
	}
}
```

- [ ] **Step 2: Run it to confirm it fails**

Run: `go test ./internal/ui/ -run TestRenderDetail`
Expected: FAIL — `m.renderDetail undefined`

- [ ] **Step 3: Write the implementation**

Create `internal/ui/detail.go`:

```go
package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/tsoueid/weatherdash/internal/weather"
	"github.com/tsoueid/weatherdash/internal/widgets"
)

// detailGutter is the left margin for labelled rows.
const detailGutter = 9

// isToday reports whether a forecast day is the current local day in
// that city's timezone.
//
// The comparison happens in the city's zone, not the viewer's: at 23:00
// in Paris it is already tomorrow in Tokyo, and showing Tokyo's live
// conditions under yesterday's date would be wrong.
func (m Model) isToday(d weather.Day, city weather.City) bool {
	if d.Date.IsZero() {
		return false
	}
	loc, err := time.LoadLocation(city.TZ)
	if err != nil {
		loc = time.Local
	}
	now := time.Now().In(loc)
	return d.Date.Year() == now.Year() && d.Date.YearDay() == now.YearDay()
}

// renderDetail draws one city on one day: the zoomed view of a single
// matrix cell.
func (m Model) renderDetail() string {
	th := newTheme()
	lines := make([]string, 0, m.h)

	city, hasCity := m.selectedCity()
	if !hasCity {
		return m.fill([]string{th.dim.Render("No cities saved. Press a to add one.")})
	}

	day, hasDay := m.dayAt(m.cursorCity, m.cursorDay)
	if !hasDay {
		return m.fill([]string{
			th.header.Render(city.Full()),
			"",
			th.dim.Render("No forecast data for this day yet. Press r to refresh."),
		})
	}

	// Header: who, when, and where in the window.
	title := fmt.Sprintf("%s — %s — day %d of %d",
		city.Full(), day.Date.Format("Mon 2 Jan"), m.cursorDay+1, m.days)
	lines = append(lines, th.header.Render(widgets.Pad(" "+title, m.w)))
	lines = append(lines, widgets.Pad(strings.Repeat("─", m.w), m.w))

	// Hero: condition, range, and the day's headline numbers.
	icon := weather.Icon(day.Code, m.icons)
	lines = append(lines, widgets.Pad(fmt.Sprintf("  %s  %s      %s / %s",
		icon, weather.Label(day.Code),
		m.units.Temp(day.HiC), m.units.Temp(day.LoC)), m.w))
	lines = append(lines, th.dim.Render(widgets.Pad(fmt.Sprintf(
		"       %s total · wind to %s · UV %.0f",
		m.units.Precip(day.PrecipMM), m.units.Speed(day.WindMaxKPH), day.UVMax), m.w)))

	// The current-conditions line exists only for today. A future day
	// has no "now", and showing today's live reading under Friday's
	// date would be wrong in a way that looks right.
	if m.isToday(day, city) {
		if f, ok := m.selectedForecast(); ok {
			lines = append(lines, widgets.Pad(fmt.Sprintf(
				"       now %s · feels %s · %s %s",
				m.units.Temp(f.Current.TempC), m.units.Temp(f.Current.ApparentC),
				widgets.WindArrow(f.Current.WindDir), m.units.Speed(f.Current.WindKPH)), m.w))
		}
	}
	lines = append(lines, strings.Repeat(" ", m.w))

	chartW := m.w - detailGutter - 2
	if chartW < 8 {
		chartW = 8
	}

	temps := make([]float64, 0, len(day.Hours))
	probs := make([]int, 0, len(day.Hours))
	mms := make([]float64, 0, len(day.Hours))
	for _, h := range day.Hours {
		temps = append(temps, h.TempC)
		probs = append(probs, h.PrecipProb)
		mms = append(mms, h.PrecipMM)
	}

	row := func(label, body string) string {
		return widgets.Pad("  "+widgets.Pad(label, detailGutter-2)+body, m.w)
	}

	lines = append(lines, row("Temp",
		widgets.Sparkline(temps, chartW, func(v float64) lipglossStyle {
			return styleFor(tempColor(v))
		})))
	lines = append(lines, row("Rain", widgets.PrecipBand(probs, chartW)))
	lines = append(lines, row("", th.dim.Render(widgets.Sparkline(mms, chartW, nil))))

	arrows, speeds := widgets.WindRow(day.Hours, chartW, m.units)
	lines = append(lines, row("Wind", arrows))
	lines = append(lines, row("", th.dim.Render(speeds)))
	lines = append(lines, row("", th.dim.Render(hourAxis(chartW))))
	lines = append(lines, strings.Repeat(" ", m.w))

	lines = append(lines, row("Sun", fmt.Sprintf("%s %s %s",
		day.Sunrise.Format("15:04"),
		widgets.SunArc(day.Sunrise, day.Sunset, time.Now(), chartW-14),
		day.Sunset.Format("15:04"))))

	return m.fillWithFooter(lines,
		" ←→ day · ↑↓ city · esc back · u units · ? help")
}

// hourAxis labels a 24-hour chart every three hours.
func hourAxis(width int) string {
	axis := []rune(strings.Repeat(" ", width))
	for h := 0; h < 24; h += 3 {
		col := h * width / 24
		label := fmt.Sprintf("%02d", h)
		for i, r := range label {
			if col+i < width {
				axis[col+i] = r
			}
		}
	}
	return string(axis)
}
```

- [ ] **Step 4: Add the shared frame helpers to `internal/ui/view.go`**

```go
// fill pads a block of lines to exactly the window size, so every
// screen returns a full frame and none of them has to repeat the
// arithmetic.
func (m Model) fill(lines []string) string {
	return m.fillWithFooter(lines, "")
}

// fillWithFooter pads lines and pins a hint to the bottom row.
func (m Model) fillWithFooter(lines []string, hint string) string {
	out := make([]string, 0, m.h)
	for _, l := range lines {
		out = append(out, widgets.Pad(l, m.w))
	}
	for len(out) < m.h-1 {
		out = append(out, strings.Repeat(" ", m.w))
	}
	if hint != "" {
		out = append(out, newTheme().dim.Render(widgets.Pad(hint, m.w)))
	} else {
		out = append(out, strings.Repeat(" ", m.w))
	}
	return strings.Join(out[:m.h], "\n")
}
```

Add `"strings"` and the `widgets` import.

- [ ] **Step 5: Add the style shim to `internal/ui/theme.go`**

```go
// lipglossStyle aliases the style type so detail.go can name it without
// importing lipgloss directly.
type lipglossStyle = lipgloss.Style

// styleFor builds a foreground style from a colour.
func styleFor(c lipgloss.Color) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(c)
}
```

Note: `widgets.Colorize` is `func(float64) lipgloss.Style`, so the alias must match exactly.

- [ ] **Step 6: Run the tests**

Run: `go test ./internal/ui/ -v -race -run TestRenderDetail`
Expected: PASS, 6 tests

- [ ] **Step 7: Commit**

```bash
git add internal/ui/
git commit -m "feat: add detail screen renderer"
```

---

### Task 25: Zoom navigation

**Files:**
- Modify: `internal/ui/update.go` (Enter, Esc, detail-screen key routing)
- Modify: `internal/ui/view.go` (dispatch `screenDetail`)
- Create: `internal/ui/zoom_test.go`

**Interfaces:**
- Consumes: `renderDetail` (Task 24); `handleGridKey` (Task 20)
- Produces: `(Model).handleDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd)`

> **The property this task delivers:** `←→ / ↑↓` keep working *inside* the detail view, and escaping returns to the grid with the cursor exactly where it was. The detail screen is a magnifying glass slid across the matrix, not a separate destination you navigate back from.

- [ ] **Step 1: Write the failing test**

Create `internal/ui/zoom_test.go`:

```go
package ui

import "testing"

func TestEnterZoomsInAndEscReturns(t *testing.T) {
	m := send(testModel(t, 4), "down", "right", "right", "enter")

	if m.screen != screenDetail {
		t.Fatalf("screen = %v, want screenDetail", m.screen)
	}
	if m.cursorCity != 1 || m.cursorDay != 2 {
		t.Errorf("zooming changed the cursor to %d,%d; want 1,2",
			m.cursorCity, m.cursorDay)
	}

	m = send(m, "esc")
	if m.screen != screenGrid {
		t.Errorf("screen = %v, want screenGrid", m.screen)
	}
	if m.cursorCity != 1 || m.cursorDay != 2 {
		t.Errorf("escaping changed the cursor to %d,%d; want 1,2",
			m.cursorCity, m.cursorDay)
	}
}

// The detail view is a magnifying glass over the matrix: moving inside
// it moves the same cursor, without returning to the grid.
func TestDetailNavigationMovesTheSharedCursor(t *testing.T) {
	m := send(testModel(t, 4), "enter", "right", "right", "down")

	if m.screen != screenDetail {
		t.Fatalf("screen = %v, want to stay in screenDetail", m.screen)
	}
	if m.cursorDay != 2 {
		t.Errorf("cursorDay = %d, want 2", m.cursorDay)
	}
	if m.cursorCity != 1 {
		t.Errorf("cursorCity = %d, want 1", m.cursorCity)
	}
}

func TestDetailNavigationClamps(t *testing.T) {
	m := send(testModel(t, 2), "enter", "left", "left", "up", "up")

	if m.cursorCity != 0 || m.cursorDay != 0 {
		t.Errorf("cursor = %d,%d; want 0,0", m.cursorCity, m.cursorDay)
	}
	if m.screen != screenDetail {
		t.Errorf("clamping must not leave the detail screen")
	}
}

func TestEnterWithNoCitiesStaysOnGrid(t *testing.T) {
	m := New(defaultTestConfig(), emptyTestCache())
	m.w, m.h = 100, 30
	m = send(m, "enter")

	if m.screen != screenGrid {
		t.Errorf("screen = %v, want screenGrid with no cities to zoom into", m.screen)
	}
}

func TestViewRendersDetailWhenZoomed(t *testing.T) {
	m := send(testModel(t, 3), "enter")
	if got := m.View(); got == "" {
		t.Error("View must render the detail screen")
	}
}
```

- [ ] **Step 2: Run it to confirm it fails**

Run: `go test ./internal/ui/ -run TestEnterZooms`
Expected: FAIL — Enter does nothing yet

- [ ] **Step 3: Extend `handleKey` and `handleGridKey` in `internal/ui/update.go`**

Add to the screen switch in `handleKey`:

```go
	case screenDetail:
		return m.handleDetailKey(msg)
```

Add to `handleGridKey`:

```go
	case key.Matches(msg, m.keys.Enter):
		// Nothing to zoom into with no cities; stay put rather than
		// opening an empty detail screen.
		if len(m.cities) > 0 {
			m.screen = screenDetail
		}
```

Add the new handler:

```go
// handleDetailKey moves the SAME cursor the grid uses, so escaping
// returns to exactly the cell the user zoomed out of and panning inside
// the detail view needs no round trip through the grid.
func (m Model) handleDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Back):
		m.screen = screenGrid
	case key.Matches(msg, m.keys.Up):
		m.moveCity(-1)
		m.ensureCursorVisible()
	case key.Matches(msg, m.keys.Down):
		m.moveCity(1)
		m.ensureCursorVisible()
	case key.Matches(msg, m.keys.Left):
		m.moveDay(-1)
	case key.Matches(msg, m.keys.Right):
		m.moveDay(1)
	case key.Matches(msg, m.keys.Refresh):
		return m.refreshCmd()
	}
	return m, nil
}
```

- [ ] **Step 4: Dispatch in `View`**

Add to the screen switch in `internal/ui/view.go`:

```go
	case screenDetail:
		return m.renderDetail()
```

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/ui/ -v -race`
Expected: PASS

- [ ] **Step 6: Verify by hand**

Run: `make build && ./weatherdash --demo`

1. Move to Tokyo/Friday, press `⏎` — the detail screen shows exactly that cell.
2. Press `→` twice and `↓` once — the detail screen follows, no round trip.
3. Press `esc` — the grid highlight is on the cell you last viewed, not where you started.

- [ ] **Step 7: Commit**

```bash
git add internal/ui/
git commit -m "feat: zoom between grid and detail with a shared cursor"
```

---

# Phase 7 — Adding and Removing Cities

### Task 26: Debounced search command

**Files:**
- Modify: `internal/ui/commands.go` (add search commands)
- Create: `internal/ui/search_test.go`

**Interfaces:**
- Consumes: `geocode.Client` (Task 11); `debounceMsg`, `geocodeMsg` (Task 21)
- Produces: `ui.debounceCmd(query string, seq int) tea.Cmd`, `ui.geocodeCmd(g *geocode.Client, query string, seq int) tea.Cmd`, constant `searchDebounce`

> **Why a sequence number rather than cancelling the previous request:** typing "Paris" fires five debounce timers. Only the last should produce results, and an in-flight request for "Pari" may well land *after* the one for "Paris". Stamping each with a monotonic sequence and ignoring anything but the newest is simpler and more reliable than cancellation, and it needs no shared mutable state.

- [ ] **Step 1: Write the failing test**

Create `internal/ui/search_test.go`:

```go
package ui

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tsoueid/weatherdash/internal/geocode"
)

func TestDebounceCmdCarriesSeqAndQuery(t *testing.T) {
	msg := debounceCmd("Par", 4)().(debounceMsg)
	if msg.seq != 4 || msg.query != "Par" {
		t.Errorf("debounceMsg = %+v, want seq 4 query Par", msg)
	}
}

func TestGeocodeCmdReturnsResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"results":[{"id":1,"name":"Paris","country":"France",
			"country_code":"FR","admin1":"Île-de-France","latitude":48.85,
			"longitude":2.35,"timezone":"Europe/Paris"}]}`))
	}))
	defer srv.Close()

	g := geocode.NewClient()
	g.BaseURL = srv.URL

	msg := geocodeCmd(g, "Paris", 2)().(geocodeMsg)
	if msg.err != nil {
		t.Fatalf("err = %v", msg.err)
	}
	if msg.seq != 2 {
		t.Errorf("seq = %d, want 2", msg.seq)
	}
	if len(msg.results) != 1 || msg.results[0].Name != "Paris" {
		t.Errorf("results = %+v", msg.results)
	}
}

func TestGeocodeCmdReportsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	g := geocode.NewClient()
	g.BaseURL = srv.URL

	if msg := geocodeCmd(g, "Paris", 1)().(geocodeMsg); msg.err == nil {
		t.Error("a failing search must report an error")
	}
}

func TestGeocodeCmdNilClient(t *testing.T) {
	if msg := geocodeCmd(nil, "Paris", 1)().(geocodeMsg); msg.err == nil {
		t.Error("a nil geocoder must produce an error, not a panic")
	}
}
```

- [ ] **Step 2: Run it to confirm it fails**

Run: `go test ./internal/ui/ -run TestDebounceCmd`
Expected: FAIL — `undefined: debounceCmd`

- [ ] **Step 3: Append to `internal/ui/commands.go`**

```go
// searchDebounce is how long the input must be idle before a search
// fires. Long enough that typing a city name costs one request rather
// than one per character; short enough to feel immediate.
const searchDebounce = 250 * time.Millisecond

// debounceCmd fires after the search input has been idle.
//
// Every keystroke schedules one of these. Only the message whose seq
// matches the model's current sequence triggers a request, so the four
// timers left over from typing "Pari" expire harmlessly.
func debounceCmd(query string, seq int) tea.Cmd {
	return tea.Tick(searchDebounce, func(time.Time) tea.Msg {
		return debounceMsg{seq: seq, query: query}
	})
}

// geocodeCmd performs a city search.
//
// The result carries its sequence number because responses can arrive
// out of order: a request for "Pari" may well land after the one for
// "Paris", and applying it would replace the right answer with a stale
// one.
func geocodeCmd(g *geocode.Client, query string, seq int) tea.Cmd {
	return func() tea.Msg {
		if g == nil {
			return geocodeMsg{seq: seq, err: errors.New("no geocoder configured")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		results, err := g.Search(ctx, query)
		return geocodeMsg{seq: seq, results: results, err: err}
	}
}
```

Add `"github.com/tsoueid/weatherdash/internal/geocode"` to the imports.

- [ ] **Step 4: Run it to confirm it passes**

Run: `go test ./internal/ui/ -v -race -run 'TestDebounce|TestGeocode'`
Expected: PASS, 4 tests

- [ ] **Step 5: Commit**

```bash
git add internal/ui/
git commit -m "feat: add debounced city search commands"
```

---

### Task 27: Add-city modal

**Files:**
- Create: `internal/ui/addcity.go`, `internal/ui/addcity_test.go`
- Modify: `internal/ui/model.go` (search state fields), `internal/ui/update.go`, `internal/ui/view.go`

**Interfaces:**
- Consumes: `debounceCmd`, `geocodeCmd` (Task 26)
- Produces: `(Model).renderAddCity() string`, `(Model).handleAddCityKey(msg tea.KeyMsg) (tea.Model, tea.Cmd)`; Model fields `search textinput.Model`, `searchSeq int`, `searchResults []geocode.Place`, `searchSel int`, `searchErr error`, `searching bool`

> **Why results show the administrative region:** disambiguation is the entire reason this screen exists. A list of eight rows all reading "Paris" is not a picker, it is a coin flip.

- [ ] **Step 1: Write the failing test**

Create `internal/ui/addcity_test.go`:

```go
package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/tsoueid/weatherdash/internal/geocode"
)

func parisResults() []geocode.Place {
	return []geocode.Place{
		{ID: 1, Name: "Paris", Admin1: "Île-de-France", Country: "France",
			CC: "FR", Latitude: 48.85, Longitude: 2.35, Timezone: "Europe/Paris"},
		{ID: 2, Name: "Paris", Admin1: "Texas", Country: "United States",
			CC: "US", Latitude: 33.66, Longitude: -95.55, Timezone: "America/Chicago"},
	}
}

func TestAOpensSearch(t *testing.T) {
	m := send(testModel(t, 3), "a")
	if m.screen != screenAddCity {
		t.Fatalf("screen = %v, want screenAddCity", m.screen)
	}
	if !m.search.Focused() {
		t.Error("the search input must be focused on open")
	}
}

func TestEscClosesSearchWithoutAdding(t *testing.T) {
	m := send(testModel(t, 3), "a", "esc")
	if m.screen != screenGrid {
		t.Errorf("screen = %v, want screenGrid", m.screen)
	}
	if len(m.cities) != 3 {
		t.Errorf("got %d cities, want the original 3", len(m.cities))
	}
}

func TestRenderAddCityShowsDisambiguatedResults(t *testing.T) {
	m := send(testModel(t, 2), "a")
	m.searchResults = parisResults()

	out := m.renderAddCity()
	if !strings.Contains(out, "Île-de-France") || !strings.Contains(out, "Texas") {
		t.Errorf("results must be disambiguated by region:\n%s", out)
	}
	for i, line := range strings.Split(out, "\n") {
		if got := lipgloss.Width(line); got != m.w {
			t.Errorf("line %d width = %d, want %d", i, got, m.w)
		}
	}
}

func TestSearchResultSelectionMoves(t *testing.T) {
	m := send(testModel(t, 2), "a")
	m.searchResults = parisResults()

	m = send(m, "down")
	if m.searchSel != 1 {
		t.Errorf("searchSel = %d, want 1", m.searchSel)
	}
	m = send(m, "down", "down")
	if m.searchSel != 1 {
		t.Errorf("searchSel = %d, want it clamped to 1", m.searchSel)
	}
	m = send(m, "up", "up")
	if m.searchSel != 0 {
		t.Errorf("searchSel = %d, want 0", m.searchSel)
	}
}

func TestTypingSchedulesDebounce(t *testing.T) {
	m := send(testModel(t, 2), "a")
	before := m.searchSeq

	next, cmd := m.Update(keyMsg("P"))
	m = next.(Model)

	if m.searchSeq <= before {
		t.Errorf("searchSeq = %d, want greater than %d", m.searchSeq, before)
	}
	if cmd == nil {
		t.Error("typing must schedule a debounce command")
	}
}

// Only the newest debounce may fire a request.
func TestStaleDebounceIsIgnored(t *testing.T) {
	m := send(testModel(t, 2), "a")
	m.searchSeq = 5

	_, cmd := m.Update(debounceMsg{seq: 2, query: "Par"})
	if cmd != nil {
		t.Error("a superseded debounce must not trigger a search")
	}
}

func TestStaleGeocodeResultIsIgnored(t *testing.T) {
	m := send(testModel(t, 2), "a")
	m.searchSeq = 5
	m.searchResults = parisResults()

	next, _ := m.Update(geocodeMsg{seq: 2, results: nil})
	m = next.(Model)

	if len(m.searchResults) != 2 {
		t.Error("a superseded geocode response must not clear current results")
	}
}

func TestGeocodeErrorShownInline(t *testing.T) {
	m := send(testModel(t, 2), "a")
	m.searchSeq = 3

	next, _ := m.Update(geocodeMsg{seq: 3, err: errTest})
	m = next.(Model)

	if m.screen != screenAddCity {
		t.Error("a search error must leave the modal open")
	}
	if m.searchErr == nil {
		t.Error("the error must be recorded for inline display")
	}
}
```

Add to `model_test.go`:

```go
var errTest = errors.New("search failed")
```

with `"errors"` imported.

- [ ] **Step 2: Run it to confirm it fails**

Run: `go test ./internal/ui/ -run TestAOpensSearch`
Expected: FAIL — `undefined: screenAddCity` handling

- [ ] **Step 3: Add search state to `internal/ui/model.go`**

Add to the `Model` struct:

```go
	search        textinput.Model
	searchSeq     int
	searchResults []geocode.Place
	searchSel     int
	searchErr     error
	searching     bool
```

And in `New`, after the filter input:

```go
	si := textinput.New()
	si.Placeholder = "city name"
	si.Prompt = "  search: "
	si.CharLimit = 64
```

assigning `search: si` in the returned struct.

- [ ] **Step 4: Write the modal renderer**

Create `internal/ui/addcity.go`:

```go
package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/tsoueid/weatherdash/internal/widgets"
)

// renderAddCity draws the city search modal.
func (m Model) renderAddCity() string {
	th := newTheme()
	lines := []string{
		th.header.Render(widgets.Pad(" Add a city", m.w)),
		widgets.Pad(strings.Repeat("─", m.w), m.w),
		widgets.Pad(m.search.View(), m.w),
		strings.Repeat(" ", m.w),
	}

	switch {
	case m.searchErr != nil:
		lines = append(lines, th.warn.Render(
			widgets.Pad("  "+m.searchErr.Error(), m.w)))
	case m.searching:
		lines = append(lines, th.dim.Render(
			widgets.Pad("  "+m.spinner.View()+" searching…", m.w)))
	case len(m.searchResults) == 0 && m.search.Value() != "":
		lines = append(lines, th.dim.Render(
			widgets.Pad("  no matches", m.w)))
	default:
		for i, p := range m.searchResults {
			// The region is what makes these rows distinguishable; a
			// list of eight identical "Paris" entries is a coin flip,
			// not a picker.
			row := fmt.Sprintf("  %s  (%.2f, %.2f)", p.Label(), p.Latitude, p.Longitude)
			if i == m.searchSel {
				row = th.cursor.Render(widgets.Pad(row, m.w))
			} else {
				row = widgets.Pad(row, m.w)
			}
			lines = append(lines, row)
		}
	}

	return m.fillWithFooter(lines, " type to search · ↑↓ select · ⏎ add · esc cancel")
}

// handleAddCityKey routes keys while the modal is open.
func (m Model) handleAddCityKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Back):
		m.screen = screenGrid
		m.search.Blur()
		m.search.SetValue("")
		m.searchResults = nil
		m.searchErr = nil
		m.searchSel = 0
		return m, nil

	case msg.Type == tea.KeyUp:
		if m.searchSel > 0 {
			m.searchSel--
		}
		return m, nil

	case msg.Type == tea.KeyDown:
		if m.searchSel < len(m.searchResults)-1 {
			m.searchSel++
		}
		return m, nil

	case msg.Type == tea.KeyEnter:
		return m.commitSelectedPlace()
	}

	// Anything else is text. Each edit bumps the sequence and schedules
	// a fresh debounce; the timers left over from earlier keystrokes
	// expire without firing because their seq no longer matches.
	var cmd tea.Cmd
	m.search, cmd = m.search.Update(msg)

	m.searchSeq++
	m.searchErr = nil
	return m, tea.Batch(cmd, debounceCmd(m.search.Value(), m.searchSeq))
}
```

- [ ] **Step 5: Route messages in `internal/ui/update.go`**

Add to `handleKey`'s screen switch:

```go
	case screenAddCity:
		return m.handleAddCityKey(msg)
```

Add to `handleGridKey`:

```go
	case key.Matches(msg, m.keys.Add):
		m.screen = screenAddCity
		m.search.SetValue("")
		m.search.Focus()
		m.searchResults = nil
		m.searchSel = 0
		m.searchErr = nil
		return m, textinput.Blink
```

Add these cases to the `Update` type switch:

```go
	case debounceMsg:
		// Only the newest debounce fires a request.
		if msg.seq != m.searchSeq || strings.TrimSpace(msg.query) == "" {
			return m, nil
		}
		m.searching = true
		return m, geocodeCmd(m.geocoder, msg.query, msg.seq)

	case geocodeMsg:
		// Responses can arrive out of order; ignore superseded ones.
		if msg.seq != m.searchSeq {
			return m, nil
		}
		m.searching = false
		m.searchErr = msg.err
		if msg.err == nil {
			m.searchResults = msg.results
			m.searchSel = 0
		}
		return m, nil
```

Add `"strings"` and the `textinput` import.

- [ ] **Step 6: Dispatch in `View`**

```go
	case screenAddCity:
		return m.renderAddCity()
```

- [ ] **Step 7: Add a temporary `commitSelectedPlace` stub to `addcity.go`**

Task 28 replaces this with the real implementation.

```go
// commitSelectedPlace adds the highlighted result. Task 28 adds
// duplicate detection and config persistence.
func (m Model) commitSelectedPlace() (tea.Model, tea.Cmd) {
	if m.searchSel < 0 || m.searchSel >= len(m.searchResults) {
		return m, nil
	}
	m.cities = append(m.cities, m.searchResults[m.searchSel].ToCity())
	m.screen = screenGrid
	m.search.Blur()
	m.searchResults = nil
	return m.refreshCmd()
}
```

- [ ] **Step 8: Run the tests**

Run: `go test ./internal/ui/ -v -race`
Expected: PASS

- [ ] **Step 9: Commit**

```bash
git add internal/ui/
git commit -m "feat: add city search modal with debounced geocoding"
```

---

### Task 28: Persisting adds, deletes, and duplicate detection

**Files:**
- Create: `internal/ui/cities.go`, `internal/ui/cities_test.go`
- Modify: `internal/ui/addcity.go` (real `commitSelectedPlace`), `internal/ui/update.go` (delete key + confirm)

**Interfaces:**
- Consumes: `store.SaveConfig` (Task 5)
- Produces: `(Model).addCity(c weather.City) (Model, error)`, `(Model).deleteSelectedCity() Model`, `(Model).syncConfigCities() Model`, `ui.saveConfigCmd(path string, cfg store.Config) tea.Cmd`, `ui.nearDuplicate(cities []weather.City, c weather.City) (weather.City, bool)`; Model field `confirmDelete bool`

> **Why config write failures are surfaced but cache write failures are not:** a cache that fails to save costs one slow startup. A config that fails to save loses a city the user just added, and they will not find out until the next launch.

- [ ] **Step 1: Write the failing test**

Create `internal/ui/cities_test.go`:

```go
package ui

import (
	"path/filepath"
	"testing"

	"github.com/tsoueid/weatherdash/internal/store"
	"github.com/tsoueid/weatherdash/internal/weather"
)

func TestAddCityAppendsAndSyncsConfig(t *testing.T) {
	m := testModel(t, 2)
	m.configPath = filepath.Join(t.TempDir(), "config.toml")

	m, err := m.addCity(weather.City{
		ID: "om-1", Name: "Lisbon", Lat: 38.72, Lon: -9.14, TZ: "Europe/Lisbon",
	})
	if err != nil {
		t.Fatalf("addCity: %v", err)
	}
	if len(m.cities) != 3 {
		t.Fatalf("got %d cities, want 3", len(m.cities))
	}
	if len(m.cfg.Cities) != 3 {
		t.Errorf("config holds %d cities, want 3", len(m.cfg.Cities))
	}
}

// Two entries for the same place would produce two identical rows and
// two wasted coordinates in every request.
func TestAddCityRejectsNearDuplicate(t *testing.T) {
	m := testModel(t, 1)
	m.cities[0].Lat, m.cities[0].Lon = 48.8566, 2.3522

	_, err := m.addCity(weather.City{
		ID: "different-id", Name: "Paris", Lat: 48.8570, Lon: 2.3525,
	})
	if err == nil {
		t.Fatal("adding a place within the duplicate radius must error")
	}
}

func TestAddCityAllowsDistinctPlacesWithTheSameName(t *testing.T) {
	m := testModel(t, 1)
	m.cities[0].Name = "Paris"
	m.cities[0].Lat, m.cities[0].Lon = 48.8566, 2.3522
	m.configPath = filepath.Join(t.TempDir(), "config.toml")

	if _, err := m.addCity(weather.City{
		ID: "om-2", Name: "Paris", Lat: 33.66, Lon: -95.55,
	}); err != nil {
		t.Fatalf("Paris, Texas must be addable alongside Paris, France: %v", err)
	}
}

func TestDeleteSelectedCityRemovesAndClamps(t *testing.T) {
	m := testModel(t, 3)
	m.configPath = filepath.Join(t.TempDir(), "config.toml")
	m.cursorCity = 2
	removedID := m.cities[2].ID

	m = m.deleteSelectedCity()

	if len(m.cities) != 2 {
		t.Fatalf("got %d cities, want 2", len(m.cities))
	}
	if m.cursorCity != 1 {
		t.Errorf("cursorCity = %d, want 1 after deleting the last row", m.cursorCity)
	}
	if _, ok := m.data[removedID]; ok {
		t.Error("the deleted city's cached forecast must be dropped")
	}
	if len(m.cfg.Cities) != 2 {
		t.Errorf("config holds %d cities, want 2", len(m.cfg.Cities))
	}
}

func TestDeleteOnEmptyListIsSafe(t *testing.T) {
	m := New(defaultTestConfig(), emptyTestCache())
	m.w, m.h = 100, 30
	m = m.deleteSelectedCity() // must not panic

	if len(m.cities) != 0 {
		t.Errorf("got %d cities, want 0", len(m.cities))
	}
}

func TestSaveConfigCmdWritesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	cfg := store.DefaultConfig()
	cfg.Cities = []weather.City{{ID: "a", Name: "Paris", TZ: "Europe/Paris"}}

	if msg := saveConfigCmd(path, cfg)(); msg != nil {
		if e, ok := msg.(configSavedMsg); ok && e.err != nil {
			t.Fatalf("save reported %v", e.err)
		}
	}

	got, _, err := store.LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Cities) != 1 || got.Cities[0].Name != "Paris" {
		t.Errorf("round trip gave %+v", got.Cities)
	}
}

func TestDeleteRequiresConfirmation(t *testing.T) {
	m := send(testModel(t, 3), "d")
	if !m.confirmDelete {
		t.Fatal("d must arm a confirmation rather than deleting immediately")
	}
	if len(m.cities) != 3 {
		t.Errorf("got %d cities; nothing may be deleted before confirming", len(m.cities))
	}

	m.configPath = ""
	m = send(m, "d")
	if len(m.cities) != 2 {
		t.Errorf("got %d cities, want 2 after confirming", len(m.cities))
	}
	if m.confirmDelete {
		t.Error("the confirmation must be disarmed after acting")
	}
}

func TestDeleteConfirmationCancels(t *testing.T) {
	m := send(testModel(t, 3), "d", "esc")
	if m.confirmDelete {
		t.Error("esc must disarm the confirmation")
	}
	if len(m.cities) != 3 {
		t.Errorf("got %d cities, want 3", len(m.cities))
	}
}
```

- [ ] **Step 2: Run it to confirm it fails**

Run: `go test ./internal/ui/ -run TestAddCity`
Expected: FAIL — `m.addCity undefined`

- [ ] **Step 3: Write the implementation**

Create `internal/ui/cities.go`:

```go
package ui

import (
	"fmt"
	"math"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/tsoueid/weatherdash/internal/store"
	"github.com/tsoueid/weatherdash/internal/weather"
)

// duplicateRadius is how close two coordinates must be to count as the
// same place, in degrees — roughly a kilometre. Geocoding the same city
// twice can return very slightly different coordinates, so exact
// equality would not catch a genuine duplicate.
const duplicateRadius = 0.01

// configSavedMsg reports the outcome of a config write.
type configSavedMsg struct{ err error }

// nearDuplicate finds an existing city at essentially the same place.
func nearDuplicate(cities []weather.City, c weather.City) (weather.City, bool) {
	for _, existing := range cities {
		if math.Abs(existing.Lat-c.Lat) < duplicateRadius &&
			math.Abs(existing.Lon-c.Lon) < duplicateRadius {
			return existing, true
		}
	}
	return weather.City{}, false
}

// addCity appends a city and syncs the saved order.
//
// Duplicates are refused rather than silently merged: two rows for the
// same place would waste a coordinate slot in every request and give
// the user two identical lines with no way to tell them apart.
func (m Model) addCity(c weather.City) (Model, error) {
	if dup, ok := nearDuplicate(m.cities, c); ok {
		return m, fmt.Errorf("%s is already saved", dup.Name)
	}
	m.cities = append(m.cities, c)
	return m.syncConfigCities(), nil
}

// deleteSelectedCity removes the city under the cursor.
func (m Model) deleteSelectedCity() Model {
	city, ok := m.selectedCity()
	if !ok {
		return m
	}

	out := make([]weather.City, 0, len(m.cities)-1)
	for _, c := range m.cities {
		if c.ID != city.ID {
			out = append(out, c)
		}
	}
	m.cities = out
	delete(m.data, city.ID)

	m.clampCursor()
	m.ensureCursorVisible()
	return m.syncConfigCities()
}

// syncConfigCities copies the display order into the saved config.
//
// This is only ever called from add, delete, and manual reorder —
// never from a sort, which is a view transform. Letting a sort reach
// this function would destroy the user's manual order permanently.
func (m Model) syncConfigCities() Model {
	cities := make([]weather.City, len(m.cities))
	copy(cities, m.cities)
	m.cfg.Cities = cities
	return m
}

// saveConfigCmd persists the config.
//
// Unlike the cache, this error is reported: a failed cache write costs
// one slow startup, but a failed config write loses a city the user
// just added, and they would not find out until the next launch.
func saveConfigCmd(path string, cfg store.Config) tea.Cmd {
	if path == "" {
		return nil
	}
	return func() tea.Msg {
		return configSavedMsg{err: store.SaveConfig(path, cfg)}
	}
}
```

- [ ] **Step 4: Replace `commitSelectedPlace` in `internal/ui/addcity.go`**

```go
// commitSelectedPlace adds the highlighted result, persists it, and
// triggers a refresh so the new row fills in immediately.
func (m Model) commitSelectedPlace() (tea.Model, tea.Cmd) {
	if m.searchSel < 0 || m.searchSel >= len(m.searchResults) {
		return m, nil
	}

	next, err := m.addCity(m.searchResults[m.searchSel].ToCity())
	if err != nil {
		// Stay in the modal so the user can pick a different result.
		m.searchErr = err
		return m, nil
	}
	m = next

	m.screen = screenGrid
	m.search.Blur()
	m.search.SetValue("")
	m.searchResults = nil
	m.searchSel = 0
	m.searchErr = nil

	m, fetch := m.refreshCmd()
	return m, tea.Batch(saveConfigCmd(m.configPath, m.cfg), fetch)
}
```

- [ ] **Step 5: Add delete-with-confirmation to `internal/ui/update.go`**

Add `confirmDelete bool` to the `Model` struct, then add to `handleGridKey`:

```go
	case key.Matches(msg, m.keys.Delete):
		if !m.confirmDelete {
			// Deleting a city is cheap to redo but annoying to
			// discover, so it takes two presses rather than one.
			m.confirmDelete = true
			return m, nil
		}
		m.confirmDelete = false
		m = m.deleteSelectedCity()
		return m, saveConfigCmd(m.configPath, m.cfg)
```

Any other key must disarm it — add this at the top of `handleGridKey`:

```go
	if m.confirmDelete && !key.Matches(msg, m.keys.Delete) {
		m.confirmDelete = false
	}
```

And show it in the footer (`internal/ui/footer.go`), before the other cases:

```go
	case m.confirmDelete:
		if c, ok := m.selectedCity(); ok {
			status = th.warn.Render("press d again to delete " + c.Name + " · esc cancels")
		}
```

Add the `configSavedMsg` case to `Update`:

```go
	case configSavedMsg:
		if msg.err != nil {
			m.lastErr = msg.err
		}
		return m, nil
```

- [ ] **Step 6: Run the tests**

Run: `go test ./... -race`
Expected: PASS

- [ ] **Step 7: Verify by hand**

Run: `make build && ./weatherdash`

1. Press `a`, type `Paris` — two or more distinct results appear, each naming its region.
2. Select Paris, France and press `⏎` — it appears in the grid and fills with real data.
3. Press `a` and add Paris, France again — it is refused with "already saved".
4. Add Paris, Texas — accepted, because it is a different place.
5. Press `d` — the footer asks for confirmation; press `esc` and nothing is deleted.
6. Press `d` twice — the city is removed.
7. Quit and relaunch — the city list persisted.

- [ ] **Step 8: Commit**

```bash
git add internal/ui/
git commit -m "feat: persist city adds and deletes with duplicate detection"
```

---

# Phase 8 — Sort, Filter, Toggles, Help

### Task 29: Sort modes with cursor preservation

**Files:**
- Create: `internal/ui/sortmode.go`, `internal/ui/sortmode_test.go`
- Modify: `internal/ui/update.go` (`s` key), `internal/ui/cities.go` (call `rebuildView`)

**Interfaces:**
- Consumes: `Forecast.MeanHigh`, `Forecast.TotalPrecip` (Task 2)
- Produces: `ui.sortMode` with `sortManual`, `sortWarmest`, `sortColdest`, `sortDriest`; `(sortMode).String()`, `(sortMode).Next()`, `ui.parseSortMode(string) sortMode`; `(Model).rebuildView() Model`

> **Two invariants this task must hold, and a test pins each.** Sorting is a *view transform* — it rebuilds `m.cities` from `m.cfg.Cities` and never writes back, so manual order survives. And sorting preserves **which city** the cursor is on, not which index: otherwise pressing `s` teleports you to a different city, which feels like a bug even when the sort is correct.

- [ ] **Step 1: Write the failing test**

Create `internal/ui/sortmode_test.go`:

```go
package ui

import "testing"

func TestSortModeCycles(t *testing.T) {
	seen := map[sortMode]bool{}
	s := sortManual
	for i := 0; i < 4; i++ {
		seen[s] = true
		s = s.Next()
	}
	if len(seen) != 4 {
		t.Errorf("cycling visited %d modes, want 4", len(seen))
	}
	if s != sortManual {
		t.Errorf("cycling four times ended on %v, want back at sortManual", s)
	}
}

func TestSortWarmestOrdersByMeanHigh(t *testing.T) {
	// testModel gives city i a mean high that rises with i.
	m := testModel(t, 4)
	m.sortMode = sortWarmest
	m = m.rebuildView()

	first, last := m.cities[0].ID, m.cities[len(m.cities)-1].ID
	if first != m.cfg.Cities[3].ID {
		t.Errorf("warmest first = %q, want the highest-index city", first)
	}
	if last != m.cfg.Cities[0].ID {
		t.Errorf("warmest last = %q, want the lowest-index city", last)
	}
}

func TestSortColdestIsTheReverse(t *testing.T) {
	warm := testModel(t, 4)
	warm.sortMode = sortWarmest
	warm = warm.rebuildView()

	cold := testModel(t, 4)
	cold.sortMode = sortColdest
	cold = cold.rebuildView()

	if warm.cities[0].ID != cold.cities[3].ID {
		t.Error("coldest must be the reverse of warmest")
	}
}

// Sorting must never rewrite the saved order.
func TestSortDoesNotMutateConfig(t *testing.T) {
	m := testModel(t, 4)
	before := make([]string, len(m.cfg.Cities))
	for i, c := range m.cfg.Cities {
		before[i] = c.ID
	}

	m.sortMode = sortWarmest
	m = m.rebuildView()

	for i, c := range m.cfg.Cities {
		if c.ID != before[i] {
			t.Fatalf("config order changed at %d: %q, want %q", i, c.ID, before[i])
		}
	}
}

func TestSortReturnsToManualOrder(t *testing.T) {
	m := testModel(t, 4)
	m.sortMode = sortWarmest
	m = m.rebuildView()
	m.sortMode = sortManual
	m = m.rebuildView()

	for i, c := range m.cities {
		if c.ID != m.cfg.Cities[i].ID {
			t.Errorf("manual order not restored at %d: %q", i, c.ID)
		}
	}
}

// Pressing s must keep the cursor on the same CITY, not the same row.
func TestSortPreservesCursorCity(t *testing.T) {
	m := testModel(t, 5)
	m.cursorCity = 1
	want := m.cities[1].ID

	m.sortMode = sortWarmest
	m = m.rebuildView()

	if got := m.cities[m.cursorCity].ID; got != want {
		t.Errorf("cursor landed on %q, want it to follow %q", got, want)
	}
}

func TestSortPutsCitiesWithoutDataLast(t *testing.T) {
	m := testModel(t, 3)
	orphan := m.cities[0].ID
	delete(m.data, orphan)

	m.sortMode = sortWarmest
	m = m.rebuildView()

	if m.cities[len(m.cities)-1].ID != orphan {
		t.Errorf("city without data is at %v, want last", m.cities)
	}
}

func TestParseSortModeRoundTrip(t *testing.T) {
	for _, s := range []sortMode{sortManual, sortWarmest, sortColdest, sortDriest} {
		if got := parseSortMode(s.String()); got != s {
			t.Errorf("parseSortMode(%q) = %v, want %v", s.String(), got, s)
		}
	}
	if got := parseSortMode("nonsense"); got != sortManual {
		t.Errorf("unknown sort = %v, want sortManual", got)
	}
}
```

- [ ] **Step 2: Run it to confirm it fails**

Run: `go test ./internal/ui/ -run TestSortMode`
Expected: FAIL — `undefined: sortMode`

- [ ] **Step 3: Write the implementation**

Create `internal/ui/sortmode.go`:

```go
package ui

import (
	"sort"
	"strings"

	"github.com/tsoueid/weatherdash/internal/weather"
)

// sortMode is how the city axis is ordered.
type sortMode int

const (
	// sortManual is the user's saved order.
	sortManual sortMode = iota
	sortWarmest
	sortColdest
	sortDriest
)

func (s sortMode) String() string {
	switch s {
	case sortWarmest:
		return "warmest"
	case sortColdest:
		return "coldest"
	case sortDriest:
		return "driest"
	default:
		return "manual"
	}
}

func parseSortMode(s string) sortMode {
	switch s {
	case "warmest":
		return sortWarmest
	case "coldest":
		return sortColdest
	case "driest":
		return sortDriest
	default:
		return sortManual
	}
}

// Next cycles through the modes.
func (s sortMode) Next() sortMode {
	if s == sortDriest {
		return sortManual
	}
	return s + 1
}

// rebuildView regenerates the display order from the saved order.
//
// It is the ONLY place m.cities is derived, and it always starts from
// m.cfg.Cities. That is what keeps sorting a view transform: the saved
// order is never overwritten, so manual order is always one keypress
// away no matter how the list has been sorted.
//
// The cursor is carried across by CITY ID, not by row index. Preserving
// the index instead would teleport the user to a different city every
// time they pressed s, which reads as a bug even when the sort itself
// is correct.
func (m Model) rebuildView() Model {
	anchor := ""
	if c, ok := m.selectedCity(); ok {
		anchor = c.ID
	}

	out := make([]weather.City, 0, len(m.cfg.Cities))
	q := strings.ToLower(strings.TrimSpace(m.filterQuery))
	for _, c := range m.cfg.Cities {
		if q == "" || matchesFilter(c, q) {
			out = append(out, c)
		}
	}

	if m.sortMode != sortManual {
		sort.SliceStable(out, func(i, j int) bool {
			return m.lessBy(m.sortMode, out[i], out[j])
		})
	}

	m.cities = out

	// Re-anchor the cursor on the same city where possible.
	m.cursorCity = 0
	if anchor != "" {
		for i, c := range out {
			if c.ID == anchor {
				m.cursorCity = i
				break
			}
		}
	}

	m.clampCursor()
	m.ensureCursorVisible()
	return m
}

// lessBy ranks two cities under the active mode.
//
// All ranking keys are computed over the WHOLE forecast window, not the
// selected day: "driest" means driest week, which is the question a
// planning view is asking. Cities with no data rank last in every mode,
// so an unfetched row never displaces a real answer from the top.
func (m Model) lessBy(mode sortMode, a, b weather.City) bool {
	fa, oka := m.data[a.ID]
	fb, okb := m.data[b.ID]

	if oka != okb {
		return oka // the one with data sorts first
	}
	if !oka {
		return a.Name < b.Name
	}

	switch mode {
	case sortWarmest:
		if fa.MeanHigh() != fb.MeanHigh() {
			return fa.MeanHigh() > fb.MeanHigh()
		}
	case sortColdest:
		if fa.MeanHigh() != fb.MeanHigh() {
			return fa.MeanHigh() < fb.MeanHigh()
		}
	case sortDriest:
		if fa.TotalPrecip() != fb.TotalPrecip() {
			return fa.TotalPrecip() < fb.TotalPrecip()
		}
	}
	// Name is the tiebreak so the order is stable across refreshes.
	return a.Name < b.Name
}

// matchesFilter is defined in filter.go (Task 30). Until then it is a
// pass-through, declared here so rebuildView compiles.
```

Add the `filterQuery string` and `sortMode sortMode` fields to `Model`, initialise `sortMode: parseSortMode(cfg.Sort)` in `New`, and add a temporary stub at the bottom of `sortmode.go`:

```go
func matchesFilter(c weather.City, q string) bool { return true }
```

- [ ] **Step 4: Wire the `s` key in `handleGridKey`**

```go
	case key.Matches(msg, m.keys.Sort):
		m.sortMode = m.sortMode.Next()
		m.cfg.Sort = m.sortMode.String()
		m = m.rebuildView()
		return m, saveConfigCmd(m.configPath, m.cfg)
```

Also call `m = m.rebuildView()` at the end of `addCity` and `deleteSelectedCity` in `cities.go`, after `syncConfigCities`.

- [ ] **Step 5: Show the mode in the footer**

In `footer.go`, prefix the hints when sorting is active:

```go
	if m.sortMode != sortManual {
		hints = " sorted: " + m.sortMode.String() + " ·" + hints
	}
```

- [ ] **Step 6: Run the tests**

Run: `go test ./internal/ui/ -v -race`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/ui/
git commit -m "feat: add sort modes as a non-destructive view transform"
```

---

### Task 30: City filter

**Files:**
- Create: `internal/ui/filter.go`, `internal/ui/filter_test.go`
- Modify: `internal/ui/sortmode.go` (delete the `matchesFilter` stub), `internal/ui/update.go`, `internal/ui/view.go`

**Interfaces:**
- Consumes: `rebuildView` (Task 29)
- Produces: `ui.matchesFilter(c weather.City, q string) bool`, `(Model).handleFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd)`; Model field `filtering bool`

> **Why filtering matches name, country, and code:** at twenty cities the useful queries are as often "show me everything in France" as "jump to Oslo". Matching only the name would make the second work and the first fail silently.

- [ ] **Step 1: Write the failing test**

Create `internal/ui/filter_test.go`:

```go
package ui

import (
	"testing"

	"github.com/tsoueid/weatherdash/internal/weather"
)

func TestMatchesFilter(t *testing.T) {
	c := weather.City{Name: "Paris", Admin: "Île-de-France", Country: "France", CC: "FR"}
	for _, q := range []string{"par", "paris", "fran", "fr"} {
		if !matchesFilter(c, q) {
			t.Errorf("%q should match %+v", q, c)
		}
	}
	for _, q := range []string{"tokyo", "zzz"} {
		if matchesFilter(c, q) {
			t.Errorf("%q should not match %+v", q, c)
		}
	}
}

func TestFilterNarrowsTheGrid(t *testing.T) {
	m := testModel(t, 5)
	m.filterQuery = "tok"
	m = m.rebuildView()

	if len(m.cities) != 1 {
		t.Fatalf("got %d cities, want 1", len(m.cities))
	}
	if m.cities[0].Name != "Tokyo" {
		t.Errorf("filtered to %q, want Tokyo", m.cities[0].Name)
	}
}

// Filtering hides rows; it must never remove them from the saved list.
func TestFilterDoesNotTouchConfig(t *testing.T) {
	m := testModel(t, 5)
	m.filterQuery = "tok"
	m = m.rebuildView()

	if len(m.cfg.Cities) != 5 {
		t.Errorf("config holds %d cities, want all 5", len(m.cfg.Cities))
	}
}

func TestClearingTheFilterRestoresEveryRow(t *testing.T) {
	m := testModel(t, 5)
	m.filterQuery = "tok"
	m = m.rebuildView()
	m.filterQuery = ""
	m = m.rebuildView()

	if len(m.cities) != 5 {
		t.Errorf("got %d cities, want 5", len(m.cities))
	}
}

func TestFilterMatchingNothingIsSafe(t *testing.T) {
	m := testModel(t, 5)
	m.filterQuery = "zzzzz"
	m = m.rebuildView()

	if len(m.cities) != 0 {
		t.Fatalf("got %d cities, want 0", len(m.cities))
	}
	if m.cursorCity != 0 {
		t.Errorf("cursorCity = %d, want 0", m.cursorCity)
	}
	if out := m.renderGrid(); out == "" {
		t.Error("an empty filtered grid must still render chrome")
	}
}

func TestSlashOpensFilterAndEscClears(t *testing.T) {
	m := send(testModel(t, 5), "/")
	if !m.filtering {
		t.Fatal("/ must enter filter mode")
	}

	m.filterQuery = "tok"
	m = m.rebuildView()
	m = send(m, "esc")

	if m.filtering {
		t.Error("esc must leave filter mode")
	}
	if m.filterQuery != "" {
		t.Errorf("filterQuery = %q, want cleared", m.filterQuery)
	}
	if len(m.cities) != 5 {
		t.Errorf("got %d cities, want all 5 restored", len(m.cities))
	}
}
```

- [ ] **Step 2: Run it to confirm it fails**

Run: `go test ./internal/ui/ -run TestMatchesFilter`
Expected: FAIL — the stub matches everything

- [ ] **Step 3: Write the implementation**

Create `internal/ui/filter.go`, and **delete the `matchesFilter` stub from `sortmode.go`**:

```go
package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/tsoueid/weatherdash/internal/weather"
)

// matchesFilter reports whether a city matches a lowercased query.
//
// Name, region, country, and country code are all searched, because at
// twenty cities "show me everything in France" is as common a query as
// "jump to Oslo". Matching the name alone would make the second work
// and the first fail with no explanation.
func matchesFilter(c weather.City, q string) bool {
	if q == "" {
		return true
	}
	for _, field := range []string{c.Name, c.Admin, c.Country, c.CC} {
		if strings.Contains(strings.ToLower(field), q) {
			return true
		}
	}
	return false
}

// handleFilterKey routes keys while the filter input is active.
func (m Model) handleFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Back):
		// Escape clears the filter as well as leaving the mode. A
		// lingering invisible filter is how users end up convinced the
		// app lost their cities.
		m.filtering = false
		m.filter.Blur()
		m.filter.SetValue("")
		m.filterQuery = ""
		return m.rebuildView(), nil

	case msg.Type == tea.KeyEnter:
		// Keep the filter applied but return the keyboard to the grid.
		m.filtering = false
		m.filter.Blur()
		return m, nil
	}

	var cmd tea.Cmd
	m.filter, cmd = m.filter.Update(msg)
	m.filterQuery = m.filter.Value()
	return m.rebuildView(), cmd
}
```

- [ ] **Step 4: Wire it in `update.go`**

In `handleGridKey`:

```go
	case key.Matches(msg, m.keys.Filter):
		m.filtering = true
		m.filter.Focus()
		return m, textinput.Blink
```

At the top of `handleKey`, before the screen switch (so the filter input receives keys while the grid is still displayed):

```go
	if m.filtering && m.screen == screenGrid {
		return m.handleFilterKey(msg)
	}
```

Add `filtering bool` to `Model`.

- [ ] **Step 5: Show the filter in the grid**

In `renderGrid`, replace the separator line when filtering:

```go
	if m.filtering || m.filterQuery != "" {
		lines[1] = widgets.Pad(m.filter.View(), m.w)
	}
```

- [ ] **Step 6: Run the tests**

Run: `go test ./internal/ui/ -v -race`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/ui/
git commit -m "feat: add city filter across name, region, and country"
```

---

### Task 31: Density, units, and manual reorder

**Files:**
- Create: `internal/ui/toggles_test.go`
- Modify: `internal/ui/update.go`, `internal/ui/cities.go`

**Interfaces:**
- Consumes: `syncConfigCities`, `saveConfigCmd` (Task 28); `rebuildView` (Task 29)
- Produces: `(Model).moveCityInOrder(delta int) Model`

> **Why reorder is disabled outside manual sort:** dragging a row up a list that is sorted by temperature is incoherent — the next re-sort undoes it. The keys are refused with an explanation in the footer rather than silently ignored, so the user learns why instead of concluding the binding is broken.

- [ ] **Step 1: Write the failing test**

Create `internal/ui/toggles_test.go`:

```go
package ui

import (
	"path/filepath"
	"testing"

	"github.com/tsoueid/weatherdash/internal/units"
)

func TestUnitsToggle(t *testing.T) {
	m := testModel(t, 3)
	m.configPath = filepath.Join(t.TempDir(), "config.toml")

	m = send(m, "u")
	if m.units != units.Imperial {
		t.Errorf("units = %v, want Imperial", m.units)
	}
	if m.cfg.Units != "imperial" {
		t.Errorf("cfg.Units = %q, want imperial", m.cfg.Units)
	}

	m = send(m, "u")
	if m.units != units.Metric {
		t.Errorf("units = %v, want Metric", m.units)
	}
}

// Toggling units must not discard cached data: values are stored
// canonical-metric and converted at render time.
func TestUnitsToggleKeepsData(t *testing.T) {
	m := testModel(t, 3)
	before := len(m.data)
	m = send(m, "u")

	if len(m.data) != before {
		t.Errorf("data has %d entries after toggling, want %d", len(m.data), before)
	}
}

func TestDensityToggle(t *testing.T) {
	m := testModel(t, 3)
	m.configPath = filepath.Join(t.TempDir(), "config.toml")

	m = send(m, "z")
	if m.density != densityExpanded {
		t.Errorf("density = %v, want expanded", m.density)
	}
	if m.cfg.Density != "expanded" {
		t.Errorf("cfg.Density = %q, want expanded", m.cfg.Density)
	}
}

func TestReorderInManualMode(t *testing.T) {
	m := testModel(t, 4)
	m.configPath = filepath.Join(t.TempDir(), "config.toml")
	m.cursorCity = 2
	moved := m.cities[2].ID

	m = send(m, "K")

	if m.cities[1].ID != moved {
		t.Errorf("city is at index %d, want 1", indexOf(m, moved))
	}
	if m.cursorCity != 1 {
		t.Errorf("cursorCity = %d, want 1 — the cursor must follow the city", m.cursorCity)
	}
	if m.cfg.Cities[1].ID != moved {
		t.Error("a manual reorder MUST write through to the saved order")
	}
}

func TestReorderClampsAtEnds(t *testing.T) {
	m := testModel(t, 3)
	m.configPath = filepath.Join(t.TempDir(), "config.toml")
	m.cursorCity = 0
	first := m.cities[0].ID

	m = send(m, "K")
	if m.cities[0].ID != first || m.cursorCity != 0 {
		t.Error("moving the top city up must be a no-op")
	}
}

func TestReorderRefusedWhileSorted(t *testing.T) {
	m := testModel(t, 4)
	m.sortMode = sortWarmest
	m = m.rebuildView()
	before := m.cfg.Cities[0].ID

	m = send(m, "K")

	if m.cfg.Cities[0].ID != before {
		t.Error("reordering while sorted must not change the saved order")
	}
}

func indexOf(m Model, id string) int {
	for i, c := range m.cities {
		if c.ID == id {
			return i
		}
	}
	return -1
}
```

- [ ] **Step 2: Run it to confirm it fails**

Run: `go test ./internal/ui/ -run TestUnitsToggle`
Expected: FAIL — `u` does nothing

- [ ] **Step 3: Add the reorder to `internal/ui/cities.go`**

```go
// moveCityInOrder shifts the selected city within the SAVED order.
//
// This is one of only three operations that write to cfg.Cities (the
// others being add and delete). It is refused outside manual sort by
// its caller: reordering a temperature-sorted list is incoherent,
// because the next re-sort would undo it.
func (m Model) moveCityInOrder(delta int) Model {
	city, ok := m.selectedCity()
	if !ok {
		return m
	}

	idx := -1
	for i, c := range m.cfg.Cities {
		if c.ID == city.ID {
			idx = i
			break
		}
	}
	target := idx + delta
	if idx < 0 || target < 0 || target >= len(m.cfg.Cities) {
		return m
	}

	m.cfg.Cities[idx], m.cfg.Cities[target] = m.cfg.Cities[target], m.cfg.Cities[idx]
	return m.rebuildView()
}
```

- [ ] **Step 4: Wire the keys in `handleGridKey`**

```go
	case key.Matches(msg, m.keys.Units):
		m.units = m.units.Toggle()
		m.cfg.Units = m.units.String()
		return m, saveConfigCmd(m.configPath, m.cfg)

	case key.Matches(msg, m.keys.Density):
		if m.density == densityCompact {
			m.density = densityExpanded
		} else {
			m.density = densityCompact
		}
		m.cfg.Density = m.density.String()
		m.ensureCursorVisible()
		return m, saveConfigCmd(m.configPath, m.cfg)

	case key.Matches(msg, m.keys.MoveUp), key.Matches(msg, m.keys.MoveDown):
		if m.sortMode != sortManual {
			// Say why rather than swallowing the key, so the user
			// learns the rule instead of assuming the binding is broken.
			m.lastErr = errReorderWhileSorted
			return m, nil
		}
		delta := -1
		if key.Matches(msg, m.keys.MoveDown) {
			delta = 1
		}
		m = m.moveCityInOrder(delta)
		return m, saveConfigCmd(m.configPath, m.cfg)
```

Add to `cities.go`:

```go
// errReorderWhileSorted explains why K and J are inert under a sort.
var errReorderWhileSorted = errors.New("press s to return to manual order before reordering")
```

with `"errors"` imported.

- [ ] **Step 5: Add `u` to the detail screen too**

In `handleDetailKey`:

```go
	case key.Matches(msg, m.keys.Units):
		m.units = m.units.Toggle()
		m.cfg.Units = m.units.String()
		return m, saveConfigCmd(m.configPath, m.cfg)
```

- [ ] **Step 6: Run the tests**

Run: `go test ./internal/ui/ -v -race`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/ui/
git commit -m "feat: add units, density, and manual reorder"
```

---

### Task 32: Help overlay

**Files:**
- Create: `internal/ui/help.go`, `internal/ui/help_test.go`
- Modify: `internal/ui/update.go`, `internal/ui/view.go`

**Interfaces:**
- Consumes: `keyMap.FullHelp` (Task 16)
- Produces: `(Model).renderHelp() string`

> **Why the overlay is generated from the `keyMap` rather than written out:** a hand-maintained help screen drifts the moment a binding changes, and it drifts silently. Generating it means adding a binding automatically documents it.

- [ ] **Step 1: Write the failing test**

Create `internal/ui/help_test.go`:

```go
package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestHelpOpensAndCloses(t *testing.T) {
	m := send(testModel(t, 3), "?")
	if m.screen != screenHelp {
		t.Fatalf("screen = %v, want screenHelp", m.screen)
	}

	m = send(m, "esc")
	if m.screen != screenGrid {
		t.Errorf("screen = %v, want screenGrid", m.screen)
	}
}

func TestHelpReturnsToTheScreenItWasOpenedFrom(t *testing.T) {
	m := send(testModel(t, 3), "enter", "?", "esc")
	if m.screen != screenDetail {
		t.Errorf("screen = %v, want to return to screenDetail", m.screen)
	}
}

func TestHelpListsEveryBinding(t *testing.T) {
	m := send(testModel(t, 3), "?")
	out := m.renderHelp()

	for _, want := range []string{"add city", "filter", "sort", "units", "quit"} {
		if !strings.Contains(out, want) {
			t.Errorf("help is missing %q:\n%s", want, out)
		}
	}
}

func TestHelpLineWidths(t *testing.T) {
	m := send(testModel(t, 3), "?")
	for i, line := range strings.Split(m.renderHelp(), "\n") {
		if got := lipgloss.Width(line); got != m.w {
			t.Errorf("line %d width = %d, want %d", i, got, m.w)
		}
	}
}
```

- [ ] **Step 2: Run it to confirm it fails**

Run: `go test ./internal/ui/ -run TestHelpOpens`
Expected: FAIL — `?` does nothing

- [ ] **Step 3: Write the implementation**

Create `internal/ui/help.go`:

```go
package ui

import (
	"fmt"
	"strings"

	"github.com/tsoueid/weatherdash/internal/widgets"
)

// helpGroups names each row returned by keyMap.FullHelp.
var helpGroups = []string{"Navigation", "Cities", "Display"}

// renderHelp draws the full key reference.
//
// It is generated from the keyMap rather than written out by hand, so a
// new binding documents itself. A hand-maintained help screen drifts
// the moment a key changes, and it drifts silently.
func (m Model) renderHelp() string {
	th := newTheme()
	lines := []string{
		th.header.Render(widgets.Pad(" Keys", m.w)),
		widgets.Pad(strings.Repeat("─", m.w), m.w),
	}

	for i, group := range m.keys.FullHelp() {
		name := "Other"
		if i < len(helpGroups) {
			name = helpGroups[i]
		}
		lines = append(lines, th.label.Render(widgets.Pad("  "+name, m.w)))
		for _, b := range group {
			h := b.Help()
			lines = append(lines, widgets.Pad(
				fmt.Sprintf("    %s  %s",
					widgets.Pad(h.Key, 10), th.dim.Render(h.Desc)), m.w))
		}
		lines = append(lines, strings.Repeat(" ", m.w))
	}

	return m.fillWithFooter(lines, " esc or ? to close")
}
```

- [ ] **Step 4: Wire it in `update.go`**

Add `helpReturn screen` to `Model`, then in `handleKey`, before the screen switch:

```go
	if key.Matches(msg, m.keys.Help) {
		if m.screen == screenHelp {
			m.screen = m.helpReturn
		} else {
			m.helpReturn = m.screen
			m.screen = screenHelp
		}
		return m, nil
	}
	if m.screen == screenHelp {
		if key.Matches(msg, m.keys.Back) {
			m.screen = m.helpReturn
		}
		return m, nil
	}
```

- [ ] **Step 5: Dispatch in `View`**

```go
	case screenHelp:
		return m.renderHelp()
```

- [ ] **Step 6: Run the tests**

Run: `go test ./... -race && make lint`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/ui/
git commit -m "feat: add help overlay generated from key bindings"
```

---

# Phase 9 — Auto-Refresh

### Task 33: Refresh ticker

**Files:**
- Modify: `internal/ui/commands.go` (add `tickCmd`), `internal/ui/update.go` (handle `tickMsg`, start in `Init`)
- Create: `internal/ui/tick_test.go`

**Interfaces:**
- Consumes: `tickMsg` (Task 21), `refreshCmd` (Task 22), `Config.RefreshInterval` (Task 5)
- Produces: `ui.tickCmd(d time.Duration) tea.Cmd`

> **The bug this task must avoid:** rescheduling the tick only on the path where a refresh actually fires. Skip the reschedule when the refresh is suppressed — no cities yet, or a rate-limit backoff in effect — and the ticker dies permanently, so the app silently stops updating for the rest of the session. The reschedule must be unconditional.

- [ ] **Step 1: Write the failing test**

Create `internal/ui/tick_test.go`:

```go
package ui

import (
	"testing"
	"time"
)

func TestTickCmdProducesTickMsg(t *testing.T) {
	if _, ok := tickCmd(time.Millisecond)().(tickMsg); !ok {
		t.Error("tickCmd must produce a tickMsg")
	}
}

func TestTickTriggersRefreshAndReschedules(t *testing.T) {
	m := testModel(t, 3)
	m = m.WithDeps(Deps{Provider: &fakeProvider{}})
	before := m.reqID

	next, cmd := m.Update(tickMsg(time.Now()))
	m = next.(Model)

	if m.reqID != before+1 {
		t.Errorf("reqID = %d, want %d — the tick must fetch", m.reqID, before+1)
	}
	if cmd == nil {
		t.Fatal("the tick must return commands, including the next tick")
	}
}

// If the reschedule is conditional, a suppressed refresh kills the
// ticker for the rest of the session.
func TestTickRescheduesEvenWhenRefreshIsSuppressed(t *testing.T) {
	cases := map[string]func(Model) Model{
		"no cities": func(m Model) Model {
			m.cities = nil
			m.cfg.Cities = nil
			return m
		},
		"backoff": func(m Model) Model {
			m.backoffTil = time.Now().Add(time.Hour)
			return m
		},
		"no provider": func(m Model) Model {
			return m.WithDeps(Deps{})
		},
	}

	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			m := mutate(testModel(t, 3).WithDeps(Deps{Provider: &fakeProvider{}}))

			_, cmd := m.Update(tickMsg(time.Now()))
			if cmd == nil {
				t.Fatal("the next tick must still be scheduled")
			}
			if _, ok := cmd().(tickMsg); !ok {
				t.Error("the returned command must eventually produce a tickMsg")
			}
		})
	}
}

func TestInitStartsTheTicker(t *testing.T) {
	m := testModel(t, 2)
	m = m.WithDeps(Deps{Provider: &fakeProvider{}})

	if m.Init() == nil {
		t.Error("Init must return commands")
	}
}
```

- [ ] **Step 2: Run it to confirm it fails**

Run: `go test ./internal/ui/ -run TestTickCmd`
Expected: FAIL — `undefined: tickCmd`

- [ ] **Step 3: Add the command to `internal/ui/commands.go`**

```go
// tickCmd schedules the next auto-refresh.
//
// One batched request per interval costs roughly 96 calls a day at the
// default fifteen minutes — negligible against any rate limit, and the
// difference between a live dashboard and one quietly showing this
// morning's weather at six in the evening.
func tickCmd(d time.Duration) tea.Cmd {
	if d <= 0 {
		d = 15 * time.Minute
	}
	return tea.Tick(d, func(t time.Time) tea.Msg { return tickMsg(t) })
}
```

- [ ] **Step 4: Handle it in `Update`**

Add to the type switch:

```go
	case tickMsg:
		next, fetch := m.refreshCmd()
		m = next
		// The reschedule is UNCONDITIONAL. refreshCmd returns a nil
		// command whenever the fetch is suppressed — no cities yet, no
		// provider, or a rate-limit backoff — and tying the next tick
		// to that path would kill the ticker permanently the first
		// time any of those happened.
		cmds := []tea.Cmd{tickCmd(m.cfg.RefreshInterval())}
		if fetch != nil {
			cmds = append(cmds, fetch)
		}
		return m, tea.Batch(cmds...)
```

- [ ] **Step 5: Start it in `Init`**

```go
func (m Model) Init() tea.Cmd {
	next, fetch := m.refreshCmd()
	m = next

	cmds := []tea.Cmd{m.spinner.Tick, tickCmd(m.cfg.RefreshInterval())}
	if fetch != nil {
		cmds = append(cmds, fetch)
	}
	return tea.Batch(cmds...)
}
```

Note this also fixes the awkward `*(&m) = next` from Task 22 — `Init` has a value receiver, so reassigning `m` locally is sufficient and the earlier line was a no-op worth removing.

- [ ] **Step 6: Run the tests**

Run: `go test ./internal/ui/ -v -race`
Expected: PASS

- [ ] **Step 7: Verify by hand**

Run: `./weatherdash` with `refresh_minutes = 1` in config, and confirm the footer age resets to "just now" about once a minute without any keypress.

- [ ] **Step 8: Commit**

```bash
git add internal/ui/
git commit -m "feat: add auto-refresh ticker with unconditional rescheduling"
```

---

# Phase 10 — Polish and Golden Tests

### Task 34: Empty state

**Files:**
- Create: `internal/ui/empty.go`, `internal/ui/empty_test.go`
- Modify: `internal/ui/grid.go`

**Interfaces:**
- Produces: `(Model).renderEmpty() string`

> **Why the first run needs its own screen:** a new user launches the binary and sees a grid with no rows and no explanation. The empty state is the only onboarding this application has, so it must name the exact key to press.

- [ ] **Step 1: Write the failing test**

Create `internal/ui/empty_test.go`:

```go
package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestEmptyStateOnFirstRun(t *testing.T) {
	m := New(defaultTestConfig(), emptyTestCache())
	m.w, m.h = 100, 30

	out := m.View()
	if !strings.Contains(out, "a") || !strings.Contains(strings.ToLower(out), "add") {
		t.Errorf("the empty state must name the add key:\n%s", out)
	}
	for i, line := range strings.Split(out, "\n") {
		if got := lipgloss.Width(line); got != 100 {
			t.Errorf("line %d width = %d, want 100", i, got)
		}
	}
}

// A filter that hides every row is NOT a first run, and saying "add a
// city" there would be actively misleading.
func TestFilteredToNothingIsNotTheEmptyState(t *testing.T) {
	m := testModel(t, 5)
	m.filterQuery = "zzzzz"
	m = m.rebuildView()

	out := m.View()
	if strings.Contains(strings.ToLower(out), "add your first") {
		t.Errorf("a filtered-empty grid must not show the first-run message:\n%s", out)
	}
	if !strings.Contains(strings.ToLower(out), "match") {
		t.Errorf("a filtered-empty grid should say nothing matched:\n%s", out)
	}
}

func TestEmptyStateAtSmallSizes(t *testing.T) {
	m := New(defaultTestConfig(), emptyTestCache())
	m.w, m.h = 55, 16

	for i, line := range strings.Split(m.View(), "\n") {
		if got := lipgloss.Width(line); got != 55 {
			t.Errorf("line %d width = %d, want 55", i, got)
		}
	}
}
```

- [ ] **Step 2: Run it to confirm it fails**

Run: `go test ./internal/ui/ -run TestEmptyState`
Expected: FAIL — the grid renders bare chrome

- [ ] **Step 3: Write the implementation**

Create `internal/ui/empty.go`:

```go
package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/tsoueid/weatherdash/internal/widgets"
)

// renderEmpty draws the first-run and no-matches states.
//
// The two are deliberately distinct. "Add your first city" is correct
// on a fresh install and actively misleading when the user has twenty
// cities and a filter that happens to match none of them.
func (m Model) renderEmpty() string {
	th := newTheme()

	var body string
	if m.filterQuery != "" {
		body = lipgloss.JoinVertical(lipgloss.Center,
			th.label.Render("No cities match \""+m.filterQuery+"\""),
			"",
			th.dim.Render("esc  clear the filter"),
		)
	} else {
		body = lipgloss.JoinVertical(lipgloss.Center,
			th.header.Render("weatherdash"),
			"",
			th.label.Render("Add your first city to get started."),
			"",
			th.dim.Render("a   search for a city"),
			th.dim.Render("?   show all keys"),
			th.dim.Render("q   quit"),
		)
	}

	lines := strings.Split(
		lipgloss.Place(m.w, m.h-1, lipgloss.Center, lipgloss.Center, body), "\n")

	out := make([]string, 0, m.h)
	for _, l := range lines {
		out = append(out, widgets.Pad(l, m.w))
	}
	for len(out) < m.h-1 {
		out = append(out, strings.Repeat(" ", m.w))
	}
	out = append(out, m.footer(m.w))
	return strings.Join(out[:m.h], "\n")
}
```

- [ ] **Step 4: Dispatch from `renderGrid`**

At the top of `renderGrid`:

```go
	if len(m.cities) == 0 {
		return m.renderEmpty()
	}
```

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/ui/ -v -race`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/ui/
git commit -m "feat: add first-run and no-matches empty states"
```

---

### Task 35: Surfacing startup recovery

**Files:**
- Modify: `internal/ui/model.go` (notice field), `internal/ui/footer.go`, `cmd/weatherdash/main.go`
- Create: `internal/ui/notice_test.go`

**Interfaces:**
- Consumes: `store.Recovery` (Task 5)
- Produces: `(Model).WithNotice(text string) Model`; Model field `notice string`

> **Why this is its own task:** `LoadConfig` already backs up a corrupt file and recovers, but nothing tells the user. A person who launches the app and finds twenty cities gone deserves to know the file was salvaged and where it went — otherwise the recovery is indistinguishable from data loss.

- [ ] **Step 1: Write the failing test**

Create `internal/ui/notice_test.go`:

```go
package ui

import (
	"strings"
	"testing"
)

func TestNoticeAppearsInTheFooter(t *testing.T) {
	m := testModel(t, 2).WithNotice("config was corrupt; backed up to config.toml.bak")

	if !strings.Contains(m.footer(120), "backed up") {
		t.Errorf("footer = %q, want the notice", m.footer(120))
	}
}

// The notice is about startup, so it must outrank routine status.
func TestNoticeOutranksFreshness(t *testing.T) {
	m := testModel(t, 2).WithNotice("config recovered")
	m.lastOK = timeNow()

	if strings.Contains(m.footer(120), "updated") {
		t.Error("the notice must take precedence over the freshness line")
	}
}

func TestNoticeClearsOnKeypress(t *testing.T) {
	m := send(testModel(t, 2).WithNotice("config recovered"), "j")
	if m.notice != "" {
		t.Error("any keypress must dismiss the notice")
	}
}
```

Add to `model_test.go`:

```go
func timeNow() time.Time { return time.Now() }
```

- [ ] **Step 2: Run it to confirm it fails**

Run: `go test ./internal/ui/ -run TestNotice`
Expected: FAIL — `m.WithNotice undefined`

- [ ] **Step 3: Implement**

Add `notice string` to `Model` and this method to `internal/ui/model.go`:

```go
// WithNotice attaches a one-off startup message shown in the footer
// until the first keypress.
func (m Model) WithNotice(text string) Model {
	m.notice = text
	return m
}
```

In `footer.go`, add as the FIRST case of the status switch:

```go
	case m.notice != "":
		status = th.warn.Render(m.notice)
```

In `update.go`, at the top of `handleKey`:

```go
	// Any keypress acknowledges the startup notice.
	m.notice = ""
```

- [ ] **Step 4: Pass the recovery through in `cmd/weatherdash/main.go`**

```go
	cfg, rec, err := store.LoadConfig(cfgPath)
	if err != nil {
		return ui.Model{}, err
	}

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
```

- [ ] **Step 5: Run the tests and verify by hand**

Run: `go test ./... -race`

Then corrupt the config deliberately and confirm the notice appears and the backup exists:

```bash
cp ~/.config/weatherdash/config.toml /tmp/wd-backup.toml
echo "[[[ broken" >> ~/.config/weatherdash/config.toml
./weatherdash            # expect the notice in the footer
ls ~/.config/weatherdash/config.toml.bak
cp /tmp/wd-backup.toml ~/.config/weatherdash/config.toml
```

- [ ] **Step 6: Commit**

```bash
git add .
git commit -m "feat: surface config recovery to the user at startup"
```

---

### Task 36: Golden integration tests

**Files:**
- Create: `internal/ui/golden_test.go`, `internal/ui/testdata/golden/` (generated)

**Interfaces:**
- Consumes: the whole UI

> **What golden tests catch that unit tests do not:** the seams. Every widget can be individually correct while the assembled frame has a misaligned header, a footer on the wrong row, or a cursor highlight one column off. A full-frame snapshot is the only test that sees the composition.

- [ ] **Step 1: Write the golden test**

Create `internal/ui/golden_test.go`:

```go
package ui

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "rewrite golden files")

// assertGolden compares a rendered frame against a stored snapshot.
func assertGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", "golden", name+".txt")

	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("missing golden file %s — run: go test ./internal/ui/ -update", path)
	}
	if got != string(want) {
		t.Errorf("frame does not match %s\n--- got ---\n%s\n--- want ---\n%s",
			path, got, want)
	}
}

// goldenModel is deterministic: DemoModel's data is generated from a
// fixed formula, and every frame is rendered at a fixed size, so the
// only thing that can change a snapshot is a rendering change.
func goldenModel(t *testing.T, w, h int) Model {
	t.Helper()
	m := DemoModel()
	m.w, m.h = w, h
	return m
}

func TestGoldenGrid80x24(t *testing.T) {
	assertGolden(t, "grid_80x24", goldenModel(t, 80, 24).View())
}

func TestGoldenGrid120x40(t *testing.T) {
	assertGolden(t, "grid_120x40", goldenModel(t, 120, 40).View())
}

func TestGoldenGridExpanded(t *testing.T) {
	m := goldenModel(t, 120, 40)
	m.density = densityExpanded
	assertGolden(t, "grid_expanded_120x40", m.View())
}

func TestGoldenGridNarrow(t *testing.T) {
	assertGolden(t, "grid_60x20", goldenModel(t, 60, 20).View())
}

func TestGoldenEmptyState(t *testing.T) {
	m := New(defaultTestConfig(), emptyTestCache())
	m.w, m.h = 80, 24
	assertGolden(t, "empty_80x24", m.View())
}

func TestGoldenTooSmall(t *testing.T) {
	assertGolden(t, "too_small_30x10", goldenModel(t, 30, 10).View())
}

func TestGoldenHelp(t *testing.T) {
	m := goldenModel(t, 80, 24)
	m.helpReturn = screenGrid
	m.screen = screenHelp
	assertGolden(t, "help_80x24", m.View())
}

func TestGoldenAddCityModal(t *testing.T) {
	m := goldenModel(t, 80, 24)
	m.screen = screenAddCity
	m.search.SetValue("Paris")
	m.searchResults = parisResults()
	assertGolden(t, "addcity_80x24", m.View())
}
```

> **The detail screen is excluded from the golden set on purpose.** `renderDetail` draws a sun-position marker from `time.Now()`, so its frame changes during the day and the snapshot would fail at some hours and pass at others. It is covered by the width and content assertions in Task 24 instead. If a detail snapshot is wanted later, inject a clock rather than relaxing the comparison.

- [ ] **Step 2: Generate the snapshots**

Run: `go test ./internal/ui/ -run TestGolden -update`

- [ ] **Step 3: Inspect every generated file by eye**

Run: `cat internal/ui/testdata/golden/grid_80x24.txt`

Check each one: aligned columns, the header sitting above the right columns, the footer on the last line, the cursor highlight on the intended cell, no torn borders. **A golden file is only as good as this review** — committing a wrong frame locks the bug in and makes the test defend it.

- [ ] **Step 4: Confirm the tests pass against the snapshots**

Run: `go test ./internal/ui/ -run TestGolden`
Expected: PASS

- [ ] **Step 5: Confirm the snapshots are stable**

Run the suite twice more. Any frame that differs between runs has non-determinism in it (a clock, a map iteration) and must be fixed rather than tolerated.

Run: `go test ./internal/ui/ -run TestGolden -count=3`
Expected: PASS

- [ ] **Step 6: Full verification**

Run: `make test && make lint && make build`
Expected: every package passes with `-race`, `gofmt -l .` is silent, the binary builds

- [ ] **Step 7: Commit**

```bash
git add internal/ui/
git commit -m "test: add golden frame snapshots for every screen"
```

---

## Final Acceptance

Run through this once the last task is committed.

- [ ] `make test` passes with `-race`
- [ ] `make lint` is silent
- [ ] `./weatherdash --demo` renders eight cities and navigates correctly
- [ ] Add three real cities, including an ambiguous name (`Paris`) and a non-ASCII one (`Málaga`) — the picker disambiguates and the grid stays aligned
- [ ] Quit and relaunch — the grid paints from cache before the network responds
- [ ] Resize from 120 columns down to 55 and back — cell tiers degrade and recover cleanly
- [ ] Narrow below 50 columns — the "too small" message appears instead of torn output
- [ ] Disconnect the network and press `r` — stale markers appear, cached data remains
- [ ] Delete down to a single city and refresh — **exercises the single-coordinate object response live**
- [ ] `s` cycles sorts, keeps the cursor on the same city, and `s` back to manual restores the saved order
- [ ] `K`/`J` reorder in manual mode, are refused with an explanation while sorted, and persist across a relaunch
- [ ] `u` toggles units instantly with no refetch

