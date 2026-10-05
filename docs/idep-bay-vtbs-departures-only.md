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
