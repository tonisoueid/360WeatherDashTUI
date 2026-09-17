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
		condThunder: "☈", condUnknown: "?",
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
