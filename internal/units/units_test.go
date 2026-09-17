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
