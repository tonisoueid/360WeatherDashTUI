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
