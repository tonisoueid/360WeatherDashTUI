package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/tonisoueid/360WeatherDashTUI/internal/weather"
	"github.com/tonisoueid/360WeatherDashTUI/internal/widgets"
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
	if !strings.Contains(out, "(day 4 of 7)") {
		t.Errorf("detail must show '(day 4 of 7)':\n%s", out)
	}
}

func TestRenderDetailHeaderFlagAndParens(t *testing.T) {
	m := testModel(t, 2)
	m.cities[0].Country = "France"
	m.cities[0].CC = "FR"
	m.cursorCity, m.cursorDay = 0, 2

	out := m.renderDetail()
	if !strings.Contains(out, "(day 3 of 7)") {
		t.Errorf("header must parenthesise the day counter:\n%s", out)
	}
	if strings.Contains(out, "— day 3 of 7") {
		t.Errorf("day counter must not be a dash segment:\n%s", out)
	}
	for _, l := range flagLines("FR") {
		if !strings.Contains(out, l) {
			t.Errorf("header must draw the flag block:\n%s", out)
			break
		}
	}

	// The title is bold orange on the grey banner.
	titleSt := newTheme().warn.Bold(true).Background(flagPanelColor)
	if !strings.Contains(out, titleSt.Render(" Paris · France — Sat 19 Sep (day 3 of 7)")) {
		t.Errorf("the title must be bold orange on the banner:\n%s", out)
	}

	// The flag block is three lines of art; the padding must still
	// land every line on exactly the terminal width.
	for i, line := range strings.Split(out, "\n") {
		if got := lipgloss.Width(line); got != m.w {
			t.Errorf("line %d width = %d, want %d: %q", i, got, m.w, line)
		}
	}
}

func TestRenderDetailHeaderFlagUpperRightTitleCentred(t *testing.T) {
	m := testModel(t, 2)
	m.cities[0].Country = "France"
	m.cities[0].CC = "FR"
	m.cursorCity, m.cursorDay = 0, 2

	flag := flagLines("FR")
	lines := strings.Split(m.renderDetail(), "\n")

	// The flag owns every row flush against the right edge; the title
	// sits on the flag's optically centred row beside it.
	titleRow := (len(flag) - 1) / 2
	for i := 0; i < len(flag); i++ {
		if !strings.Contains(lines[i], flag[i]) {
			t.Errorf("header row %d must carry the flag:\n%q", i, lines[i])
		}
		if !strings.HasSuffix(strings.TrimRight(lines[i], " "), strings.TrimRight(flag[i], " ")) {
			t.Errorf("header row %d must end with the flag, flush right:\n%q", i, lines[i])
		}
		if got := strings.Contains(lines[i], "Paris"); got != (i == titleRow) {
			t.Errorf("header row %d title presence = %v, want %v:\n%q", i, got, i == titleRow, lines[i])
		}
	}
}

