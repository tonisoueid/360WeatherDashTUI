# weatherdash — Design Spec

**Date:** 2026-09-17
**Status:** Approved for implementation planning
**Stack:** Go 1.26 · Bubble Tea · Lipgloss · Open-Meteo

---

## 1. Problem

No tool answers *"across the 15-20 places I care about, where is the weather good
this week — and what exactly happens on Friday in Tokyo?"* without opening twenty
browser tabs. `wttr.in` renders one city beautifully. Phone apps show one city at
a time. Nothing does the **comparison** view in a terminal.

## 2. Product shape

A single-binary TUI whose home screen is a **cities × days matrix**. Any cell
zooms into a full hourly breakdown for that one city on that one day. Saved
cities persist across runs. The grid paints instantly from cache at startup and
refreshes in the background.

### Goals

- Scan 10-20 cities × 7 days in one screen, without scrolling horizontally.
- Zoom from any cell into that city/day's hourly detail in one keystroke.
- Zero-configuration first run — no API key, no signup.
- Instant startup: render before the network responds.
- Never lose the saved city list, and never block or crash on network failure.

### Non-goals (v1)

Heatmap / comfort-scoring mode · non-interactive `--print` mode · city groups or
tags · air quality · moon phase · mouse support · weather alerts · radar.

Each is additive and none changes the architecture. The heatmap mode is the
natural v2: it reuses the sort comparators, since "driest week" and "best week"
are the same computation.

## 3. Key decisions

| Decision | Choice | Rationale |
|---|---|---|
| Data source | **Open-Meteo** | No API key → `go install` and it runs. Bundles a matching geocoding API. |
| Home screen | **Cities × days matrix** | The use case is planning and comparison, not single-city depth. |
| Drill-down | **Two levels** (matrix → cell) | A cell *is* the coordinate. No intermediate "city week" screen. |
| Scale target | **10-20 cities, 7 days** | Compact rows by default; city axis scrolls under a pinned header. |
| Unit storage | **Canonical metric** | Convert at render. Unit toggle is instant and never taints the cache. |
| Sorting | **View transform** | Never rewrites saved order, so manual order is always recoverable. |

### Provider alternatives considered

| Provider | Key? | Forecast | Verdict |
|---|---|---|---|
| **Open-Meteo** | No | 16 days hourly + daily | **Chosen.** Batched multi-coordinate requests; geocoding included. |
| met.no | No (UA required) | ~9 days | Good quality, strict caching etiquette, no batching. |
| OpenWeatherMap | Yes | 5 days / 3h free | Key requirement turns first run into a config wizard. |
| WeatherAPI.com | Yes | 3 days free | Same, plus a shorter horizon. |

The API-key property is a **UX decision wearing a technical costume**: requiring
a key means the first-run experience is a setup form instead of a dashboard.

---

## 4. Screens

### 4.1 Grid (home)

```
┌─ Outlook ─────────────────────────────────────────────────────┐
│            Today   Thu 18  Fri 19  Sat 20  Sun 21  Mon 22     │  ← pinned
├───────────────────────────────────────────────────────────────┤
│  Paris     ☁18     🌧15    🌧14    ☁17     ☀21     ☀23        │  ← scrolls
│  Beirut    ☀27     ☀28     ☀29     ☀28     ☁26     ☀27        │
│  Tokyo     🌧11   ┃🌧12┃    ☁14     ☀17     ☀19     ☁16        │  ← cursor
│  Oslo      ❄ 3     ❄ 2     ☁ 5     ☁ 6     ☀ 8     ☀ 9        │
│  Lisbon    ☀24     ☀25     ☁23     🌧21    ☀24     ☀26        │
└───────────────────────────────────────────────────────────────┘
  ↑↓ city · ←→ day · ⏎ zoom · a add · / filter · s sort · r · ? · updated 3m ago
```

`z` toggles **expanded density** — three lines per city: icon row, hi/lo row, and
a precipitation band using `·░▒▓█` scaled by probability. The precip band is the
feature that lets you locate a dry window without reading a number.

