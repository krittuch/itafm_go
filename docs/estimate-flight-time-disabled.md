# estimate_flight_time writes disabled

## What was attempted
User reported "something wrong" with `estimate_flight_time` and asked to disable every code path that writes it, without further diagnosis for now.

## What changed
Commented out all four writers of `estimate_flight_time`, keeping surrounding logic (change-log diffing, parsing) intact so the code still compiles:

- [app/flightMovementHandler.go](../app/flightMovementHandler.go) `onFPLReceive` — call to `UpdateDepartureEstimateFlightBySchedule` (EET parsed from FPL `ITEM18`).
- [app/flightMovementHandler.go](../app/flightMovementHandler.go) `onCHGReceive` — call to `UpdateEstimateFlightByDestination` (destination/HHMM token `-13/AAAA1234`).
- [app/flightMovementHandler.go](../app/flightMovementHandler.go) `onCHGReceive` — call to `UpdateArrivalEstimateFlightByRoute` (EET parsed from CHG item).
- [repository/flight.go](../repository/flight.go) `UpdateFlight` — the `EstimateFlightTime` branch of the generic `PatchFlight` update (this path had no caller anyway).

Variables that became unused after commenting out their only use site (`hhmm`, `arrivalEstimate`) are kept alive with `_ = x` so the surrounding parsing/logging logic didn't need to change.

## Still blocked
Root cause of "something wrong" was not investigated — this is a stopgap to stop writes while that's looked into separately. The three still-active handlers (`onFPLReceive`, `onCHGReceive`) now report `updated = true` / trigger unrelated change-log entries as before, but no longer persist `estimate_flight_time` to the DB.

## How it was verified
`go build ./...` and `go vet ./...` both pass with no output/errors.
