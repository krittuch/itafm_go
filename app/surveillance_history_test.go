package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"aerothai/itafm/model"
)

type fakeSurveillanceHistoryReader struct {
	position  *model.SurveillancePosition
	err       error
	callsigns []string
	at        time.Time
}

func (f *fakeSurveillanceHistoryReader) GetSurveillancePositionAt(_ context.Context, callsigns []string, at time.Time) (*model.SurveillancePosition, error) {
	f.callsigns = callsigns
	f.at = at
	return f.position, f.err
}

func (f *fakeSurveillanceHistoryReader) retentionWindow() time.Duration {
	return 30 * time.Minute
}

func callSurveillanceHistoryAPI(history surveillanceHistoryReader, query string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/surveillance/position?"+query, nil)
	req.Header.Set("X-API-Key", "k1")
	rec := httptest.NewRecorder()
	withSurveillanceAPIKey([]string{"k1"}, handleSurveillancePosition(&fakeSurveillancePositionReader{}, history))(rec, req)
	return rec
}

func TestSurveillancePositionAPIAtUsesHistory(t *testing.T) {
	prevLookup := airlineIATALookup
	airlineIATALookup = map[string]string{"THA": "TG"}
	defer func() { airlineIATALookup = prevLookup }()

	at := time.Now().Add(-10 * time.Minute).UTC().Truncate(time.Second)
	seenAt := at.Add(2 * time.Second)
	history := &fakeSurveillanceHistoryReader{position: &model.SurveillancePosition{
		CallSign:  "TG661",
		Lat:       13.7,
		UpdatedAt: seenAt,
	}}

	rec := callSurveillanceHistoryAPI(history, "callsign=THA661&at="+url.QueryEscape(at.Format(time.RFC3339)))
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d: %s", rec.Code, rec.Body.String())
	}
	if !history.at.Equal(at) || strings.Join(history.callsigns, ",") != "THA661,TG661" {
		t.Fatalf("history called with %v %v", history.callsigns, history.at)
	}
	var got surveillancePositionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.LastUpdate != seenAt.Format(time.RFC3339) || got.Latitude != 13.7 {
		t.Fatalf("got %+v", got)
	}
}

func TestSurveillancePositionAPIAtErrors(t *testing.T) {
	history := &fakeSurveillanceHistoryReader{}
	tooOld := time.Now().Add(-31 * time.Minute).UTC().Format(time.RFC3339)
	future := time.Now().Add(5 * time.Minute).UTC().Format(time.RFC3339)

	for _, at := range []string{"yesterday", "2026-10-30 08:15:00", tooOld, future} {
		if rec := callSurveillanceHistoryAPI(history, "callsign=TG661&at="+url.QueryEscape(at)); rec.Code != http.StatusBadRequest {
			t.Errorf("at=%q: got %d, want 400", at, rec.Code)
		}
	}

	recent := url.QueryEscape(time.Now().Add(-time.Minute).UTC().Format(time.RFC3339))
	if rec := callSurveillanceHistoryAPI(nil, "callsign=TG661&at="+recent); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("no history: got %d, want 503", rec.Code)
	}
	if rec := callSurveillanceHistoryAPI(history, "callsign=TG661&at="+recent); rec.Code != http.StatusNotFound {
		t.Fatalf("no point: got %d, want 404", rec.Code)
	}
	rec := callSurveillanceHistoryAPI(&fakeSurveillanceHistoryReader{err: errors.New("db down")}, "callsign=TG661&at="+recent)
	if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), "db down") {
		t.Fatalf("db error: got %d %s", rec.Code, rec.Body.String())
	}
}

func TestSurveillanceHistoryBufferDropsOldestWhenFull(t *testing.T) {
	store := newSurveillanceHistoryStore(&sql.DB{}, time.Minute, time.Second, 2)
	for _, callsign := range []string{"TG1", "TG2", "TG3"} {
		store.record(&model.AODSSurveillance{CallSign: callsign})
	}
	store.record(&model.AODSSurveillance{})
	store.record(nil)

	if len(store.pending) != 2 || store.pending[0].CallSign != "TG2" || store.pending[1].CallSign != "TG3" || store.dropped != 1 {
		t.Fatalf("pending=%+v dropped=%d", store.pending, store.dropped)
	}

	var nilStore *surveillanceHistoryStore
	nilStore.record(&model.AODSSurveillance{CallSign: "TG1"})
}

func TestSurveillanceHistoryPartitionName(t *testing.T) {
	start := time.Date(2026, 10, 30, 8, 10, 0, 0, time.UTC)
	name := surveillanceHistoryPartitionName(start)
	if name != "surveillance_track_point_p20261030_0810" {
		t.Fatalf("got %s", name)
	}
	if got, ok := parseSurveillanceHistoryPartitionName(name); !ok || !got.Equal(start) {
		t.Fatalf("parsed %v %v", got, ok)
	}
	if _, ok := parseSurveillanceHistoryPartitionName("other_table"); ok {
		t.Fatal("parsed an unrelated name")
	}
}