Temperature text is coloured on a blue → cyan → yellow → red ramp so the grid is
*scannable* before it is *readable*.

### 4.2 Detail (one city, one day)

```
┌─ Paris, France ──────────────────────── Fri 19 Sep ── day 3 of 7 ─┐
│   🌧    Rain, heavy at times            14° / 10°                  │
│         12mm total · wind to 28 km/h · UV 2                        │
│                                                                    │
│  Temp   ▁▁▂▂▃▃▄▄▅▅▆▆▆▆▅▅▄▄▃▃▂▂▁▁                                  │
│  Rain   ░░░░▒▒▓▓████████▓▓▒▒░░░░░░░░   ← probability               │
│         ▁▁▁▁▂▂▃▄▅▅▄▄▃▃▂▂▁▁▁▁▁▁▁▁▁▁     ← mm/h                      │
│  Wind   → → ↘ ↘ ↓ ↓ ↙ ↙ ← ← ← ↖                                   │
│          8  9 12 15 18 22 28 24 19 15 12 10 km/h                   │
│         00    03    06    09    12    15    18    21               │
│                                                                    │
│  ☀ 07:12 ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━ 19:48 🌙                 │
└────────────────────────────────────────────────────────────────────┘
  ←→ day · ↑↓ city · esc back · u units
```

The **cursor `{city, day}` is shared state between both screens.** Zooming in and
out never loses position, and `←→ / ↑↓` keep working inside detail — it is a
magnifying glass slid across the matrix, not a separate destination.

The "current conditions" hero line renders **only when the selected day is
today**; future days have no "now".

### 4.3 Add-city modal

Triggered by `a`. A text input over the grid; results listed as
`Paris · Île-de-France · France (48.85, 2.35)`. Disambiguation is mandatory —
there are roughly thirty places called Paris.

### 4.4 Help overlay

`?` toggles a full-screen overlay generated from the `keyMap`, so the binding
list cannot drift from the bindings.

---

## 5. Architecture

```
cmd/weatherdash/main.go       flag parsing, wiring, tea.NewProgram

internal/weather/
  types.go        City, Hour, Day, Forecast  (canonical metric)
  provider.go     Provider interface
  openmeteo.go    batched client + WMO code → label/icon mapping
internal/geocode/geocode.go   city search (Open-Meteo geocoding API)
internal/store/
  paths.go        XDG resolution via os.UserConfigDir / os.UserCacheDir
  config.go       TOML load/save, atomic
  cache.go        JSON load/save, atomic
internal/units/units.go       conversion + formatting (metric ⇄ imperial)

internal/ui/
  model.go  update.go  view.go
  grid.go         matrix renderer + layout math
  detail.go       zoomed-cell renderer
  addcity.go      search modal (textinput + results)
  keys.go         key.Binding map — also feeds the help overlay
  theme.go        lipgloss styles, temperature colour ramp
  sortmode.go     comparators

internal/widgets/               all pure: func(data, width) string
  sparkline.go  precipband.go  cell.go  windrow.go  sunarc.go  measure.go
```

**Dependencies:** `bubbletea`, `bubbles` (textinput, spinner, help, key),
`lipgloss`, `BurntSushi/toml`. Stdlib `net/http` + `encoding/json` for the API —
Open-Meteo needs no client library.

### 5.1 Batched fetching

Open-Meteo accepts comma-separated coordinates and resolves `timezone=auto`
**per coordinate**. All 20 cities are therefore **one HTTP request**:

```
https://api.open-meteo.com/v1/forecast
  ?latitude=48.85,33.89,35.68&longitude=2.35,35.50,139.69
  &timezone=auto&forecast_days=7
  &current=temperature_2m,apparent_temperature,weather_code,
           wind_speed_10m,wind_direction_10m,relative_humidity_2m
  &hourly=temperature_2m,apparent_temperature,precipitation_probability,
          precipitation,weather_code,wind_speed_10m,wind_direction_10m
  &daily=weather_code,temperature_2m_max,temperature_2m_min,precipitation_sum,
         precipitation_probability_max,wind_speed_10m_max,uv_index_max,
         sunrise,sunset
```

