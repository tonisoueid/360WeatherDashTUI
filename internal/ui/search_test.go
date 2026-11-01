package ui

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tonisoueid/360WeatherDashTUI/internal/geocode"
)

func TestDebounceCmdCarriesSeqAndQuery(t *testing.T) {
	cmd := debounceCmd("Par", 4)
	msg := cmd().(debounceMsg)
	if msg.seq != 4 || msg.query != "Par" {
		t.Errorf("debounceMsg = %+v, want seq 4 query Par", msg)
	}
}

func TestGeocodeCmdReturnsResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"results":[{"id":1,"name":"Paris","country":"France",
			"country_code":"FR","admin1":"Île-de-France","latitude":48.85,
			"longitude":2.35,"timezone":"Europe/Paris"}]}`))
	}))
	defer srv.Close()

	g := geocode.NewClient()
	g.BaseURL = srv.URL

	msg := geocodeCmd(g, "Paris", 2)().(geocodeMsg)
	if msg.err != nil {
		t.Fatalf("err = %v", msg.err)
	}
	if msg.seq != 2 {
		t.Errorf("seq = %d, want 2", msg.seq)
	}
	if len(msg.results) != 1 || msg.results[0].Name != "Paris" {
		t.Errorf("results = %+v", msg.results)
	}
}

func TestGeocodeCmdReportsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	g := geocode.NewClient()
	g.BaseURL = srv.URL

	if msg := geocodeCmd(g, "Paris", 1)().(geocodeMsg); msg.err == nil {
		t.Error("a failing search must report an error")
	}
}

func TestGeocodeCmdNilClient(t *testing.T) {
	if msg := geocodeCmd(nil, "Paris", 1)().(geocodeMsg); msg.err == nil {
		t.Error("a nil geocoder must produce an error, not a panic")
	}
}
