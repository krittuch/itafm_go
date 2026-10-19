# CI/CD for the dev branch

## Why

On 2026-10-19 the dev gateway was still running code from before the IDEP bay fix, two weeks after it was merged to `dev`. `Dockerfile.dev` bakes the source into the image, so a `git pull` on the server does nothing until the image is rebuilt (see `docs/idep-bay-vtbs-departures-only.md`).

## What runs

`.github/workflows/dev.yml`:

| Event | classify | test | deploy |
|---|---|---|---|
| PR into `dev` | skipped | runs | skipped |
| Direct push to `dev` | `pr_merge=false` | runs | runs if tests pass |
| Push that is a PR merge into `dev` | `pr_merge=true` | skipped | runs |
| Manual `workflow_dispatch` on `dev` | skipped | runs | runs if tests pass |

- **classify** asks the GitHub API for merged PRs into `dev` whose `merge_commit_sha` equals the pushed commit. Merge commits, squash merges and rebase merges all match. If the API call fails, the push is treated as a direct push, so tests run before deploy.
- **test** runs on `ubuntu-latest`: `go vet ./...`, `go test ./...`, and a Maven package of `java-broker` (it has no tests).
- **deploy** runs on a self-hosted runner labelled `itafm-dev` on the dev server:
  1. Checks out the commit.
  2. Uses the compose project of the running `itafm_aods_gw_dev` container (default `itafm_go`), so the named volumes are kept.
  3. Runs `docker compose -f development.yml up -d --build` for the whole stack.
  4. Fails if `itafm_aods_gw_dev` or `itafm_amq_kafka_broker_dev` isn't running the image that was just built.
  5. Waits up to 3 minutes per service for `:18081/health` (gateway) and `:18080/health` (java broker).

The deploy job never runs for `pull_request` events. PRs from forks therefore can't run code on the dev server, which matters because the repo is public.

## One-time setup on the dev server

1. Go to GitHub, then Settings > Actions > Runners > New self-hosted runner (Linux). Follow the steps it shows, and add the label `itafm-dev` when `config.sh` asks for labels.
2. Run the runner as a user that can use Docker (in the `docker` group) and has Docker Compose v2 (`docker compose`).
3. Install it as a service: `sudo ./svc.sh install && sudo ./svc.sh start`.
4. From then on, deploy only through the workflow. Running `docker compose up --build` from the old checkout on the server would bring back whatever code that checkout has.

## Not covered

- A PR merge skips the tests, so the PR's own test run is the only gate. Make the `test` check required in the branch protection for `dev`, so a PR can't merge with failing tests.
- Old images build up on the server after each deploy. They aren't pruned, because `docker image prune` would affect every project on the host.
- The workflow passes `actionlint` 1.7.1, and the classify query was checked against the real merge commit `e25b394` (match) and the plain commit `f4c13dc` (no match). It hasn't run on GitHub yet, because no runner is registered.
