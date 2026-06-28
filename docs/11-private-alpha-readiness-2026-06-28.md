# Private Alpha Readiness - 2026-06-28

## Decision

DNSWatcher is ready for a Render-hosted private alpha on the `feat/add-ci`
branch.

It is not yet approved for a public launch.

## Current Runtime

- Host: Render Web Service
- Service ID: `srv-d90g7a8k1i2s73flbo3g`
- Region: `oregon`
- URL: `https://dnswatcher.onrender.com`
- Branch: `feat/add-ci`
- Runtime class: Docker web service
- Plan: Render Free

## Release Checks

Required local checks:

```sh
make generate
make test
cd frontend && npm run lint
make build
make docker-build
render blueprints validate render.yaml
git diff --check
```

Required hosted checks:

```sh
BASE_URL=https://dnswatcher.onrender.com make runtime-smoke
BASE_URL=https://dnswatcher.onrender.com make private-alpha-check
```

Browser QA:

- query screen loads nonblank
- truth note visible on query screen
- `www.github.com / A` trace succeeds and shows CNAME continuation
- beginner and advanced modes work
- source cards remain official-source only
- raw export button is enabled after a trace
- 1440 px desktop viewport has no horizontal overflow
- browser console has no relevant warnings/errors

## What Private Alpha Proves

- Render can boot the same-origin frontend/API container.
- Render can make outbound iterative DNS requests needed for `A`, `AAAA`, `NS`,
  and the flagship CNAME smoke path.
- The API rejects invalid input with `400 invalid_domain_input`.
- Rate limiting returns `429 rate_limited` for a synthetic repeated client.
- The service remains healthy after the rate-limit check.
- Logs use hashed client identifiers and do not include raw queried domains.

## Remaining Public-Launch Gates

- Run a short load/cold-start observation window on Render Free or move to a paid
  plan before public traffic.
- Decide log retention and alert thresholds for repeated `timeout`, `refused`,
  `servfail`, and `max_depth` outcomes.
- Add a small browser QA matrix for mobile and narrow tablet widths.
- Decide whether a custom domain and non-free Render plan are required for public
  launch.
- Merge `feat/add-ci` to `main` only after PR review/CI is green.

## Rollback

Render has deploy history for the service. If a branch-head deploy regresses the
runtime smoke or browser QA, roll back to the latest deploy that passed:

- `d77360c48d40fb3d03df4a371aeb5faebcd59ba2` for the runtime/UI repair
- a later docs-only deploy is acceptable only when the app tree matches a
  verified runtime commit

Do not loosen DNS truth constraints, destination filtering, or source
requirements as a rollback substitute.

## PR Posture

Open a PR from `feat/add-ci` to `main`. The PR should say:

- private alpha is approved on Render
- public launch still requires load/cold-start/logging decisions
- local and remote branch SHAs may differ in this workspace because publication
  used the GitHub Git database API after local `git push` was blocked, but the
  final trees match