// The header explains the temperature scale just before the flag: a
// vertical bar, hottest at the top, ends labelled, in ramp colours.
func TestRenderDetailHeaderTempScaleLegend(t *testing.T) {
	prev := lipgloss.ColorProfile()
	defer lipgloss.SetColorProfile(prev)
	lipgloss.SetColorProfile(termenv.ANSI256)

	m := testModel(t, 2)
	m.cities[0].Country = "France"
	m.cities[0].CC = "FR"
	m.cursorCity, m.cursorDay = 0, 2

	flag := flagLines("FR")
	lines := strings.Split(m.renderDetail(), "\n")

	if !strings.Contains(lines[0], "40°") {
		t.Errorf("the scale's hot end must label the top header row:\n%q", lines[0])
	}
	if !strings.Contains(lines[3], "-20°") {
		t.Errorf("the scale's cold end must label the bottom header row:\n%q", lines[3])
	}
	// The numbers are white on the banner, letting the bar carry the
	// colour.
	white := whiteOn(flagPanelColor)
	if !strings.Contains(lines[0], white.Render("40°")) {
		t.Errorf("the hot label must be white:\n%q", lines[0])
	}
	if !strings.Contains(lines[3], white.Render("-20°")) {
		t.Errorf("the cold label must be white:\n%q", lines[3])
	}
	for i, want := range map[int]string{0: "40°", 3: "-20°"} {
		li := strings.Index(lines[i], want)
		fi := strings.Index(lines[i], flag[i])
		if li < 0 || fi < 0 || li > fi {
			t.Errorf("the scale must come before the flag on row %d:\n%q", i, lines[i])
		}
	}

	// The bar is vertical: the middle rows carry ramp-coloured cells
	// with no labels of their own.
	if !strings.Contains(lines[1], "48;5;227") {
		t.Errorf("row below the top must carry the ramp's yellow half:\n%q", lines[1])
	}
	if !strings.Contains(lines[2], "38;5;42") {
		t.Errorf("row above the bottom must carry the ramp's green half:\n%q", lines[2])
	}

	// The whole header rides the same light grey as the flag's mount —
	// the panel sequence leading a run of spaces proves the banner
	// spans the block — and the scale keeps grey breathing room
	// before the flag.
	panelSeq, _, _ := strings.Cut(flagPanel.Render("x"), "x")
	gap := flagPanel.Render("   ")
	for i := 0; i < len(flag); i++ {
		if !strings.Contains(lines[i], panelSeq+strings.Repeat(" ", 20)) {
			t.Errorf("header row %d must carry the light-grey banner:\n%q", i, lines[i])
		}
		if !strings.Contains(lines[i], gap) {
			t.Errorf("header row %d must keep the scale spaced from the flag:\n%q", i, lines[i])
		}
	}
}

func TestRenderDetailHeaderWithoutCountryCode(t *testing.T) {
	m := testModel(t, 1)
	m.cursorCity, m.cursorDay = 0, 0

	// No country code in the flag table: no flag art anywhere, and no
	// stray gap where it would sit.
	out := m.renderDetail()
	if strings.Contains(out, "▀") {
		t.Errorf("header without a country code must be plain:\n%s", out)
	}
	if !strings.Contains(out, "Paris · X —") {
		t.Errorf("header must still name city and country:\n%s", out)
	}
}

// The day-summary hero is gone: the now line and the charts are the
// overview, and a second summary in the upper left would say the same
// thing twice. The live condition still reads — on the line's stamp,
// covered by the now-line test — but the day's totals do not render.
func TestRenderDetailHasNoHeroOverview(t *testing.T) {
	m := testModel(t, 2)
	f := m.data[m.cities[0].ID]
	f.Days[0].Code = 65 // Heavy rain
	f.Days[0].PrecipMM = 4.2
	f.Days[0].UVMax = 6
	m.data[m.cities[0].ID] = f

	out := m.renderDetail()
	for _, gone := range []string{"total", "UV"} {
		if strings.Contains(out, gone) {
			t.Errorf("the day-summary overview must not render (%q found):\n%s", gone, out)
		}
	}
}

