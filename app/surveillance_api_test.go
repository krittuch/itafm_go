package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aerothai/itafm/model"
)

type fakeSurveillancePositionReader struct {
	position  *model.SurveillancePosition
	err       error
	callsigns []string
}

func (f *fakeSurveillancePositionReader) GetSurveillancePosition(_ context.Context, callsigns []string) (*model.SurveillancePosition, error) {
	f.callsigns = callsigns
	return f.position, f.err
}

func callSurveillancePositionAPI(keys []string, reader surveillancePositionReader, apiKey, query string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/aods-mon/api/surveillance/position?"+query, nil)
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}
	rec := httptest.NewRecorder()
	withSurveillanceAPIKey(keys, handleSurveillancePosition(reader))(rec, req)
	return rec
}

func TestSurveillancePositionAPIAuth(t *testing.T) {
	reader := &fakeSurveillancePositionReader{}

	if rec := callSurveillancePositionAPI(nil, reader, "anything", "callsign=TG661"); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("no keys configured: got %d, want 503", rec.Code)
	}
	if rec := callSurveillancePositionAPI([]string{"k1"}, reader, "", "callsign=TG661"); rec.Code != http.StatusUnauthorized {
		t.Fatalf("missing key: got %d, want 401", rec.Code)
	}
	if rec := callSurveillancePositionAPI([]string{"k1"}, reader, "wrong", "callsign=TG661"); rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong key: got %d, want 401", rec.Code)
	}
	if rec := callSurveillancePositionAPI([]string{"k1", "k2"}, reader, "k2", "callsign=TG661"); rec.Code != http.StatusNotFound {
		t.Fatalf("second key: got %d, want 404 (authorised, no row)", rec.Code)
	}
}

func TestSurveillancePositionAPIReturnsPosition(t *testing.T) {
	prevLookup := airlineIATALookup
	airlineIATALookup = map[string]string{"THA": "TG"}
	defer func() { airlineIATALookup = prevLookup }()

	reader := &fakeSurveillancePositionReader{position: &model.SurveillancePosition{
		CallSign:     "TG661",
		Departure:    "VTBS",
		Destination:  "RJTT",
		Lat:          13.69,
		Lon:          100.75,
		Altitude:     12000,
		Heading:      45,
		PositionTime: "2026-10-27T10:00:00Z",
		UpdatedAt:    time.Date(2026, 10, 27, 17, 0, 5, 0, time.FixedZone("ICT", 7*3600)),
	}}

	rec := callSurveillancePositionAPI([]string{"k1"}, reader, "k1", "callsign=tha661")
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d: %s", rec.Code, rec.Body.String())
	}
	if strings.Join(reader.callsigns, ",") != "THA661,TG661" {
		t.Fatalf("looked up %v, want [THA661 TG661]", reader.callsigns)
	}

	var got surveillancePositionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	want := surveillancePositionResponse{
		CallSign:     "TG661",
		Departure:    "VTBS",
		Destination:  "RJTT",
		Latitude:     13.69,
		Longitude:    100.75,
		Altitude:     12000,
		Heading:      45,
		PositionTime: "2026-10-27T10:00:00Z",
		LastUpdate:   "2026-10-27T10:00:05Z",
	}
	if got != want {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

func TestSurveillancePositionAPIErrors(t *testing.T) {
	keys := []string{"k1"}

	for _, query := range []string{"", "callsign=T", "callsign=TG%20661", "callsign=HS-ABC", "callsign=ABCDEFGHIJK"} {
		if rec := callSurveillancePositionAPI(keys, &fakeSurveillancePositionReader{}, "k1", query); rec.Code != http.StatusBadRequest {
			t.Errorf("%q: got %d, want 400", query, rec.Code)
		}
	}

	rec := callSurveillancePositionAPI(keys, &fakeSurveillancePositionReader{err: errors.New("db down")}, "k1", "callsign=TG661")
	if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), "db down") {
		t.Fatalf("db error: got %d %s", rec.Code, rec.Body.String())
	}

	if rec := callSurveillancePositionAPI(keys, nil, "k1", "callsign=TG661"); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("nil reader: got %d, want 503", rec.Code)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/surveillance/position?callsign=TG661", nil)
	req.Header.Set("X-API-Key", "k1")
	rec = httptest.NewRecorder()
	withSurveillanceAPIKey(keys, handleSurveillancePosition(&fakeSurveillancePositionReader{}))(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST: got %d, want 405", rec.Code)
	}
}

func TestSurveillanceAPIKeysFromEnv(t *testing.T) {
	t.Setenv("GATEWAY_SURVEILLANCE_API_KEYS", " k1 , ,k2,")
	if got := surveillanceAPIKeysFromEnv(); strings.Join(got, "|") != "k1|k2" {
		t.Fatalf("got %v", got)
	}
}
