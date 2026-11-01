package ui

import (
	"context"
	"errors"
	"testing"

	"github.com/tsoueid/360WeatherDashTUI/internal/weather"
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