func TestRenderDetailTimeSectionLeads(t *testing.T) {
	m := testModel(t, 2)
	f := m.data[m.cities[0].ID]
	f.Days[0].Sunrise = time.Date(2026, 9, 17, 6, 41, 0, 0, time.UTC)
	f.Days[0].Sunset = time.Date(2026, 9, 17, 19, 26, 0, 0, time.UTC)
	m.data[m.cities[0].ID] = f
	m.cursorCity = 0

	out := m.renderDetail()
	timeIx := strings.Index(out, "Time")
	temp := strings.Index(out, "Temp")
	wind := strings.Index(out, "Wind")
	if timeIx < 0 || temp < 0 || wind < 0 {
		t.Fatalf("detail must render the Time, Temp, and Wind sections:\n%s", out)
	}
	if !(timeIx < temp && temp < wind) {
		t.Errorf("sections must run Time, Temp, Rain, Wind — Time first:\n%s", out)
	}

	// The stamps ride above the ruler on their own row: sunrise keeps
	// the sun's gold, sunset goes orange.
	if !strings.Contains(out, condStyle(weather.CondClear).Render("☀↑06:41")) {
		t.Errorf("sunrise must be stamped gold above the ruler:\n%s", out)
	}
	if !strings.Contains(out, newTheme().warn.Bold(true).Render("☀↓19:26")) {
		t.Errorf("sunset must be stamped orange above the ruler:\n%s", out)
	}

	// Every hour of the day is labelled on the ruler row with its
	// unit, and nothing hides them: the stamps are not on it.
	rows := strings.Split(out, "\n")
	var rulerRow string
	for i, l := range rows {
		if strings.HasPrefix(strings.TrimLeft(l, " "), "Time") {
			rulerRow = rows[i+1]
			break
		}
	}
	if rulerRow == "" {
		t.Fatalf("the Time section must have a ruler row:\n%s", out)
	}
	for h := 0; h < 24; h++ {
		if !strings.Contains(rulerRow, fmt.Sprintf("%02dh", h)) {
			t.Errorf("the ruler must label hour %02dh:\n%q", h, rulerRow)
		}
	}
	if strings.Contains(rulerRow, "☀") {
		t.Errorf("the ruler row must be hours only, no stamps:\n%q", rulerRow)
	}

	// The hours and dots render bright and bold, not dim chrome.
	prev := lipgloss.ColorProfile()
	defer lipgloss.SetColorProfile(prev)
	lipgloss.SetColorProfile(termenv.ANSI256)
	if !strings.Contains(m.renderDetail(), "\x1b[1;38;5;250m") {
		t.Errorf("the Time ruler must be bold and bright:\n%s", m.renderDetail())
	}
}

// Every section ends in a blank line, so the four rulers don't stack
// into one dense block.
func TestRenderDetailSectionsHaveBreathingRoom(t *testing.T) {
	m := testModel(t, 2)
	lines := strings.Split(m.renderDetail(), "\n")

	find := func(label string) int {
		for i, l := range lines {
			if strings.HasPrefix(strings.TrimLeft(l, " "), label+" ") {
				return i
			}
		}
		return -1
	}
	// Rain runs the chance numbers, blank, mm/h before its trailing
	// blank.
	extraRows := map[string]int{"Time": 1, "Temp": 2, "Rain": 2, "Wind": 1}
	for label, extra := range extraRows {
		idx := find(label)
		if idx < 0 {
			t.Fatalf("detail must have a %s section:\n%s", label, strings.Join(lines, "\n"))
		}
		if after := lines[idx+extra+1]; strings.TrimRight(after, " ") != "" {
			t.Errorf("a blank line must follow the %s section, got %q", label, after)
		}
	}
}

// The Rain section explains itself: each hour's chance is printed as
// a figure with its own percent sign, under its hour mark and in the
// deepening blue of the chance it states, and the amount line sits
// below a blank line under its own mm/h label — two questions on one
// axis, visibly not one chart.
func TestRenderDetailRainSectionExplainsItself(t *testing.T) {
	prev := lipgloss.ColorProfile()
	defer lipgloss.SetColorProfile(prev)
	lipgloss.SetColorProfile(termenv.ANSI256)

	m := testModel(t, 2)
	f := m.data[m.cities[0].ID]
	f.Days[0].Hours = []weather.Hour{
		{PrecipProb: 5, PrecipMM: 0},
		{PrecipProb: 45, PrecipMM: 0.4},
		{PrecipProb: 90, PrecipMM: 2.1},
		{PrecipProb: 65, PrecipMM: 1.0},
	}
	m.data[m.cities[0].ID] = f
	m.cursorCity, m.cursorDay = 0, 0

	lines := strings.Split(m.renderDetail(), "\n")
	rain := -1
	for i, l := range lines {
		if strings.HasPrefix(strings.TrimLeft(l, " "), "Rain ") {
			rain = i
			break
		}
	}
	if rain < 0 {
		t.Fatalf("detail must have a Rain section:\n%s", strings.Join(lines, "\n"))
	}

	// Every hour's chance is on the line itself, each figure its own
	// rendered unit wearing the deepening blue of its own chance: the
	// 5% hour stays white, the 45% hour wears rain blue, the soaked
	// hours the deepest blue.
	for pct := range map[int]bool{5: true, 45: true, 90: true, 65: true} {
		figure := fmt.Sprintf("%d%%", pct)
		want := styleFor(precipColor(float64(pct))).Render(figure)
		if !strings.Contains(lines[rain], want) {
			t.Errorf("the %s figure must wear precipColor(%d):\n%q", figure, pct, lines[rain])
		}
	}
	if got := strings.TrimRight(lines[rain+1], " "); got != "" {
		t.Errorf("a blank line must separate chance from amount, got %q", lines[rain+1])
	}

	// The amount line says what it measures in its own gutter label,
	// and each rainy hour's figure wears the blue of its intensity:
	// the 0.4mm hour is drizzle-pale, the 2.1mm and 1.0mm hours wear
	// moderate rain's blue. The dry hour prints a bare dash, so the
	// row reads as a full day of measurements.
	amount := lines[rain+2]
	if !strings.HasPrefix(strings.TrimLeft(amount, " "), "mm/h ") {
		t.Errorf("the amount line must be labelled mm/h, got %q", amount)
	}
	for mm, figure := range map[float64]string{0.4: "0.4", 2.1: "2.1", 1.0: "1.0"} {
		want := styleFor(amountColor(mm)).Render(figure)
		if !strings.Contains(amount, want) {
			t.Errorf("the %smm figure must wear amountColor(%v):\n%q", figure, mm, amount)
		}
	}
	if !strings.Contains(amount, styleFor(amountColor(0)).Render("-")) {
		t.Errorf("the dry hour must print a dash in the row's family colour:\n%q", amount)
	}
	if strings.Contains(amount, "█") {
		t.Errorf("the amount line must be figures, not a normalised sparkline:\n%q", amount)
	}
}

