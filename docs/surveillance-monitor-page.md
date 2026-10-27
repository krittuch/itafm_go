# Surveillance monitor page

## What was asked

On 2026-10-27 the user could not find a surveillance monitor or any way to filter surveillance messages. The gateway monitor had only one SURVEILLANCE counter row on `/routes`, and `/archive` covers only FLMO and IDEP. No branch or commit had ever had a surveillance page. The user chose a live in-memory view over querying `flight_aircraftlocation` or archiving raw surveillance, and asked that its memory use be limited.

## What changed

- `app/surveillance_monitor.go`: `surveillanceTrackStore` keeps the latest message per raw callsign, plus a counter for each outcome. Memory is bounded three ways:
  - at most `GATEWAY_SURVEILLANCE_MONITOR_MAX_TRACKS` tracks (default 2000); when full, the least recently seen track is dropped and `evicted` goes up
  - a track with no message for `GATEWAY_SURVEILLANCE_MONITOR_TTL` (default 15m) is dropped on the next record or read
  - each string field is cut to 32 bytes and cloned, so an oversized payload field cannot pin its full string. No raw payload is kept.

  At 2000 tracks this is well under 1 MB.
- `app/surveillanceHandler.go`: `onSurveillanceReceive` returns a `surveillanceReceiveResult` with an outcome (`queued`, `duplicate`, `not_vtbs`, `iata_failed`, `decode_error`, `rejected`), the raw and IATA callsigns, and the decoded data. The VTBS and IATA checks are unchanged.
- `app/surveillance_batcher.go`: `queue` returns `queued`, `duplicate` or `rejected`. `add` wraps it and still returns a bool for the existing tests.
- `app/mqtt.go`: the surveillance consumer records every result in the store. Route counters are unchanged: `queued` and `duplicate` count as processed, everything else as skipped.
- `app/surveillance_monitor_page.go`: `GET /surveillance` (and under the base path, e.g. `/aods-mon/surveillance`), behind the same basic auth. It serves HTML to browsers and JSON with `?format=json`. Filters: `callsign` (matches raw or IATA), `dep`, `dest`, `actype`, `ssr`, `acaddress`, `outcome`, `sort` (`last_seen`, `callsign`, `altitude`) and `limit` (default 200, max 2000). The outcome counters link to a filtered view.
- `app/gateway_monitor.go`, `app/gateway_monitor_pages.go`: route, nav link and an index card. The store is created only when the monitor is enabled. Otherwise it is nil and recording does nothing.
- `README.md`: endpoint, outcomes and both env vars.

## Not changed

- The page shows only what this gateway process has seen since it started. A restart empties it.
- The batcher's `lastFlushedSignature` map is still unbounded. It grows by one entry per distinct VTBS callsign written and is never pruned. That was not part of this change.

## Verification

- `go vet ./...` and `CGO_ENABLED=0 go test ./...` pass. New tests in `app/surveillance_monitor_test.go` cover the cap eviction, TTL expiry, field truncation, decode errors counted without a track, each filter and sort, each receive outcome, and the HTML and JSON responses.
- The page and index were rendered with sample tracks and checked in a browser at desktop and phone widths.
- Not tested: a running gateway with real Kafka traffic. The gateway image must be rebuilt for this to take effect.
