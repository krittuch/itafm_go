# Settings from .env and the development GitHub environment

## What was asked

On 2026-10-27, while wiring the surveillance position API key, the user asked for:

- dev: take settings from the `development` GitHub environment's secrets
- production: read `.env`
- stop using `.envs/` altogether
- untrack `.env` and `.envs/`, and ignore both

## Why

`.envs/{.development,.local,.production}/*` and the repo-root `.env` were tracked in git, in a public repo. They held DB and ActiveMQ credentials, so any new secret put there would have been committed.

## What changed

- `.env.example` (new, tracked) lists every key the gateway and java broker read, with non-secret defaults. These keys were left out because no code reads them: `ITAFM_MQTT_*`, `ITAFM_SURV_TOPIC`, `ITAFM_FLTH_TOPIC` (only declared in `app/config.go`), `MQTT_FLIGHT_MOVEMENT_TOPIC`, `MQTT_IDEP_TOPIC`, `DISABLE_ITAFM_MQTT`, `DB_DATABASE`, `POSTGRES_PASSWORD`.
- `production.yml`, `development.yml`, `local.yml`, `stack.development.yml`: `gateway` and `java_broker` use `env_file: ./.env` in place of the `.envs/` files. Inline `environment:` values (flight movement off in production, the dev archive DB) still override it. Compose fails when `.env` is missing.
- `.github/workflows/dev.yml`, deploy job:
  - It runs in the `development` environment, which allows only the `dev` branch.
  - A new step writes `.env` (umask 077) with one line per `.env.example` key, from the environment secret of the same name. An unset secret falls back to the value in `.env.example`.
  - The step fails if a key that is empty in `.env.example` (other than `GATEWAY_*`) has no secret. It also fails on a value containing a newline.
  - A last `if: always()` step deletes `.env`.
- `git rm --cached` on `.env` and `.envs/`. `.gitignore` now ignores `.env` and `.envs/` (in place of the 9 single-file entries), and `.dockerignore` ignores both.
- `compose_services_test.go`:
  - every compose/stack file reads `./.env` for both services and never `.envs`
  - `.env.example` keeps `KAFKA_BROKERS=localhost:9099`
  - the deploy job uses the environment and maps a secret for every `.env.example` key
  - both ignore files cover `.env` and `.envs`
- `README.md`, `docs/dev-ci-cd.md`: the new source of settings, the deploy step, and the one-time `gh secret set -f` setup.

## Things to know

- `git pull` on a server deletes its `.env` and `.envs/` files, because the commit removes them from git. Build the new `.env` before pulling.
- The old values are still in git history, and the repo is public, so the DB and ActiveMQ passwords in them should be rotated.
- The production gateway and java broker get every key in `.env`, so the broker now also sees the DB settings. Both run on the same host.
- `docker stack deploy` takes `env_file` values literally (no quote stripping), so the job writes values unquoted.
- Env values are visible to anyone on a server who can run `docker inspect` / `docker service inspect`.

## Verification

- `go vet ./...` and `CGO_ENABLED=0 go test ./...` pass. The compose tests read only tracked files, so CI does not need a `.env`.
- The workflow YAML parses, and the deploy steps are in order: build, write `.env`, compose down, stack deploy, …, remove `.env`.
- The write step's script ran locally under bash:
  - With every required key set, it wrote them unquoted, including a value with `=` and `$`. Kafka and batch keys fell back to `.env.example`, the file was mode 600, and it exited 0.
  - With required keys missing, it listed them and exited 1.
- `docker compose -f <file> config` rendered `production.yml`, `development.yml`, `local.yml` and `stack.development.yml` with a `.env` present. `production.yml` failed without one.
- Not tested: a real dev deploy through the `development` environment (its secrets are not set yet), or a production `up` with the new `.env`.