// The amount gradient runs the meteorological intensity classes in
// the theme's blue family, with a violet stop for torrential hours.
func TestAmountColorBands(t *testing.T) {
	cases := []struct {
		mmph float64
		want lipgloss.Color
	}{
		{0.1, lipgloss.Color("153")}, // drizzle
		{0.9, lipgloss.Color("153")},
		{1, lipgloss.Color("39")}, // moderate rain
		{3.9, lipgloss.Color("39")},
		{4, lipgloss.Color("27")}, // heavy rain
		{9.9, lipgloss.Color("27")},
		{10, lipgloss.Color("57")}, // torrential
		{40, lipgloss.Color("57")},
	}
	for _, tc := range cases {
		if got := amountColor(tc.mmph); got != tc.want {
			t.Errorf("amountColor(%v) = %v, want %v", tc.mmph, got, tc.want)
		}
	}
}

// The rain gradient runs white through the theme's blues, breaking at
// the same decision boundaries the grid's shading uses.
func TestPrecipColorBands(t *testing.T) {
	cases := []struct {
		pct  float64
		want lipgloss.Color
	}{
		{0, lipgloss.Color("15")},   // white
		{9.9, lipgloss.Color("15")}, // barely a possibility
		{10, lipgloss.Color("153")}, // pale blue
		{29, lipgloss.Color("153")},
		{30, lipgloss.Color("39")}, // rain blue — "might rain"
		{59, lipgloss.Color("39")},
		{60, lipgloss.Color("27")}, // deep blue — "will rain"
		{100, lipgloss.Color("27")},
	}
	for _, tc := range cases {
		if got := precipColor(tc.pct); got != tc.want {
			t.Errorf("precipColor(%v) = %v, want %v", tc.pct, got, tc.want)
		}
	}
}

// The temperature chart is framed by the day's high and low: rules
// above and below, each carrying its value.
func TestRenderDetailTempBounds(t *testing.T) {
	m := testModel(t, 1)
	m.cursorCity, m.cursorDay = 0, 0 // day 0: HiC 10, LoC 2

	lines := strings.Split(m.renderDetail(), "\n")
	hiRow, loRow := "", ""
	for i, l := range lines {
		if strings.HasPrefix(strings.TrimLeft(l, " "), "Temp ") && i+2 < len(lines) {
			hiRow, loRow = l, lines[i+2]
		}
	}
	if hiRow == "" {
		t.Fatalf("detail must have a Temp section:\n%s", strings.Join(lines, "\n"))
	}
	if !strings.Contains(hiRow, "─") {
		t.Errorf("the high bound must be a rule:\n%q", hiRow)
	}
	if n := strings.Count(hiRow, "10°"); n != 2 {
		t.Errorf("the high bound must carry the day's high at both ends, found %d:\n%q", n, hiRow)
	}
	if !strings.Contains(loRow, "─") {
		t.Errorf("the low bound must be a rule:\n%q", loRow)
	}
	if n := strings.Count(loRow, "2°"); n != 2 {
		t.Errorf("the low bound must carry the day's low at both ends, found %d:\n%q", n, loRow)
	}
}