```go
type Provider interface {
    FetchAll(ctx context.Context, cities []City) ([]Forecast, error)
}
```

One `tea.Cmd`, one message, one atomic state swap. This eliminates per-city
in-flight tracking and stale-response races entirely.

**Two hazards this creates. Both are specified as test cases in §9.**

1. **A single coordinate returns a JSON object, not an array.** Decode into
   `json.RawMessage`, sniff the first non-whitespace byte for `[`, then branch.
   Without this, the happy path works until a user deletes down to one city.
2. **Results are positional.** The command must close over a **snapshot** of the
   city IDs, never read the live model inside the callback. Results convert to
   `map[cityID]Forecast` on arrival, so an add, delete, or reorder mid-flight
   cannot misalign rows. Reading the model inside the closure is the classic Go
   capture bug, and it surfaces as Oslo's weather appearing under Beirut only
   when a city is deleted during a refresh.

**Accepted trade-off:** failure is all-or-nothing; no partial success. Because
the design is cache-first, a failed refresh shows every row as cached-and-stale,
which is more honest than a grid where some rows are fresh with no way to tell.

### 5.2 Timezone semantics

"Friday" in Tokyo and "Friday" in Paris are different absolute time ranges. For
a **planning** matrix, aligning by each city's **local calendar day** is the
correct semantics — the user wants "what is Friday like there". Each city's IANA
timezone is resolved once at add time and stored; `timezone=auto` makes the API
return daily buckets already in local time.

### 5.3 Domain types

```go
type City struct {
    ID, Name, Admin, Country, CC string
    Lat, Lon float64
    TZ       string  // IANA, resolved once at add time
}

type Hour struct {
    Time time.Time
    TempC, ApparentC, PrecipMM, WindKPH float64
    PrecipProb, WindDir, Code int
}

type Day struct {
    Date             time.Time  // local calendar date
    Code             int
    HiC, LoC         float64
    PrecipMM         float64
    PrecipProb       int
    WindMaxKPH       float64
    UVMax            float64
    Sunrise, Sunset  time.Time
    Hours            []Hour     // 24, local
}

type Forecast struct {
    CityID    string
    Current   Hour
    Days      []Day
    FetchedAt time.Time
}
```

All values are canonical metric. `internal/units` converts at render time only,
so `u` toggles instantly and cached data is never unit-tainted.

### 5.4 Model and messages

```go
type screen int
const (screenGrid screen = iota; screenDetail; screenAddCity; screenHelp)

type Model struct {
    screen   screen
    cities   []weather.City             // display order (post-sort view)
    data     map[string]weather.Forecast
    cursor   struct{ city, day int }    // SHARED by grid and detail
    scroll   int                        // index of first visible row
    sortMode sortMode
    density  density                    // compact | expanded
    units    units.System
    filter   textinput.Model
    w, h     int
    reqID    int
    lastOK   time.Time
    lastErr  error
    spinner  spinner.Model
    keys     keyMap
    help     help.Model
}

type forecastsMsg struct {
    reqID     int
    cityIDs   []string   // snapshot the results map onto
    forecasts []weather.Forecast
    err       error
}
type tickMsg     time.Time
type debounceMsg struct{ seq int; query string }
type geocodeMsg  struct{ results []geocode.Place; err error }
```

`Update` discards any `forecastsMsg` whose `reqID` is not current. This guards a
manual `r` landing while the auto-refresh ticker's request is in flight.

### 5.5 Layout math

The grid header renders outside the scroll region and stays pinned; only city
rows scroll. `visibleRows = h - headerLines - footerLines`, with `scroll`
clamped to keep `cursor.city` in view under a one-row scrolloff.

Cell content is selected from available width, `cellW = (w - labelW) / days`:

