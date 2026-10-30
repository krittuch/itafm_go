package app

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"aerothai/itafm/model"

	"github.com/lib/pq"
)

const (
	surveillanceHistoryTable           = "surveillance_track_point"
	surveillanceHistoryPartitionSpan   = 10 * time.Minute
	surveillanceHistoryPartitionLayout = "20060102_1504"
	surveillanceHistoryMaintainEvery   = time.Minute
)

type surveillanceHistoryPoint struct {
	CallSign     string
	SeenAt       time.Time
	Departure    string
	Destination  string
	Lat          float64
	Lon          float64
	Altitude     float64
	GroundSpeed  float64
	Heading      float64
	PositionTime string
}

// surveillanceHistoryStore keeps every written surveillance point for a short
// retention window in the archive DB. Old data is removed by dropping whole
// time partitions rather than DELETE, so the table never needs vacuuming.
type surveillanceHistoryStore struct {
	db            *sql.DB
	retention     time.Duration
	flushInterval time.Duration
	maxBuffered   int
	now           func() time.Time

	mu      sync.Mutex
	pending []surveillanceHistoryPoint
	dropped uint64
}

func newSurveillanceHistoryStore(db *sql.DB, retention, flushInterval time.Duration, maxBuffered int) *surveillanceHistoryStore {
	if db == nil {
		return nil
	}
	if retention <= 0 {
		retention = 30 * time.Minute
	}
	if flushInterval <= 0 {
		flushInterval = 5 * time.Second
	}
	if maxBuffered <= 0 {
		maxBuffered = 20000
	}
	return &surveillanceHistoryStore{
		db:            db,
		retention:     retention,
		flushInterval: flushInterval,
		maxBuffered:   maxBuffered,
		now:           time.Now,
	}
}

func (s *surveillanceHistoryStore) start() error {
	if err := s.setup(context.Background()); err != nil {
		return err
	}
	go s.run()
	return nil
}

func (s *surveillanceHistoryStore) run() {
	flushTicker := time.NewTicker(s.flushInterval)
	maintainTicker := time.NewTicker(surveillanceHistoryMaintainEvery)
	defer flushTicker.Stop()
	defer maintainTicker.Stop()
	for {
		select {
		case <-flushTicker.C:
			s.flush(context.Background())
		case <-maintainTicker.C:
			if err := s.maintainPartitions(context.Background()); err != nil {
				log.Printf("surveillance history partition maintenance failed: %v", err)
			}
		}
	}
}

func (s *surveillanceHistoryStore) setup(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS `+surveillanceHistoryTable+` (
		callsign TEXT NOT NULL,
		seen_at TIMESTAMPTZ NOT NULL,
		departure TEXT,
		destination TEXT,
		latitude DOUBLE PRECISION,
		longitude DOUBLE PRECISION,
		altitude REAL,
		gspeed REAL,
		heading REAL,
		position_time TEXT
	) PARTITION BY RANGE (seen_at)`)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_`+surveillanceHistoryTable+`_callsign_seen_at
		ON `+surveillanceHistoryTable+` (callsign, seen_at)`)
	if err != nil {
		return err
	}
	return s.maintainPartitions(ctx)
}

func (s *surveillanceHistoryStore) record(surv *model.AODSSurveillance) {
	if s == nil || surv == nil || surv.CallSign == "" {
		return
	}
	point := surveillanceHistoryPoint{
		CallSign:     surv.CallSign,
		SeenAt:       s.now().UTC(),
		Departure:    surv.Departure,
		Destination:  surv.Destination,
		Lat:          surv.Lat,
		Lon:          surv.Lon,
		Altitude:     surv.Altitude,
		GroundSpeed:  surv.GroundSpeed,
		Heading:      surv.Heading,
		PositionTime: surv.DateTime,
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.pending) >= s.maxBuffered {
		s.pending = s.pending[1:]
		s.dropped++
	}
	s.pending = append(s.pending, point)
}

func (s *surveillanceHistoryStore) flush(ctx context.Context) {
	s.mu.Lock()
	batch := s.pending
	s.pending = nil
	dropped := s.dropped
	s.dropped = 0
	s.mu.Unlock()

	if dropped > 0 {
		log.Printf("surveillance history buffer full: dropped %d oldest points", dropped)
	}
	if len(batch) == 0 {
		return
	}
	// History is best effort: a failed batch is logged and dropped so a DB
	// outage cannot grow memory or hold up the flight_aircraftlocation writes.
	if err := s.insert(ctx, batch); err != nil {
		log.Printf("surveillance history insert of %d points failed: %v", len(batch), err)
	}
}

