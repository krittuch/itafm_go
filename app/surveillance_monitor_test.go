package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aerothai/itafm/model"
)

func newTestSurveillanceTrackStore(maxTracks int, ttl time.Duration, now *time.Time) *surveillanceTrackStore {
	store := newSurveillanceTrackStore(maxTracks, ttl)
	store.now = func() time.Time { return *now }
	return store
}

func recordTestTrack(store *surveillanceTrackStore, callsign string, outcome surveillanceOutcome, data *model.AODSSurveillance) {
	store.record(surveillanceReceiveResult{Outcome: outcome, RawCallSign: callsign, Data: data})
}

func TestSurveillanceTrackStoreEvictsLeastRecentlySeenAtCap(t *testing.T) {
	now := time.Date(2026, 10, 27, 10, 0, 0, 0, time.UTC)
	store := newTestSurveillanceTrackStore(2, time.Hour, &now)

	recordTestTrack(store, "THA1", surveillanceOutcomeQueued, nil)
	now = now.Add(time.Second)
	recordTestTrack(store, "THA2", surveillanceOutcomeQueued, nil)
	now = now.Add(time.Second)
	recordTestTrack(store, "THA1", surveillanceOutcomeDuplicate, nil)
	now = now.Add(time.Second)
	recordTestTrack(store, "THA3", surveillanceOutcomeQueued, nil)

	snap := store.snapshot(surveillanceTrackFilter{})
	if snap.Stored != 2 || snap.Evicted != 1 {
		t.Fatalf("stored=%d evicted=%d, want 2 and 1", snap.Stored, snap.Evicted)
	}
	if snap.Tracks[0].CallSign != "THA3" || snap.Tracks[1].CallSign != "THA1" {
		t.Fatalf("got %s,%s, want THA3,THA1", snap.Tracks[0].CallSign, snap.Tracks[1].CallSign)
	}
	if snap.Tracks[1].Messages != 2 || snap.Tracks[1].Outcome != surveillanceOutcomeDuplicate {
		t.Fatalf("THA1 messages=%d outcome=%s", snap.Tracks[1].Messages, snap.Tracks[1].Outcome)
	}
	if snap.OutcomeCounts[surveillanceOutcomeQueued] != 3 || snap.OutcomeCounts[surveillanceOutcomeDuplicate] != 1 {
		t.Fatalf("unexpected outcome counts %v", snap.OutcomeCounts)
	}
}

func TestSurveillanceTrackStoreDropsExpiredTracks(t *testing.T) {
	now := time.Date(2026, 10, 27, 10, 0, 0, 0, time.UTC)
	store := newTestSurveillanceTrackStore(10, time.Minute, &now)

	recordTestTrack(store, "THA1", surveillanceOutcomeQueued, nil)
	now = now.Add(30 * time.Second)
	recordTestTrack(store, "THA2", surveillanceOutcomeQueued, nil)
	now = now.Add(45 * time.Second)

	snap := store.snapshot(surveillanceTrackFilter{})
	if snap.Stored != 1 || snap.Tracks[0].CallSign != "THA2" {
		t.Fatalf("expected only THA2 to remain, got %+v", snap.Tracks)
	}
}

func TestSurveillanceTrackStoreCountsDecodeErrorsWithoutStoringTrack(t *testing.T) {
	now := time.Now()
	store := newTestSurveillanceTrackStore(10, time.Hour, &now)

	store.record(surveillanceReceiveResult{Outcome: surveillanceOutcomeDecodeError})

	snap := store.snapshot(surveillanceTrackFilter{})
	if snap.Stored != 0 || snap.OutcomeCounts[surveillanceOutcomeDecodeError] != 1 {
		t.Fatalf("stored=%d counts=%v", snap.Stored, snap.OutcomeCounts)
	}
}

func TestSurveillanceTrackStoreTruncatesLongFields(t *testing.T) {
	now := time.Now()
	store := newTestSurveillanceTrackStore(10, time.Hour, &now)

	long := strings.Repeat("X", 1000)
	recordTestTrack(store, long, surveillanceOutcomeNotVTBS, &model.AODSSurveillance{AircraftType: long})

	track := store.snapshot(surveillanceTrackFilter{}).Tracks[0]
	if len(track.CallSign) != surveillanceTrackMaxFieldLen || len(track.AircraftType) != surveillanceTrackMaxFieldLen {
		t.Fatalf("callsign len=%d actype len=%d", len(track.CallSign), len(track.AircraftType))
	}
}