| `cellW` | Content |
|---|---|
| ≥ 10 | `☀ 24/12 ░` |
| ≥ 7 | `☀ 24/12` |
| ≥ 5 | `☀24` |
| < 5 | drop day columns from the right (7 → 5 → 3) |
| `w` < 50 | fall back to a single-city list; the matrix is unusable |

**Every measurement uses `lipgloss.Width()`, never `len()`.** Weather emoji are
double-width and accented city names are multi-byte; `len()` drifts borders on
exactly the cities whose names have accents. The icon tier
(`ascii` / `unicode` / `nerdfont`) has a known glyph width that feeds `cellW`, so
changing tier changes layout math.

`forecast_days` is configurable from 3 to 16; the grid renders as many day
columns as width allows and the detail screen's "day N of M" reflects the
configured value.

---

## 6. Persistence

`~/.config/weatherdash/config.toml`, resolved via `os.UserConfigDir()`:

```toml
units           = "metric"     # metric | imperial
refresh_minutes = 15
density         = "compact"    # compact | expanded
icons           = "unicode"    # ascii | unicode | nerdfont
sort            = "manual"
forecast_days   = 7

[[city]]
id      = "01J..."
name    = "Paris"
admin   = "Île-de-France"
country = "France"
cc      = "FR"
lat     = 48.8566
lon     = 2.3522
tz      = "Europe/Paris"
```

`~/.cache/weatherdash/cache.json`:
`{"version": 1, "forecasts": {"<cityID>": {…, "fetched_at": "…"}}}`

TOML for config because humans hand-edit it; JSON for cache because only the
machine touches it.

**Atomic writes** for both: `os.CreateTemp` **in the target directory** → write →
`Sync()` → `os.Rename`. The temp file must share the filesystem or the rename
fails. A crash mid-save must never destroy a twenty-city list.

---

## 7. Runtime behaviour

### 7.1 Startup sequence

1. Load config; if absent, write a default with zero cities.
2. Load cache; corrupt or missing is non-fatal and treated as empty.
3. **The first `View()` draws the grid from cache**, with staleness markers.
4. `Init()` returns `tea.Batch(fetchCmd, tickCmd, spinner.Tick)`.
5. `forecastsMsg` arrives → merge into `data`, persist cache, re-render.

Perceived startup is instant rather than one network round-trip.

**Empty state** (first run): a centred panel explaining `a` to add a city — never
an empty grid.

### 7.2 Freshness indicators

The footer always states data age:

```
… · updated 3m ago              fresh, dim
… · ⠿ refreshing…               request in flight
… · ⚠ stale 2h · network down   failed, showing cache
```

Auto-refresh runs every `refresh_minutes` (default 15) via `tea.Tick`. One
batched request means this costs roughly 96 API calls per day in total.

### 7.3 Key bindings

| Key | Action |
|---|---|
| `↑↓` / `jk` | Move city cursor |
| `←→` / `hl` | Move day cursor |
| `⏎` | Zoom into cell · `esc` back |
| `a` | Add city · `d` delete (with confirm) |
| `/` | Filter visible cities |
| `s` | Cycle sort: manual → warmest → coldest → driest |
| `K` / `J` | Reorder city — **only in manual sort** |
| `z` | Toggle density |
| `u` | Toggle units |
| `r` | Refresh now |
| `?` | Help overlay · `q` quit |

`/` and `a` are deliberately distinct keys: filtering existing rows and searching
for new cities are different operations, and merging them creates mode confusion.

### 7.4 Sorting

```go
type sortMode int
const (sortManual sortMode = iota; sortWarmest; sortColdest; sortDriest)
```

Ranking keys, all computed over the **entire configured forecast window** rather
than the selected day: `sortWarmest`/`sortColdest` by mean daily high,
`sortDriest` by summed `PrecipMM`. Ties break on city name for stable ordering.