func (s *surveillanceHistoryStore) insert(ctx context.Context, batch []surveillanceHistoryPoint) error {
	n := len(batch)
	callsigns := make([]string, n)
	seenAt := make([]string, n)
	departures := make([]string, n)
	destinations := make([]string, n)
	lats := make([]float64, n)
	lons := make([]float64, n)
	altitudes := make([]float64, n)
	speeds := make([]float64, n)
	headings := make([]float64, n)
	positionTimes := make([]string, n)
	for i, p := range batch {
		callsigns[i] = p.CallSign
		seenAt[i] = p.SeenAt.UTC().Format(time.RFC3339Nano)
		departures[i] = p.Departure
		destinations[i] = p.Destination
		lats[i] = p.Lat
		lons[i] = p.Lon
		altitudes[i] = p.Altitude
		speeds[i] = p.GroundSpeed
		headings[i] = p.Heading
		positionTimes[i] = p.PositionTime
	}

	_, err := s.db.ExecContext(ctx, `INSERT INTO `+surveillanceHistoryTable+` (
		callsign, seen_at, departure, destination, latitude, longitude,
		altitude, gspeed, heading, position_time)
		SELECT * FROM unnest($1::text[], $2::timestamptz[], $3::text[], $4::text[],
			$5::float8[], $6::float8[], $7::float4[], $8::float4[], $9::float4[], $10::text[])`,
		pq.Array(callsigns), pq.Array(seenAt), pq.Array(departures), pq.Array(destinations),
		pq.Array(lats), pq.Array(lons), pq.Array(altitudes), pq.Array(speeds), pq.Array(headings),
		pq.Array(positionTimes),
	)
	return err
}

// maintainPartitions creates the partitions for the current and next two
// slots and drops every partition that ends before the retention window.
func (s *surveillanceHistoryStore) maintainPartitions(ctx context.Context) error {
	now := s.now().UTC()
	current := now.Truncate(surveillanceHistoryPartitionSpan)
	for i := 0; i < 3; i++ {
		start := current.Add(time.Duration(i) * surveillanceHistoryPartitionSpan)
		// Each partition is UNLOGGED: no WAL for short-lived data. A crash
		// empties it, which only loses at most the retention window.
		_, err := s.db.ExecContext(ctx, fmt.Sprintf(
			`CREATE UNLOGGED TABLE IF NOT EXISTS %s PARTITION OF %s FOR VALUES FROM ('%s') TO ('%s')`,
			surveillanceHistoryPartitionName(start),
			surveillanceHistoryTable,
			start.Format(time.RFC3339),
			start.Add(surveillanceHistoryPartitionSpan).Format(time.RFC3339),
		))
		if err != nil {
			return err
		}
	}

	rows, err := s.db.QueryContext(ctx, `SELECT c.relname
		FROM pg_inherits i
		JOIN pg_class c ON c.oid = i.inhrelid
		WHERE i.inhparent = $1::regclass`, surveillanceHistoryTable)
	if err != nil {
		return err
	}
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return err
		}
		names = append(names, name)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	cutoff := now.Add(-s.retention)
	for _, name := range names {
		start, ok := parseSurveillanceHistoryPartitionName(name)
		if !ok || start.Add(surveillanceHistoryPartitionSpan).After(cutoff) {
			continue
		}
		if _, err := s.db.ExecContext(ctx, `DROP TABLE IF EXISTS `+name); err != nil {
			return err
		}
	}
	return nil
}

func surveillanceHistoryPartitionName(start time.Time) string {
	return surveillanceHistoryTable + "_p" + start.UTC().Format(surveillanceHistoryPartitionLayout)
}

func parseSurveillanceHistoryPartitionName(name string) (time.Time, bool) {
	suffix, ok := strings.CutPrefix(name, surveillanceHistoryTable+"_p")
	if !ok {
		return time.Time{}, false
	}
	start, err := time.Parse(surveillanceHistoryPartitionLayout, suffix)
	if err != nil {
		return time.Time{}, false
	}
	return start, true
}

// GetSurveillancePositionAt returns the stored point closest to at for any of
// callsigns, or nil when there is none.
func (s *surveillanceHistoryStore) GetSurveillancePositionAt(ctx context.Context, callsigns []string, at time.Time) (*model.SurveillancePosition, error) {
	rows, err := s.db.QueryContext(ctx, `(SELECT callsign, departure, destination, latitude, longitude,
			altitude, heading, position_time, seen_at
		FROM `+surveillanceHistoryTable+`
		WHERE callsign = ANY($1) AND seen_at <= $2 AND seen_at >= $3
		ORDER BY seen_at DESC LIMIT 1)
		UNION ALL
		(SELECT callsign, departure, destination, latitude, longitude,
			altitude, heading, position_time, seen_at
		FROM `+surveillanceHistoryTable+`
		WHERE callsign = ANY($1) AND seen_at > $2
		ORDER BY seen_at ASC LIMIT 1)`,
		pq.Array(callsigns), at.UTC(), at.Add(-s.retention).UTC(),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nearest *model.SurveillancePosition
	for rows.Next() {
		var (
			callsign, departure, destination, positionTime sql.NullString
			lat, lon, altitude, heading                    sql.NullFloat64
			seenAt                                         time.Time
		)
		if err := rows.Scan(&callsign, &departure, &destination, &lat, &lon,
			&altitude, &heading, &positionTime, &seenAt); err != nil {
			return nil, err
		}
		if nearest != nil && absDuration(seenAt.Sub(at)) >= absDuration(nearest.UpdatedAt.Sub(at)) {
			continue
		}
		nearest = &model.SurveillancePosition{
			CallSign:     callsign.String,
			Departure:    departure.String,
			Destination:  destination.String,
			Lat:          lat.Float64,
			Lon:          lon.Float64,
			Altitude:     altitude.Float64,
			Heading:      heading.Float64,
			PositionTime: positionTime.String,
			UpdatedAt:    seenAt,
		}
	}
	return nearest, rows.Err()
}

func (s *surveillanceHistoryStore) retentionWindow() time.Duration {
	return s.retention
}

func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}
