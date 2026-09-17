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