// Runs only with a disposable Postgres, e.g.
// SURVEILLANCE_HISTORY_TEST_DSN='postgres://postgres:postgres@localhost:55432/postgres?sslmode=disable'
func TestSurveillanceHistoryStoreAgainstPostgres(t *testing.T) {
	dsn := os.Getenv("SURVEILLANCE_HISTORY_TEST_DSN")
	if dsn == "" {
		t.Skip("SURVEILLANCE_HISTORY_TEST_DSN not set")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	_, _ = db.ExecContext(ctx, `DROP TABLE IF EXISTS `+surveillanceHistoryTable)

	clock := time.Date(2026, 10, 30, 8, 0, 0, 0, time.UTC)
	store := newSurveillanceHistoryStore(db, 30*time.Minute, time.Second, 100)
	store.now = func() time.Time { return clock }
	if err := store.setup(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.setup(ctx); err != nil {
		t.Fatalf("second setup: %v", err)
	}

	// Points every 5 s for 15 minutes, with a gap from 08:10:00 to 08:11:00.
	for clock.Before(time.Date(2026, 10, 30, 8, 15, 0, 0, time.UTC)) {
		if clock.Minute() != 10 {
			store.record(&model.AODSSurveillance{
				CallSign: "TG661", Departure: "VTBS", Destination: "RJTT",
				Lat: float64(clock.Unix() % 1000), Altitude: 12000, Heading: 45,
				DateTime: clock.Format(time.RFC3339),
			})
		}
		clock = clock.Add(5 * time.Second)
		if clock.Second() == 0 {
			if err := store.maintainPartitions(ctx); err != nil {
				t.Fatal(err)
			}
			store.flush(ctx)
		}
	}
	store.flush(ctx)

	var count int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM `+surveillanceHistoryTable).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 14*12 {
		t.Fatalf("stored %d points, want %d", count, 14*12)
	}

	var persistence string
	if err := db.QueryRowContext(ctx, `SELECT relpersistence FROM pg_class WHERE relname = $1`,
		surveillanceHistoryPartitionName(time.Date(2026, 10, 30, 8, 0, 0, 0, time.UTC))).Scan(&persistence); err != nil {
		t.Fatal(err)
	}
	if persistence != "u" {
		t.Fatalf("partition persistence %q, want unlogged", persistence)
	}

	cases := []struct {
		at   time.Time
		want time.Time
	}{
		{time.Date(2026, 10, 30, 8, 5, 2, 0, time.UTC), time.Date(2026, 10, 30, 8, 5, 0, 0, time.UTC)},
		{time.Date(2026, 10, 30, 8, 5, 3, 0, time.UTC), time.Date(2026, 10, 30, 8, 5, 5, 0, time.UTC)},
		{time.Date(2026, 10, 30, 8, 10, 40, 0, time.UTC), time.Date(2026, 10, 30, 8, 11, 0, 0, time.UTC)},
		{time.Date(2026, 10, 30, 8, 10, 10, 0, time.UTC), time.Date(2026, 10, 30, 8, 9, 55, 0, time.UTC)},
		{time.Date(2026, 10, 30, 7, 59, 0, 0, time.UTC), time.Date(2026, 10, 30, 8, 0, 0, 0, time.UTC)},
	}
	for _, c := range cases {
		got, err := store.GetSurveillancePositionAt(ctx, []string{"THA661", "TG661"}, c.at)
		if err != nil {
			t.Fatal(err)
		}
		if got == nil || !got.UpdatedAt.Equal(c.want) || got.PositionTime != c.want.Format(time.RFC3339) || got.Destination != "RJTT" {
			t.Fatalf("at %s: got %+v, want point at %s", c.at.Format(time.TimeOnly), got, c.want.Format(time.TimeOnly))
		}
	}
	if got, err := store.GetSurveillancePositionAt(ctx, []string{"TG999"}, clock); err != nil || got != nil {
		t.Fatalf("unknown callsign: %+v %v", got, err)
	}

	// 08:51: partitions 08:00 and 08:10 end at or before the 08:21 cutoff.
	clock = time.Date(2026, 10, 30, 8, 51, 0, 0, time.UTC)
	if err := store.maintainPartitions(ctx); err != nil {
		t.Fatal(err)
	}
	rows, err := db.QueryContext(ctx, `SELECT c.relname FROM pg_inherits i JOIN pg_class c ON c.oid = i.inhrelid
		WHERE i.inhparent = $1::regclass ORDER BY 1`, surveillanceHistoryTable)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		names = append(names, strings.TrimPrefix(name, surveillanceHistoryTable+"_p20261030_"))
	}
	rows.Close()
	if strings.Join(names, ",") != "0820,0830,0850,0900,0910" {
		t.Fatalf("partitions after cleanup: %v", names)
	}
}
