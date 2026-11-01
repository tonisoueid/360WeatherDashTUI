# AGENTS.md

This file provides guidance to coding agents working with code in this repository.

## Project

`weatherdash` — a single-binary Bubble Tea TUI whose home screen is a **cities × days
weather matrix**. Any cell zooms into that city's hourly detail for that day. Data comes
from Open-Meteo (no API key). Module path: `github.com/tonisoueid/360WeatherDashTUI`.

Authoritative background, both worth reading before non-trivial work:

- `docs/superpowers/specs/2026-09-17-weatherdash-design.md` — design decisions and rationale
- `docs/superpowers/plans/2026-09-17-weatherdash.md` — the 36-task implementation plan

## Commands

```bash
make build                    # go build -o weatherdash ./cmd/weatherdash
make test                     # go test ./... -race
make lint                     # go vet ./... + gofmt -l .
make run                      # build, then ./weatherdash
./weatherdash --demo          # synthetic data, zero network — the way to eyeball the UI
./weatherdash --version

go test ./internal/ui/ -run TestGoldenGrid80x24    # one test
go test ./internal/ui/ -update                     # rewrite golden frame snapshots

go run ./tools/mapgen -out internal/widgets/worldmask.go   # regenerate the world map's
                                                            # land mask (see below) — run by
                                                            # hand, approximately never
```

`make build`, `go test ./... -race`, and `make lint` all passing is the verification gate
before reporting work done. Golden files are regenerated only when a rendering change is
intended — review the diff, never blind-`-update`.

The TUI needs a real terminal; to smoke-test non-interactively, drive it under a pty
(`script -qec "./weatherdash --demo" /dev/null`) rather than piping.

## Architecture

Dependency flow is strictly one-way: `store`/`units` depend on nothing internal, `widgets`
and `ui` sit on top.

```
cmd/weatherdash/main.go   flags, config+cache load, dependency injection, tea.NewProgram
internal/weather/         domain types (types.go), Provider iface, batched Open-Meteo
                          client (openmeteo.go), response decode (decode.go),
                          WMO code → condition/label/icon (wmo.go)
internal/geocode/         Open-Meteo city search for the add-city modal
internal/store/           XDG paths, TOML config, JSON cache, WriteAtomic
internal/units/           metric ⇄ imperial conversion + formatting
internal/widgets/         pure func(data, width) string renderers
internal/ui/              Bubble Tea model/update/view + per-screen renderers
tools/mapgen/             stdlib-only generator for internal/widgets/worldmask.go; never
                          run by the build, tests, or CI — see "The world map" below
```

### Invariants that span files

These are the rules a change is most likely to break silently:

- **All stored values are canonical metric** (°C, km/h, mm). Conversion happens only in
  `internal/units`, at render time. Nothing unit-converted is ever written to cache or
  config, so `u` toggles instantly and can never taint stored data.
- **Measure terminal width with `lipgloss.Width()`, never `len()`.** Weather glyphs are
  double-width and city names are multi-byte; `len()` drifts borders on exactly the rows
  with accented names. `widgets.Pad`/`PadLeft`/`Truncate` enforce this.
- **Sorting and filtering are view transforms.** They derive `Model.cities` from
  `cfg.Cities` and never write back, so the user's manual order is always recoverable.
  `K`/`J` reordering is therefore disabled outside `sortManual`.
- **Every config and cache write goes through `store.WriteAtomic`** — temp file in the
  *target directory*, fsync, rename. A different directory breaks atomicity with EXDEV.
- **Zero `log.Fatal`/`os.Exit` in `internal/ui` or below.** Only `main.go` exits, and only
  for pre-TUI startup failures. Everything after startup surfaces in the footer.
- **Dependencies are fixed:** bubbletea, bubbles, lipgloss, BurntSushi/toml, stdlib
  net/http + encoding/json. Adding a module is an explicit decision, not a silent `go get`.

### The batched fetch, and its two hazards

All cities are fetched in **one** HTTP request (Open-Meteo takes comma-separated
coordinates and resolves `timezone=auto` per coordinate). One `tea.Cmd`, one message, one
atomic state swap — no per-city in-flight tracking. Two traps, both covered by tests:

1. **A single coordinate returns a JSON object, not an array.** `decode.go` sniffs the
   first non-whitespace byte. Without it the happy path works until a user deletes down to
   one city.
2. **Results are positional.** `fetchCmd` *copies* the city slice into the closure and
   carries the ID snapshot in `forecastsMsg`; `Update` maps results onto that snapshot,
   never the live model. Reading live state here makes Oslo's weather appear under Beirut
   when a city is deleted mid-flight.

