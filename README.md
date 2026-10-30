# iTAFM Gateway Stack

This repository runs a message pipeline:

1. `java_broker`: ActiveMQ destinations -> Kafka topics
2. `gateway`: Kafka topics -> PostgreSQL updates
3. `kafka`: Redpanda Kafka broker

## Compose services

- `kafka` in `/Users/krittuch/Projects/itafm_go/development.yml`
- `java_broker` in `/Users/krittuch/Projects/itafm_go/development.yml`
- `gateway` in `/Users/krittuch/Projects/itafm_go/development.yml`

## Monitor endpoints

Both runtime services expose HTTP monitor endpoints so you can debug flow without reading logs.

### Java broker monitor

- Default URL: `http://<host>:18080`
- Endpoints:
  - `GET /health`
  - `GET /routes`

Example:

```bash
curl -s http://127.0.0.1:18080/health | jq
curl -s http://127.0.0.1:18080/routes | jq
```

Key route fields:

- `online`: route is connected/subscribed
- `received_count`: messages read from ActiveMQ
- `forwarded_count`: messages written to Kafka
- `last_error`: most recent route error

### Gateway monitor

- Default URL: `http://<host>:18081`
- Endpoints:
  - `GET /health`
  - `GET /routes`
  - `GET /surveillance`: latest track per callsign, held in memory, with filters (`callsign`, `dep`, `dest`, `actype`, `ssr`, `acaddress`, `outcome`, `sort`, `limit`) and per-outcome counters

Example:

```bash
curl -s http://127.0.0.1:18081/health | jq
curl -s http://127.0.0.1:18081/routes | jq
curl -s -u aodsMon:<password> 'http://127.0.0.1:18081/surveillance?format=json&outcome=not_vtbs' | jq
```

Surveillance outcomes: `queued` (sent to the DB batch), `duplicate` (unchanged since the last write), `not_vtbs`, `iata_failed`, `decode_error`, `rejected`.

### Surveillance position API

`GET https://<EDGE_SERVER_NAME>/aods-monitor/api/surveillance/position?callsign=<callsign>` returns the stored position of one flight from `flight_aircraftlocation`. `callsign` can be the ICAO callsign (`THA661`) or the IATA code (`TG661`).

The edge nginx managed in agos_ops (`nginx/templates/default.conf.template`) forwards `/aods-monitor/` to the gateway monitor on port 18081 and strips the prefix, so the gateway sees `/api/surveillance/position`. That route has no IP allow-list; the API key is the only gate. On the server itself, `http://127.0.0.1:18081/api/surveillance/position` reaches the same handler.

It does not use the monitor login. Clients send an API key in the `X-API-Key` header; keys come from `GATEWAY_SURVEILLANCE_API_KEYS`. With no key configured the API answers `503`.

Where the keys are set:

- Dev: the `GATEWAY_SURVEILLANCE_API_KEYS` secret of the `development` GitHub environment. The deploy job writes it into `.env`; a new or changed key takes effect on the next deploy.
- Production: `GATEWAY_SURVEILLANCE_API_KEYS=<key1>,<key2>` in the server's `.env`. `docker compose -f production.yml up -d gateway` recreates the gateway when the value changed.

Generate a key with `openssl rand -hex 32`.

```bash
curl -s -H 'X-API-Key: <key>' 'https://<EDGE_SERVER_NAME>/aods-monitor/api/surveillance/position?callsign=TG661' | jq
```

```json
{
  "callsign": "TG661",
  "departure": "VTBS",
  "destination": "RJTT",
  "latitude": 13.69,
  "longitude": 100.75,
  "altitude": 12000,
  "heading": 45,
  "position_time": "2026-10-27T10:00:00Z",
  "last_update": "2026-10-27T10:00:05Z"
}
```

`position_time` is the time in the surveillance message; `last_update` is when the gateway last wrote the row (`updated_at`, UTC).

