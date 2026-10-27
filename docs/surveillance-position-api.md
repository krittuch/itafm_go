# Surveillance position API

## What was asked

On 2026-10-27 the user asked for an API that returns one flight's current location, heading, departure, destination, last update and altitude, looked up by callsign or IATA code, with its own authentication. They chose the `flight_aircraftlocation` table as the source (over the in-memory monitor tracks) and an API key header for auth.

## What changed

- `model/surveillance.go`: `SurveillancePosition`.
- `repository/surveillance.go`, `controller/surveillance.go`: `GetSurveillancePosition` reads the newest row (`updated_at DESC NULLS LAST`) whose `callsign` is one of the given values. NULL columns scan to zero values. No row returns `nil`.
- `app/surveillance_api.go`:
  - `GET /api/surveillance/position?callsign=...` is served on the monitor server, with and without the monitor base path.
  - Clients call it through the edge nginx as `https://<EDGE_SERVER_NAME>/aods-monitor/api/surveillance/position?callsign=...`. agos_ops' `nginx/templates/default.conf.template` has `location /aods-monitor/ { proxy_pass http://aods_monitor/; }` (upstream port 18081), the same on its `development` and `production` branches. The trailing slash strips the prefix, so no nginx change was needed. Client headers such as `X-API-Key` and the query string pass through unchanged, and Django's `location /api/` does not match because the path starts with `/aods-monitor/`.
  - The callsign is upper-cased and must be 2-10 letters or digits. Rows are stored under the IATA callsign, so `THA661` is looked up as both `THA661` and `TG661`.
  - The DB query has a 5 s timeout.
  - DB errors are logged and answered with a generic `500`, so no DB detail reaches clients.
- Auth (`withSurveillanceAPIKey`) is separate from the monitor's basic auth:
  - The key goes only in the `X-API-Key` header, never in the URL, so it stays out of access logs.
  - `GATEWAY_SURVEILLANCE_API_KEYS` holds comma-separated keys, one per client.
  - Keys are compared in constant time.
  - No key configured means `503`. A missing or wrong key means `401`.
  - Start-up logs the number of keys, never the keys themselves.
- `app/gateway_monitor.go`, `app/mqtt.go`: route registration, and the monitor gets a reader over the main DB.
- `README.md`: endpoint, response, errors, the env var and where each environment's keys are set.

## Constraints and open points

- The API runs on the monitor HTTP server, so `GATEWAY_MONITOR_ENABLED=false` turns it off too.
- `/aods-monitor/` has no IP allow-list in nginx (unlike Portainer and Kuma), so the API is reachable from the internet and the API key is its only protection.
- Not confirmed: that the production gateway runs on the host the prod edge nginx forwards `/aods-monitor/` to.
- Keys are kept out of git. Like every other setting, they come from the untracked `.env` (see `docs/dotenv-and-github-environment.md`):
  - Dev: the `GATEWAY_SURVEILLANCE_API_KEYS` secret of the `development` GitHub environment, which the deploy job writes into `.env`.
  - Production: the server's `.env`.
  - With no key set, the API answers `503`. The deploy does not fail.
- Env values are visible to anyone on the server who can run `docker inspect` / `docker service inspect`.
- `flight_aircraftlocation` keeps one row per callsign and never deletes it. A flight number seen yesterday still returns yesterday's position. Clients should check `last_update`.
- The table's real column types are not in this repo. If `updated_at` is `timestamp` without a time zone, the value is in the DB session's zone (Asia/Bangkok on the servers), but `last_update` would label it UTC. With `timestamptz`, which is the Django default, it is correct.
- The project has no OpenAPI/Swagger setup, so the endpoint is documented in the README only.

## Verification

- `go vet ./...` and `CGO_ENABLED=0 go test ./...` pass. `app/surveillance_api_test.go` covers:
  - no keys, missing key, wrong key, and a second valid key
  - ICAO to IATA candidates
  - the full response, with `last_update` converted to UTC
  - `400` for bad callsigns, `500` without leaking the DB error, `503` without a reader, `405` for POST
  - env key parsing
- The query ran against a disposable `postgres:16-alpine` with two column-type variants: float/varchar/timestamptz, and numeric/timestamptz/timestamp. In both, ICAO input matched the IATA row, NULL columns returned zero values, and an unknown callsign returned no row.
- Not tested: the real dev or production table, a deployed gateway, or a call through nginx. The nginx route was read from agos_ops, not called; the server's `EDGE_SERVER_NAME` is not in this repo.
