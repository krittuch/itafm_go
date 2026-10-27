# CI/CD for the dev branch (Docker Swarm stack)

## Why

On 2026-10-19 the dev gateway was still running code from before the IDEP bay fix, two weeks after it was merged to `dev`. `Dockerfile.dev` bakes the source into the image, so a `git pull` on the server does nothing until the image is rebuilt (see `docs/idep-bay-vtbs-departures-only.md`).

The deploy follows AGOS_DJ's dev deploy (`AGOS_DJ/.github/workflows/ci.yml`, `docs/operations/swarm_phase2_note.md`): images are built on the server, tagged by commit, and run as a Docker Swarm stack with `--resolve-image never`, so no registry is needed.

## When jobs run

`.github/workflows/dev.yml`:

| Event | test | deploy |
|---|---|---|
| PR into `dev` | runs | no |
| Direct push to `dev` | runs | after tests pass |
| PR merge into `dev` whose PR run passed | skipped | runs |
| PR merge whose PR run didn't pass, multi-commit push, rebase | runs | after tests pass |
| Manual `workflow_dispatch` on `dev` | runs | after tests pass |

`classify` is AGOS_DJ's rule. Tests are skipped only when all of these hold:
- the pushed commit's first parent is the previous tip of `dev`
- the pushed commit is the merge commit of a merged PR into `dev`
- that PR's latest `dev.yml` run succeeded

If the API lookup fails, the tests run.

`test` runs on `ubuntu-latest`: `go vet ./...`, `go test ./...`, and the `java-broker` Maven package (it has no tests).

The deploy job never runs for `pull_request` events. PRs from forks therefore can't run code on the dev server, which matters because the repo is public.

## The stack

`stack.development.yml` defines the Swarm stack `itafm_go`, with the same 4 services as `development.yml`:

| Service | Image | Network / ports | Data |
|---|---|---|---|
| `itafm_go_aods_archive_db` | `postgres:16-alpine` | 5435 published in host mode | external volume `itafm_go_aods_archive_db_dev` |
| `itafm_go_itafm_kafka` | `redpandadata/redpanda:v24.2.9` | 9099 published in host mode, advertises `localhost:9099` | external volume `itafm_go_kafka_data_dev` |
| `itafm_go_java_broker` | `itafm_go-java_broker:<sha>` | host network (`hostnet`, external network `host`) | none |
| `itafm_go_gateway` | `itafm_go-gateway:<sha>` | host network | none |

Notes:
- The gateway and broker stay on the host network, so `localhost:9099`, `localhost:5435`, and the monitors on 18081 and 18080 work as before. agos_ops' backup stack (host port 5435) and its Kuma monitor (port 18081) don't refer to container names, so the new Swarm names don't affect them.
- The volumes are the ones the compose project `itafm_go` created. The stack declares them `external`, so the data carries over and `docker stack rm` never deletes them.
- `env_file` reads `./.env`, which the deploy job writes from the `development` GitHub environment's secrets just before it deploys. Values are written unquoted, because `stack deploy` doesn't strip quotes. The `.envs/` files are no longer used or tracked.
- The unused `.:/usr/src/app/` mount from `development.yml` is not in the stack.
- `development.yml` is left as it is. Don't `up` it on the dev server any more, or the gateway and broker would run twice.

## Deploy steps (self-hosted runner)

1. Check that Swarm is active and both volumes exist.
2. `docker build` both images, tagged `itafm_go-gateway:<sha>` and `itafm_go-java_broker:<sha>`.
3. Write `.env` (mode 600) with one line per key in `.env.example`. Each value comes from the `development` environment secret of the same name; an unset secret falls back to the value in `.env.example`. The job fails if a key that is empty there (other than `GATEWAY_*`) has no secret.
4. While any container of the compose project `itafm_go` remains, run `docker compose -p itafm_go -f development.yml down --remove-orphans` without `-v`. This only happens on the first deploy, and that is the one outage. On later runs it is skipped, because the stack's own network is also called `itafm_go_default`.
5. `docker stack deploy --resolve-image never -c stack.development.yml itafm_go`.
6. Wait up to 5 minutes until every service is `1/1` and its update state is `completed`. It fails at once on `paused` or `rollback_*`.
7. Fail unless the gateway and broker services use this commit's tag.
8. Wait up to 3 minutes each for `:18081/health` and `:18080/health`.
9. Untag older images: keep this commit's tag plus the 2 newest others, by image creation time.
10. Delete `.env`, even when an earlier step failed.

The deploy job runs in the `development` GitHub environment, which only the `dev` branch may deploy to.

## One-time setup on the dev server

1. Register a self-hosted runner for `krittuch/itafm_go`: GitHub, then Settings > Actions > Runners > New self-hosted runner (Linux). Runners belong to a single repo on a personal account, so AGOS_DJ's runner can't be reused. The default labels `self-hosted` and `Linux` are enough.
2. Run it as a user that can use Docker and has Compose v2. Install it as a service: `sudo ./svc.sh install && sudo ./svc.sh start`.
3. Make the `test` check required for `dev`.
4. Add one secret per `.env.example` key to the `development` environment. From a filled-in dotenv file: `gh secret set -f <file> --env development --repo krittuch/itafm_go`.

## Rollback

`docker stack rm itafm_go` keeps the volumes. Then `docker compose -p itafm_go -f development.yml up -d --build` brings back the compose stack. It needs a filled-in `.env` in the checkout first.

## Conflicts with agos_ops

agos_ops' `AGENTS.md` and `docs/production_plan.md` (Phase 4) record that itafm_go stays plain compose. The reason given is that a host-network service can't be expressed in a stack. AGOS_DJ's `pgbackups` shows it can, using the external network `host`. The operator chose Swarm on 2026-10-19, so agos_ops' decision and its `docs/service_inventory.md` rows need updating in that repo.

## Verification

- `actionlint` 1.7.1 passes on the workflow.
- The stack was deployed with `docker stack deploy` in a throwaway single-node Swarm (`docker:27-dind`), using stub images for the gateway and broker:
  - all 4 services reached `1/1`
  - the gateway ran on the host network with its env files applied
  - 5435 and 9099 were open on the host
- The workflow's own wait, image check, compose-down and prune scripts were extracted and run in that Swarm after a redeploy with a new SHA:
  - the wait and image checks passed
  - the image check failed as expected for a wrong SHA
  - the compose step was skipped
  - after a fix, prune kept the current tag
- The first prune version removed the running commit's tag, because identical rebuilds share a creation time. That was fixed by excluding the current tag by name.
- Not tested:
  - real gateway and broker images
  - the move from compose to stack on the real server
  - the GitHub-side classify run
