# Production runs surveillance and IDEP TOBT only

## What was asked

On 2026-10-27 the user asked that production use only two things: surveillance, and IDEP limited to updating TOBT. They chose an env switch for flight movement over a production-only code change, and asked that the raw-message archive stop in production too. They then asked that IDEP update only TOBT in every environment, not only in production.

## What changed

- `app/idepHandler.go`: the `UpdateBay` call in `onIDEPReceive` is commented out, so IDEP writes only TOBT on dev, local and production. As with the earlier estimate and actual flight time stopgaps, the code was commented out rather than deleted: `idepBayUpdate`, the VTBS stand allowlist (`app/vtbs_stands.go`), its tests, and `UpdateBay` stay, so the bay write can be restored by uncommenting the block.
- `app/mqtt.go`: `GATEWAY_FLIGHT_MOVEMENT_ENABLED` (default `true`). When `false`, the flight movement consumer (FPL/DEP/ARR/CNL/CHG/DLA) is not started, and one line is logged at start-up.
- `production.yml`: the gateway sets `GATEWAY_FLIGHT_MOVEMENT_ENABLED: "false"`. The `AODS_ARCHIVE_DB_*` variables and the `depends_on: aods_archive_db` entry were removed from the gateway, so the existing "missing settings" path in `app/app.go` turns the archive off. No IDEP or FLMO messages are archived, and the monitor's `/archive` page has no DB.
- `compose_services_test.go`: `TestProductionGatewayRunsSurveillanceAndIDEPOnly` checks that the production gateway turns flight movement off and has no archive DB settings.
- `README.md`: the switch is listed under gateway env.

## Not changed

- Dev and local still consume flight movement and archive messages; only the IDEP bay write is off everywhere.
- The `aods_archive_db` service and its volume stay in `production.yml`. The gateway no longer connects to it.
- The java broker still forwards the FLMO queue to the `itafm.flight_movement` Kafka topic. Nothing consumes it in production, so messages only take up Redpanda disk until retention deletes them.
- The monitor still lists a `flight` route; in production its counts stay at 0.

## Verification

- `go vet ./...` and `CGO_ENABLED=0 go test ./...` pass.
- Before the bay write was commented out, the binary was started with the production settings and an unreachable Kafka broker. It logged `AODS archive DB disabled` and `flight movement consumer disabled`, and started only the surveillance and IDEP consumers.
- Not tested: a deploy with real Kafka and Postgres. Each server must rebuild the gateway image for this to take effect, because the image bakes in the source (for production: `docker compose -f production.yml up -d --build gateway`).
