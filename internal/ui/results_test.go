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
