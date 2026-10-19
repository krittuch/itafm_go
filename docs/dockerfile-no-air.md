# Dev and prod images without air

## Why

`Dockerfile.dev` and `Dockerfile.prod` were based on `cosmtrek/air` and ran `air`. Hot reload was never needed on dev or prod. It also did nothing there: the compose files mount the repo at `/usr/src/app/`, while the image works in `/app`, and the Swarm stack has no mount at all. So air rebuilt the baked-in source once at start-up into `./tmp/main` and ran that. The `go build -o main` in the image was never used.

## What changed

`Dockerfile.dev` and `Dockerfile.prod` (identical) are now a two-stage build:

1. `golang:1.21-alpine` (matches `go 1.21.1` in `go.mod`) downloads modules, then builds a static binary with `CGO_ENABLED=0`. No dependency needs cgo.
2. `alpine:3.20` gets the binary and `data/`, and runs `./main` from `/app`.

`data/` is copied because `app/mqtt.go` opens `data/flight_airlinecode.csv` relative to the working directory. Nothing else is read from disk at runtime, and the app uses no TLS or `time.LoadLocation`, so the runtime image needs no `ca-certificates` or `tzdata`.

`go.sum` is now copied with `go.mod`. Before, only `go.mod` was copied, so `go mod verify` ran without checksums.

Not changed:
- `Dockerfile.local` and `.air.toml` still use air. Only dev and prod were in scope.
- The unused `.:/usr/src/app/` mounts in `development.yml` and `production.yml`.
- `.github/workflows/dev.yml` and `stack.development.yml`: they reference `Dockerfile.dev` by name only.

## Verification

- `docker build -f Dockerfile.dev` succeeded. The image is 21.7 MB.
- Running the image without the env files, the gateway started from `/app`: it loaded 217 rows from `data/flight_airlinecode.csv` and opened the monitor on 18081.
- `Dockerfile.prod` is byte-identical to `Dockerfile.dev`, so it was not built separately.
- Not tested: a full deploy on the dev server with real Kafka, MQTT and Postgres.