func TestSurveillanceTrackFilter(t *testing.T) {
	now := time.Now()
	store := newTestSurveillanceTrackStore(10, time.Hour, &now)

	store.record(surveillanceReceiveResult{
		Outcome:      surveillanceOutcomeQueued,
		RawCallSign:  "THA123",
		IATACallSign: "TG123",
		Data:         &model.AODSSurveillance{Departure: "VTBS", Destination: "RJTT", AircraftType: "B77W", SSRCode: "2101", Altitude: 30000},
	})
	store.record(surveillanceReceiveResult{
		Outcome:     surveillanceOutcomeNotVTBS,
		RawCallSign: "AIQ456",
		Data:        &model.AODSSurveillance{Departure: "VTCC", Destination: "VTSP", AircraftType: "A320", Altitude: 35000},
	})

	cases := []struct {
		name   string
		filter surveillanceTrackFilter
		want   []string
	}{
		{"iata callsign", surveillanceTrackFilter{CallSign: "tg12"}, []string{"THA123"}},
		{"departure", surveillanceTrackFilter{Departure: "vtcc"}, []string{"AIQ456"}},
		{"outcome", surveillanceTrackFilter{Outcome: "not_vtbs"}, []string{"AIQ456"}},
		{"ssr", surveillanceTrackFilter{SSRCode: "210"}, []string{"THA123"}},
		{"altitude sort", surveillanceTrackFilter{Sort: "altitude"}, []string{"AIQ456", "THA123"}},
		{"limit", surveillanceTrackFilter{Sort: "callsign", Limit: 1}, []string{"AIQ456"}},
	}
	for _, tc := range cases {
		snap := store.snapshot(tc.filter)
		got := make([]string, 0, len(snap.Tracks))
		for _, track := range snap.Tracks {
			got = append(got, track.CallSign)
		}
		if strings.Join(got, ",") != strings.Join(tc.want, ",") {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestOnSurveillanceReceiveOutcomes(t *testing.T) {
	prevLookup := airlineIATALookup
	airlineIATALookup = map[string]string{"THA": "TG"}
	defer func() { airlineIATALookup = prevLookup }()

	writer := &fakeSurveillanceBatchWriter{success: true}
	batcher := newSurveillanceBatcher(writer, time.Hour, 100)
	defer batcher.close()

	cases := []struct {
		body string
		want surveillanceOutcome
	}{
		{`not json`, surveillanceOutcomeDecodeError},
		{`{"CallSign":"THA1","Dep":"VTCC","Dest":"VTSP"}`, surveillanceOutcomeNotVTBS},
		{`{"CallSign":"ZZZ1","Dep":"VTBS","Dest":"VTSP"}`, surveillanceOutcomeIATAFailed},
		{`{"CallSign":"THA1","Dep":"VTBS","Dest":"VTSP"}`, surveillanceOutcomeQueued},
		{`{"CallSign":"THA1","Dep":"VTBS","Dest":"VTSP"}`, surveillanceOutcomeDuplicate},
	}
	for _, tc := range cases {
		if got := onSurveillanceReceive([]byte(tc.body), batcher).Outcome; got != tc.want {
			t.Errorf("%s: got %s, want %s", tc.body, got, tc.want)
		}
	}
}

func TestHandleSurveillanceRendersHTMLAndJSON(t *testing.T) {
	now := time.Now()
	m := &gatewayMonitor{surveillanceTracks: newTestSurveillanceTrackStore(10, time.Hour, &now)}
	recordTestTrack(m.surveillanceTracks, "THA123", surveillanceOutcomeQueued, &model.AODSSurveillance{Departure: "VTBS"})

	req := httptest.NewRequest(http.MethodGet, "/aods-mon/surveillance?callsign=THA", nil)
	req.Header.Set("Accept", "text/html")
	rec := httptest.NewRecorder()
	m.handleSurveillance(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "THA123") {
		t.Fatalf("html status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/aods-mon/surveillance?format=json&outcome=not_vtbs", nil)
	rec = httptest.NewRecorder()
	m.handleSurveillance(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"matched":0`) {
		t.Fatalf("json status=%d body=%s", rec.Code, rec.Body.String())
	}
}