Add `at=<RFC 3339 time>` to get the position at a past time within the last 30 minutes (`GATEWAY_SURVEILLANCE_HISTORY_RETENTION`). The answer is the stored point closest to `at`, and `last_update` is when the gateway received that point, so clients can see how far it is from `at`. Exact repeats of the previous message are not stored; any other gap means no message arrived. `at` older than the retention, or more than a minute in the future, is a `400`; without surveillance history (no archive DB) it is a `503`.

```bash
curl -s -H 'X-API-Key: <key>' 'https://<EDGE_SERVER_NAME>/aods-monitor/api/surveillance/position?callsign=TG661&at=2026-10-27T09:45:00Z' | jq
```

The history is in `surveillance_track_point` in the AODS archive DB: one `UNLOGGED` partition per 10 minutes, created by the gateway, with partitions older than the retention dropped every minute. A full 30 minutes for 200 aircraft is about 11 MB. Errors are `{"error": "..."}` with `400` (bad callsign), `401` (missing or wrong key), `404` (no row), `500` (DB error) or `503` (API disabled).

Key route fields:

- `received_count`: messages consumed from Kafka
- `processed_count`: handler path processed the message
- `skipped_count`: handler received message but skipped processing
- `decode_error_count`: payload/command decoding errors
- `consumer_error_count`: Kafka consumer loop errors
- `last_error`: most recent consumer/decode error

## Settings

The gateway and java broker read their settings from an untracked `.env` in the repo root. [`.env.example`](.env.example) lists every key.

- Production and local: copy `.env.example` to `.env` and fill it in. `docker compose` fails without it.
- Dev server: the deploy job writes `.env` from the secrets and variables of the `development` GitHub environment, as grouped in `.env.example` (see [docs/dev-ci-cd.md](docs/dev-ci-cd.md)).

## Monitor configuration

### Java broker env

- `MONITOR_ENABLED` (default: `true`)
- `MONITOR_HOST` (default: `0.0.0.0`)
- `MONITOR_PORT` (default: `18080`)

### Gateway env

- `GATEWAY_MONITOR_ENABLED` (default: `true`)
- `GATEWAY_MONITOR_HOST` (default: `0.0.0.0`)
- `GATEWAY_MONITOR_PORT` (default: `18081`)
- `SURVEILLANCE_DB_BATCH_INTERVAL` (default: `10s`).
- `SURVEILLANCE_DB_BATCH_MAX_SIZE` (default: `500`).
- `GATEWAY_FLIGHT_MOVEMENT_ENABLED` (default: `true`): `false` stops consuming the flight movement topic.
- `GATEWAY_SURVEILLANCE_MONITOR_MAX_TRACKS` (default: `2000`): most tracks the surveillance page keeps; the least recently seen is dropped first.
- `GATEWAY_SURVEILLANCE_MONITOR_TTL` (default: `15m`): a track with no message for this long is dropped.
- `GATEWAY_SURVEILLANCE_API_KEYS` (default: empty, API off): comma-separated keys accepted in `X-API-Key` by the surveillance position API. Give each client its own key.
- `GATEWAY_SURVEILLANCE_HISTORY_ENABLED` (default: `true`): keeps surveillance points in the AODS archive DB for `at=` lookups. Needs `AODS_ARCHIVE_DB_*`.
- `GATEWAY_SURVEILLANCE_HISTORY_RETENTION` (default: `30m`): how far back `at=` can ask.
- `GATEWAY_AODS_MESSAGE_ARCHIVE_ENABLED` (default: `true`): `false` stops archiving FLMO/IDEP messages while the archive DB stays connected for surveillance history. Production sets it to `false`.

## Quick debug workflow

1. Confirm Java broker route has `received_count > 0` and `forwarded_count > 0`.
2. Confirm gateway matching route has `received_count > 0`.
3. If gateway `received_count > 0` but `processed_count = 0`, inspect skip conditions (for example callsign conversion and route filters).
4. Use `last_error` from both monitors before checking container logs.
