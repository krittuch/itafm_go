# Surveillance position history (last 30 minutes)

## What was asked

On 2026-10-30 the user asked for historic surveillance search without bloating the database, since positions arrive every 5 s. Their answers to follow-up questions:

- Use: a flight's location at a specific time.
- Lookback: the last 30 minutes only.
- Lookup: one flight by callsign.
- Between points: return the nearest recorded point with its real timestamp.
- Storage: `aods_archive_db`.

## Design

- **Size.** At this retention the volume is small, so nothing is compressed.
- **Expiry.** The bloat risk was `DELETE` churn and vacuum. Expiry drops whole time partitions instead, so no row is ever deleted.
- **No WAL.** Partitions are `UNLOGGED`. A Postgres crash empties them, which loses at most the retention window.

## What changed

- `app/surveillance_history.go`: `surveillanceHistoryStore`.
  - Start-up:
    - creates `surveillance_track_point` (partitioned by `seen_at`) and its `(callsign, seen_at)` index with `IF NOT EXISTS`
    - the gateway creates the table itself because `compose/aods-archive/init.sql` only runs on an empty volume
    - a setup error disables history and is logged; the gateway keeps running
  - `record`: buffers a point stamped with the gateway's receive time (`seen_at`) and the message's `datetime` (`position_time`). The buffer holds at most 20,000 points; when full, the oldest is dropped.
  - Every 5 s, `flush` writes the buffer in one `INSERT ... SELECT unnest(...)`. A failed batch is logged and dropped. History is best effort and never blocks the `flight_aircraftlocation` writes.
  - Every minute, `maintainPartitions`:
    - creates `UNLOGGED` partitions for the current and next two 10-minute slots, named `surveillance_track_point_pYYYYMMDD_HHMM` in UTC
    - drops every partition that ends at or before now minus the retention, so up to 40 minutes are kept
  - `GetSurveillancePositionAt`:
    - one query, a `UNION ALL` of the last point at or before `at` (no older than the retention) and the first point after it
    - Go returns the closer of the two
- `app/mqtt.go`:
  - `startSurveillanceHistory` runs only when the archive DB is connected and `GATEWAY_SURVEILLANCE_HISTORY_ENABLED` is not `false`. Retention comes from `GATEWAY_SURVEILLANCE_HISTORY_RETENTION` (default `30m`).
  - The surveillance consumer records every `queued` message. Exact repeats (`duplicate`) are not stored. Every 5 s message that differs gets its own point, independent of `SURVEILLANCE_DB_BATCH_INTERVAL` (default 10 s), which only keeps the latest point per callsign.
  - New `GATEWAY_AODS_MESSAGE_ARCHIVE_ENABLED` (default `true`). When `false`:
    - FLMO/IDEP are not archived
    - the monitor's `/archive` page has no DB
    - the archive DB stays connected for history
- `app/surveillance_api.go`:
  - `GET /api/surveillance/position` takes an optional `at` (RFC 3339), served from history.
  - `400`: `at` is not RFC 3339, is older than the retention, or is more than 1 minute in the future.
  - `503`: no history store.
  - The response shape is unchanged. `last_update` is the point's `seen_at`.
- `production.yml`:
  - The gateway again sets `AODS_ARCHIVE_DB_*` (`localhost:5436`, the existing `aods_archive_db` service) and depends on it.
  - It sets `GATEWAY_AODS_MESSAGE_ARCHIVE_ENABLED: "false"`, so production still archives no FLMO/IDEP messages.
- `compose_services_test.go`: the production test now checks the archive switch is off and the archive DB port matches the service, instead of checking that no archive DB is configured.
- `README.md`: the `at` parameter and the three env vars.

## Measured size

200 callsigns × 360 points (30 min at 5 s) = 72,000 rows in two partitions on `postgres:16-alpine`:

- heap: 8.4 MB
- index: 2.5 MB
- about 156 B per row in total

The design draft estimated 4 MB. Text columns (`position_time`, dep, dest) and the row header make it larger. It is still bounded by the retention. `EXPLAIN` shows a bitmap index scan on `(callsign, seen_at)` in each partition.

## Constraints and open points

- `seen_at` is the gateway's receive time, not the message's `datetime`. The `datetime` format has not been checked against real traffic. It is stored as text in `position_time` and returned as before.
- The nearest point can be far from `at`, for example after the aircraft left coverage. The API does not cap the distance; clients should compare `last_update` to `at`.
- A gateway restart loses at most one 5 s buffer. A Postgres crash empties the `UNLOGGED` partitions.
- Partitions are created only by the running gateway. If maintenance fails for over 20 minutes, inserts fail (logged) until it recovers.
- History runs on dev, local and the dev swarm stack too, because they already set `AODS_ARCHIVE_DB_*`.

## Verification

- `go vet ./...` and `CGO_ENABLED=0 go test ./...` pass.
- `app/surveillance_history_test.go` covers:
  - the `at` API path: ICAO to IATA candidates, the passed time, the response
  - `at` errors: bad formats, too old, future, no history (`503`), no point (`404`), DB error without leaking detail (`500`)
  - buffer cap and nil store
  - partition name round trip
- `TestSurveillanceHistoryStoreAgainstPostgres` runs only with `SURVEILLANCE_HISTORY_TEST_DSN` set. It was run against a disposable `postgres:16-alpine` and passed. It covers:
  - setup run twice
  - 15 minutes of 5 s points with a 1-minute gap
  - partitions are `UNLOGGED`
  - nearest-point picks on both sides, across the gap, and before the first point
  - unknown callsign
  - dropping expired partitions
- Not tested: a deployed gateway with real Kafka traffic, or the production archive DB. The gateway image must be rebuilt for production (`docker compose -f production.yml up -d --build gateway`).