Sorting is a **view transform** over the stable `[[city]]` order in config; it
never rewrites that order, so manual order is always recoverable. Reordering
(`K`/`J`) is disabled outside `sortManual`, since reordering a sorted list is
incoherent — the footer says so rather than silently ignoring the key.

Sorting preserves **which city** the cursor is on, not its row index. Cities with
no data sort last in every mode.

### 7.5 Add-city flow

Each keystroke schedules `tea.Tick(250ms)` producing a `debounceMsg{seq}`; only
the latest `seq` issues a request, so typing does not fire one call per
character. Results come from
`geocoding-api.open-meteo.com/v1/search?name=…&count=10`.

Enter appends the city, persists config, and triggers a refresh. Latitude,
longitude, and timezone are resolved **once, here** — never re-geocoded on
refresh, which removes ambiguity permanently and saves a request per cycle.

**Duplicate detection:** coordinates within ~0.01° of an existing city produce a
warning rather than a silent second row.

---

## 8. Error handling

| Failure | Behaviour |
|---|---|
| Network down | Cached grid stays; footer `⚠ offline · stale 2h`; rows dimmed |
| HTTP 429 | Exponential backoff; footer notice; stop hammering |
| Malformed JSON | Keep cache; surface in footer |
| Geocode fails | Inline error in the modal; modal stays open |
| Config unwritable | Footer warning; app works, simply does not persist |
| Corrupt config | Back up to `config.toml.bak`, start fresh, tell the user |
| Terminal too small | `Terminal too small (need 50×15)`, not garbage output |

Nothing after startup is fatal. There are **zero** `log.Fatal` calls in the UI
layer.

---

## 9. Verification

### Unit and table-driven

- **Widgets** are pure `func(data, width) string`, so golden-string tests apply
  directly. Every case asserts `lipgloss.Width(out) == requested` across ASCII,
  accented, and emoji input.
- **`internal/units`**: round-trip conversions and formatting.
- **`internal/store`**: config and cache round-trip; a simulated crash mid-write
  leaves the original file intact; corrupt input recovers rather than panics.

### Provider

`httptest.Server` serving recorded fixtures covering:

- multi-city array response
- **single-city object response** (§5.1 hazard 1)
- HTTP 429
- malformed JSON
- null values inside an hourly series

### Update loop

`Update(Model, Msg) (Model, Cmd)` is a pure function, so these need no mocking:

- cursor clamps at all four grid edges
- scroll follows the cursor with scrolloff
- a `forecastsMsg` carrying a stale `reqID` is discarded
- deleting a city while a request is in flight does not misalign rows
  (§5.1 hazard 2)
- `s` preserves the cursor's **city**, not its index
- `tea.WindowSizeMsg` at 50, 80, and 120 columns selects the correct cell tier

### Integration

`teatest` golden full-frame renders at 80×24 and 120×40 for: empty first-run
state, cached-and-stale grid, loaded grid, detail screen, add-city modal.

### Manual smoke test

Run `go run ./cmd/weatherdash` and:

1. Add three cities, including one ambiguous name (`Paris`) and one non-ASCII
   name (`Málaga`).
2. Quit and relaunch — confirm the grid paints from cache before the network
   returns.
3. Resize the terminal narrow and wide; confirm cell tiers degrade cleanly.
4. Disconnect the network and press `r`; confirm stale markers rather than a
   crash or a blank grid.
5. Delete down to a single city and refresh (exercises §5.1 hazard 1 live).

---

## 10. Build order

Each step ends with something runnable or testable.

1. `internal/store` + `internal/units` — no dependencies, testable in isolation
2. `internal/weather` types + `openmeteo` provider against fixtures
3. `internal/widgets` — pure renderers with golden tests
4. **Grid screen with fake data** — the app becomes visible and navigable here,
   before any network code exists
5. Real data wiring + cache-first startup + atomic persistence
6. Detail screen (shared cursor)
7. Add-city + geocode + debounce
8. Sort, filter, density, units toggle, help overlay
9. Auto-refresh ticker + staleness indicators
10. Polish: empty state, resize degradation, error surfaces