`Update` also discards any `forecastsMsg` whose `reqID` is stale, so a manual `r` landing
during the ticker's request cannot be overwritten by the older response.

### UI model notes

- **One shared `{cursorCity, cursorDay}`** drives both grid and detail. Detail is a
  magnifying glass over the matrix, not a separate destination — zooming never loses
  position, and arrows keep working inside it.
- **Cache-first startup:** `New` is handed the cache before the program runs, so the first
  `View()` draws a populated grid. `Init` only kicks a background refresh — and it does so
  by posting `refreshKickMsg` rather than fetching directly, because `Init` has a value
  receiver and any model mutation there lands on a discarded copy.
- **Deps are injected after `New`** (`WithDeps`), which is what lets tests and `--demo`
  build a fully functional model with no network.
- **Layout degrades by tier, not by truncation.** `widgets.TierFor(cellW)` picks one tier
  for the whole frame; when the configured window doesn't fit, day columns are dropped from
  the right. Below 50×15 the grid is replaced by a message.
- **Timezones:** each city's IANA zone is resolved once at add time and stored; days are
  the city's *local* calendar days. `main.go` blank-imports `time/tzdata` so this works on
  systems without tzdata.

### The world map (`m` on the grid)

A second home view: `internal/ui/worldmap.go` plots saved cities on a world map instead of
the matrix. It shares the grid's cursor and day axis like every other screen, but with one
deliberate exception — **it is not persisted**. `z` (density) and `u` (units) both mutate
`m.cfg` and return `saveConfigCmd`; the `Map` case in `handleGridKey` does neither, so
weatherdash always opens on the grid.

- **Land data is generated, not fetched at runtime.** `internal/widgets/worldmask.go` is a
  1°-resolution land/sea grid produced once by `tools/mapgen` from Natural Earth's public-
  domain 110m coastline data, then committed like any other source file. Regenerating it
  needs a network call; running weatherdash never does.
- **Rendering splits across the same seam as the flag rasterizer.** `internal/widgets`
  holds pure geometry — `MapSize`, `LandRaster`, `Project` — with no colour, matching the
  package's `func(data, width) string` contract. `internal/ui/worldmap.go` turns that into
  styled half-blocks, the same row-pairing technique `flags.go`'s `flagLines` uses.
- **The glyph carries the land/sea shape, colour is layered on top.** Unlike a flag (many
  hues, one glyph), a map only has two terrain colours, so `mapRasterGlyph` picks
  `█`/`▀`/`▄`/ ` ` from the actual land pattern. A monochrome terminal that strips all
  styling still shows a coastline; a flag rendered the same way would not.
- **`detailReturn`** (alongside the older `helpReturn`) tracks which screen `⏎` was pressed
  on, so `esc` from detail surfaces back to the map when that's where the zoom started,
  not unconditionally to the grid.
- Chip (temperature label) placement follows `internal/widgets/sparkline.go`'s `numberRow`
  convention: skip a value that would collide rather than draw over a neighbour. The
  marker dot is never skipped, only its adjacent number.

## Testing conventions

Testability is a design requirement here, not an afterthought — the code is shaped so that
almost nothing needs mocks:

- `internal/widgets` is pure `func(data, width) string`; tests assert
  `lipgloss.Width(out) == requested` across ASCII, accented, and emoji input. Widgets emit
  plain text (colour is applied by callers) so goldens compare characters, not ANSI escapes.
- `Update` is a pure function, so cursor clamping, scroll-off, stale `reqID` discarding and
  cursor-preserving sorts are tested directly with no program running.
- Provider and geocoder tests run against `httptest` servers serving `testdata/*.json`
  fixtures (multi-city array, single-city object, 429, malformed, null hourly values).
- Full-frame golden snapshots live in `internal/ui/testdata/golden/`. They are deterministic
  because `DemoModel` generates data from a fixed formula, `nowFunc` is pinned to
  `goldenClock`, and the lipgloss colour profile is forced to ANSI256 (under `go test`
  stdout is not a TTY, which would otherwise strip the cursor styling from the frame).

## Workflow

- Conventional commits, one per completed task: `feat:`, `fix:`, `test:`, `chore:`.
- Work is test-first, task-by-task against the plan document, with a clean tree per commit.
- Key bindings live in `internal/ui/keys.go` and feed both `Update` and the help overlay —
  add a binding there, never inline in `Update`, or the help screen drifts from reality.
