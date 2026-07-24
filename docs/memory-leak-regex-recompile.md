# High RAM usage after prolonged runtime

## What was attempted
Investigated why the gateway process's RSS grows to ~5GB after running for a
while, by walking the Kafka consumption hot path (`app/mqtt.go` ->
`app/flightMovementHandler.go` / `app/idepHandler.go` / `app/surveillanceHandler.go`)
and the repository layer for classic Go leak patterns: unclosed `*sql.Rows`/
`*sql.Stmt`, unbounded caches, and goroutine leaks.

## What changed
Found that `onFPLReceive`, `onCMDReceive`, `onCNLReceive`, `onDLYReceive`,
`onCHGReceive` (`app/flightMovementHandler.go`) and `onIDEPReceive`
(`app/idepHandler.go`) called `regexp.MustCompile` / `regexp.MatchString`
inline on every invocation — i.e. on every single Kafka message on the flight
movement and IDEP topics (up to 16 call sites, several messages compiling 3-5
regexes each). Compiling a regex allocates a parse tree and bytecode program
each time; under sustained message throughput this produces continuous
allocation churn that keeps the Go heap's high-water mark elevated, since the
runtime doesn't aggressively return scavenged pages back to the OS while
allocation stays active.

Hoisted all 16 regexes to a single package-level `var (...)` block in
`app/flightMovementHandler.go` (compiled once at package init), and updated
all call sites in both files to reference the shared, precompiled
`regexp.Regexp` values instead of recompiling. Removed the now-unused
`regexp` import from `app/idepHandler.go`.

Also noted, but did not change (dead code, not on the live path): 
`repository/surveillance.go`'s `InsertOrUpdateSurveillance` leaks two
unclosed `*sql.Stmt` per call. It's unreferenced — the live surveillance path
uses `InsertOrUpdateSurveillanceBatch`, which closes its statements
correctly.

## What is still blocked
Nothing blocked. This fix removes the main source of allocation churn found
during the investigation. It has not been confirmed to fully resolve the 5GB
RSS in production yet — no memory profiling was available on this host to
capture a before/after heap profile (no `net/http/pprof` endpoint is wired
into the gateway monitor).

## How it was verified
- `go build ./...` — passes.
- `go vet ./...` — passes.
- `go test ./...` — fails with `dyld: missing LC_UUID load command`, but
  confirmed via `git stash` that this failure is pre-existing on `dev` and
  unrelated to this change (macOS/Xcode toolchain linking issue on this
  machine, not a code defect).

Recommended follow-up for production confirmation: add a `net/http/pprof`
mux under the existing gateway monitor server (`app/gateway_monitor.go`)
behind the same basic-auth gate, and capture `/debug/pprof/heap` before and
after deploying this change under real traffic.
