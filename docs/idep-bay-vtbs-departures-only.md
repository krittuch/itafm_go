# IDEP bay limited to known VTBS departure stands

## What broke

On 2026-10-04, the bay of arrival flight TG 204 at VTBS flipped between `C3` and `9` (later `3`). The change log showed `9 -> C3` eight times, each written by `tgw_go`. The writes back to `9` left no log entry. AGOS reads the bay live when it checks Catering `Leave AC`. During one flip it decided a parked truck had left the aircraft and stamped `Leave AC` while the truck was still parked at the aircraft.

## Root cause

`onIDEPReceive` wrote `DepartureParkingStand` through `UpdateBay` for every flight with that flight number scheduled today or tomorrow. The update did not check:

- the airport the stand belonged to (`Departure`)
- the flight type
- the flight date (`EOBT`)

It also wrote no change log. An IDEP for TG 204 departing another airport therefore overwrote the bay of TG 204 arriving at VTBS.

## Change

- `app/idepHandler.go`: the new `idepBayUpdate` returns a bay only when `Departure == "VTBS"`, `EOBT` is present, and the stand is a known VTBS stand. The TOBT update is unchanged.
- `app/vtbs_stands.go`: allowlist of the 186 valid VTBS stands supplied by the user on 2026-10-05. The match ignores case and surrounding spaces; anything else (such as `9` or `3`) is dropped. Update this list when stands are added or renamed.
- `repository/flight.go`: `UpdateBay` now updates only `type = 'DEP'` flights whose STD is within 12 hours of EOBT, and only when the bay actually changed. It records each change in `flight_flightchangelog` in the same statement. An EOBT without a timezone is treated as UTC (`+00`), the same as TOBT.
- `controller/flight.go`: parameter renamed to `eobt`.
- `app/idep_handler_test.go`: tests for the gating and the allowlist.

## Verification

- `CGO_ENABLED=0 go build ./...` and `go test ./...` pass. cgo was disabled because the host Xcode license was not accepted; the tests don't need cgo.
- The same `UpdateBay` statement was run against a disposable `postgres:16-alpine` with the session timezone set to `Asia/Bangkok`. Only the matching DEP row changed. A repeated call logged nothing. The ARR flight and the next day's DEP flight were unchanged.

## Not covered

The service has not been deployed. Arrival bays still come only from the Thai Airways feed (`tgw_go`). IDEP `ArrivalParkingStand` is still ignored.

## Follow-up 2026-10-19: flip still seen on VZ 103 (ARR), dev server

On 2026-10-19, on the dev server (DB `192.87.110.14`), the ARR flight VZ 103 showed the same pattern: `6 -> A2` was logged every 5 minutes, and nothing logged the writes back to `6`.

The current code can't produce this. `6` fails all three gates: it isn't in `vtbsStandList`, the IDEP for an arrival isn't `Departure == "VTBS"`, and `UpdateBay` only touches `type = 'DEP'`. It would also be logged. No other code in this repo writes `bay`; the `UpdateFlight` repository method has no caller. `go test ./app/` passes.

The likely cause is that the dev gateway is still running the pre-fix build. `Dockerfile.dev` (same as `Dockerfile.prod`) copies the source into `/app` at image build time, and `air` builds from `/app`. `development.yml` mounts the repo at `/usr/src/app/`, so `git pull` on the server changes nothing in the running container. The fix is on `origin/dev` and `origin/production` but not on `origin/main`, so the server checkout must be on `dev` before rebuilding with `docker compose -f development.yml up -d --build gateway`.

Verified: `docker exec itafm_aods_gw_dev grep -c idepBayUpdate /app/app/idepHandler.go` returned `0` on the dev server, so the container was running the pre-fix code.
