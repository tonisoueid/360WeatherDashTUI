package ui

import (
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"

	"github.com/tsoueid/weatherdash/internal/geocode"
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
	screenMap
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

	// sortMode orders the city axis; filterQuery narrows it. Both are
	// view transforms: rebuildView derives m.cities from cfg.Cities and
	// never writes back, so the saved order always survives them.
	sortMode    sortMode
	filterQuery string

	w, h int

	// Collaborators and fetch bookkeeping. Deps are injected after New
	// so that tests and the demo mode can build a fully functional
	// model with no network at all.
	provider   weather.Provider
	geocoder   *geocode.Client
	configPath string
	cachePath  string
	reqID      int
	loading    bool
	lastOK     time.Time
	lastErr    error
	backoffTil time.Time

	// confirmDelete holds the delete-confirmation modal open over the
	// grid. Deleting a city is cheap to redo but annoying to discover
	// was gone, so it asks first.
	confirmDelete bool

	// filtering is true while the filter input has the keyboard.
	filtering bool

	// helpReturn is the screen ? was pressed on, so closing the overlay
	// returns to it instead of always landing on the grid.
	helpReturn screen

	// detailReturn is the screen ⏎ was pressed on to zoom into detail
	// — the grid or the map — so esc surfaces back to whichever one the
	// user actually came from. The zero value is screenGrid, which is
	// exactly today's behaviour when the map is never visited.
	detailReturn screen

	// notice is a one-off startup message (config recovery) shown in
	// the footer until the first keypress.
	notice string

	// startedAt anchors the notice grace window. A terminal answering
	// the colour queries sent at startup emits a burst of key-shaped
	// events that would otherwise dismiss the notice before the first
	// frame is flushed.
	startedAt time.Time

	// nowFunc supplies the clock to date-labelled rendering. Injecting
	// it keeps golden snapshots deterministic; nil means time.Now.
	nowFunc func() time.Time

	cfg     store.Config
	keys    keyMap
	help    help.Model
	spinner spinner.Model
	filter  textinput.Model

	// Add-city modal state. searchSeq is the sequence discipline for
	// debounced searches: every keystroke bumps it, and any debounce or
	// response carrying an older number is ignored.
	search        textinput.Model
	searchSeq     int
	searchResults []geocode.Place
	searchSel     int
	searchErr     error
	searching     bool
}

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

// WithNotice attaches a one-off startup message shown in the footer
// until the first keypress.
func (m Model) WithNotice(text string) Model {
	m.notice = text
	return m
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

	si := textinput.New()
	si.Placeholder = "city name"
	si.Prompt = "  search: "
	si.CharLimit = 64

	data := make(map[string]weather.Forecast, len(cache.Forecasts))
	for id, f := range cache.Forecasts {
		data[id] = f
	}

	cities := make([]weather.City, len(cfg.Cities))
	copy(cities, cfg.Cities)

	return Model{
		screen:    screenGrid,
		cities:    cities,
		data:      data,
		days:      cfg.ForecastDays,
		density:   parseDensity(cfg.Density),
		units:     u,
		icons:     icons,
		sortMode:  parseSortMode(cfg.Sort),
		startedAt: time.Now(),
		cfg:       cfg,
		keys:      defaultKeyMap(),
		help:      help.New(),
		spinner:   sp,
		filter:    fi,
		search:    si,
	}
}

// now returns the model's clock.
func (m Model) now() time.Time {
	if m.nowFunc != nil {
		return m.nowFunc()
	}
	return time.Now()
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
	if len(m.cities) == 0 {
		maxDay = 0
	}
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
