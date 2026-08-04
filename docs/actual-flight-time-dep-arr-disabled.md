# DEP/ARR actual_flight_time writes disabled

## What was attempted
Temporarily prevent departure and arrival movement messages from updating `actual_flight_time` while leaving other update paths unchanged.

## What changed
The calls to `UpdateDepartureFlight` and `UpdateArrivalFlight` in `onCMDReceive` were commented out. Message parsing, validation, flight lookups, and existing change-log comparisons remain in place.

The generic `PatchFlight` writer for `actual_flight_time` remains enabled.

## Still blocked
The reason these movement updates need to be paused has not been investigated. This is a temporary stopgap.

## How it was verified
`go build ./...` and `go vet ./...` pass with no output/errors. The test suite passes with `go test -ldflags=-linkmode=external ./...`.

Plain `go test ./...` cannot execute the generated test binaries on this machine because Go 1.21.6's internal linker omits the `LC_UUID` load command required by macOS 26; external linking avoids that environment/toolchain incompatibility.