// The detail footer speaks in the same voice as the grid's: bold,
// one colour per function, dim separators.
func TestRenderDetailFooterColorfulHints(t *testing.T) {
	prev := lipgloss.ColorProfile()
	defer lipgloss.SetColorProfile(prev)
	lipgloss.SetColorProfile(termenv.ANSI256)

	m := testModel(t, 2)
	out := m.renderDetail()

	th := newTheme()
	seen := map[string]bool{}
	for _, h := range detailHints(th) {
		if !strings.Contains(out, h.style.Render(h.text)) {
			t.Errorf("detail footer must colour %q:\n%s", h.text, out)
		}
		seen[h.style.Render(h.text)] = true
	}
	if len(seen) != len(detailHints(th)) {
		t.Errorf("each detail hint must wear its own colour, got %d distinct", len(seen))
	}
	if !strings.Contains(out, th.dim.Render("·")) {
		t.Errorf("detail footer separators must stay dim:\n%s", out)
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

// Wind figures wear the colour of their strength, with the breaks at
// the Beaufort scale's own boundaries: white below "moderate breeze",
// yellow through "fresh breeze", orange for strong wind, red from
// gale force up.
func TestWindColorBands(t *testing.T) {
	cases := []struct {
		kph  float64
		want lipgloss.Color
	}{
		{0, lipgloss.Color("15")},    // white
		{19.9, lipgloss.Color("15")}, // still a gentle breeze
		{20, lipgloss.Color("227")},  // yellow from Beaufort 4
		{38, lipgloss.Color("227")},  // fresh breeze tops out
		{39, lipgloss.Color("214")},  // orange from Beaufort 6
		{61, lipgloss.Color("214")},  // near gale tops out
		{62, lipgloss.Color("196")},  // red from Beaufort 8, gale
		{120, lipgloss.Color("196")}, // storm
	}
	for _, tc := range cases {
		if got := windColor(tc.kph); got != tc.want {
			t.Errorf("windColor(%v) = %v, want %v", tc.kph, got, tc.want)
		}
	}
}

// The rendered detail colours the wind row's figures by the speed
// each states, so a dangerous gust stands out before its digits are
// read.
func TestRenderDetailWindFiguresColored(t *testing.T) {
	prev := lipgloss.ColorProfile()
	defer lipgloss.SetColorProfile(prev)
	lipgloss.SetColorProfile(termenv.ANSI256)

	m := testModel(t, 2)
	f := m.data[m.cities[0].ID]
	f.Days[0].Hours = []weather.Hour{
		{WindKPH: 5, WindDir: 0},
		{WindKPH: 75, WindDir: 180},
	}
	m.data[m.cities[0].ID] = f
	m.cursorCity, m.cursorDay = 0, 0

	out := m.renderDetail()
	if !strings.Contains(out, styleFor(windColor(75)).Render("75 km/h")) {
		t.Errorf("the gale-force figure must wear red:\n%s", out)
	}
	if !strings.Contains(out, styleFor(windColor(5)).Render("5 km/h")) {
		t.Errorf("the calm figure must wear white:\n%s", out)
	}
}

// The now line: a vertical rule at the city's current time on the
// charts' midnight-to-midnight scale, with the whole current read —
// condition and temperature, nothing else — as one line following its
// column.
func TestRenderDetailNowLine(t *testing.T) {
	prev := lipgloss.ColorProfile()
	defer lipgloss.SetColorProfile(prev)
	lipgloss.SetColorProfile(termenv.ANSI256)

	m := testModel(t, 2)
	m.nowFunc = func() time.Time { return time.Date(2026, 9, 18, 12, 30, 0, 0, time.UTC) }
	f := m.data[m.cities[0].ID]
	f.Current = weather.Hour{Code: 3, TempC: 15, ApparentC: 14, WindKPH: 22, WindDir: 90}
	m.data[m.cities[0].ID] = f
	m.cursorCity, m.cursorDay = 0, 1 // 2026-09-18 is today per the injected clock

	lines := strings.Split(m.renderDetail(), "\n")

	// chartW = 100-9-2 = 89 and 12:30 is minute 750 of 1440, so the
	// line sits at chart column 46; the chart body starts at screen
	// column 9, so x = 55.
	x := 9 + (12*60+30)*89/1440

	// The rule strikes through the chart area on that column.
	struck := 0
	for _, l := range lines {
		if widgets.CellAt(l, x) == '│' {
			struck++
		}
	}
	if struck < 3 {
		t.Errorf("the now line must strike through the chart area, found %d rows:\n%s",
			struck, strings.Join(lines, "\n"))
	}

	// The single current-conditions line rides the line's column in
	// the theme's bold bright white: condition and temperature only —
	// no feels-like, no wind.
	var overview string
	for _, l := range lines {
		if strings.Contains(l, "Now : Overcast - 15°") {
			overview = l
			break
		}
	}
	if overview == "" {
		t.Fatalf("the current read must be one line, %q:\n%s",
			"Now : Overcast - 15°", strings.Join(lines, "\n"))
	}
	if !strings.Contains(overview, newTheme().labelSel.Render("Now : Overcast - 15°")) {
		t.Errorf("the current read must be bold bright white:\n%q", overview)
	}
	if strings.Contains(overview, "feels") || strings.Contains(overview, "km/h") {
		t.Errorf("the current read must carry condition and temperature only:\n%q", overview)
	}
	if widgets.CellAt(overview, x) != 'N' || widgets.CellAt(overview, x-1) != ' ' {
		t.Errorf("the current read must start at the line's column %d:\n%q", x, overview)
	}
}

// A day that is not the city's today gets no now line and no
// current-conditions read.
func TestRenderDetailNoNowLineOnFutureDay(t *testing.T) {
	m := testModel(t, 2)
	m.nowFunc = func() time.Time { return time.Date(2026, 9, 18, 12, 30, 0, 0, time.UTC) }
	m.cursorCity, m.cursorDay = 0, 0 // 2026-09-17: yesterday to the injected clock

	out := m.renderDetail()
	if strings.Contains(out, "│") {
		t.Errorf("a day that is not today must not carry a now line:\n%s", out)
	}
	if strings.Contains(out, "Now :") {
		t.Errorf("a day that is not today must not show current conditions:\n%s", out)
	}
}

// At narrow widths the ruler drops labels rather than overwriting
// their neighbours: every hour token that survives keeps its unit.
func TestRenderDetailRulerNarrowWidthHonest(t *testing.T) {
	m := testModel(t, 2)
	m.w, m.h = 60, 20
	m.cursorCity, m.cursorDay = 0, 0

	rows := strings.Split(m.renderDetail(), "\n")
	var rulerRow string
	for i, l := range rows {
		if strings.HasPrefix(strings.TrimLeft(l, " "), "Time") {
			rulerRow = rows[i+1]
			break
		}
	}
	if rulerRow == "" {
		t.Fatalf("the Time section must have a ruler row:\n%s", strings.Join(rows, "\n"))
	}
	hours := 0
	for _, tok := range strings.Fields(rulerRow) {
		if len(tok) < 2 || tok[0] < '0' || tok[0] > '9' {
			continue
		}
		if !strings.HasSuffix(tok, "h") {
			t.Errorf("hour label %q must keep its unit, not overwrite its neighbour:\n%q", tok, rulerRow)
		}
		hours++
	}
	if hours < 8 {
		t.Errorf("a 60-column ruler must still label a good span of hours, got %d:\n%q", hours, rulerRow)
	}
}
