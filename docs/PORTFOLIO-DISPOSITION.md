# DNSWatcher — Portfolio Disposition

**Status:** Active (self-hosted service, public-facing release
candidate) — **Go backend** (iterative DNS trace engine +
deterministic DNS lab + HTTP API) + React TypeScript frontend +
OpenAPI contract on `origin/main`, with **Dockerfile + fly.toml +
render.yaml** deployment assets shipped. **Third self-hosted
service cluster member** (after RedditSentimentAnalyzer + IT
Service Health). Introduces new sub-shape: **public-facing
self-hosted service** (operator-self-hosts-for-public, distinct
from RSA's personal-self-hosted-for-external and ITSH's
corporate-internal). Operator still selecting deployment target —
local stash has `scripts/koyeb-deploy.sh` plus `fly.toml` and
`render.yaml` on canonical main. **Second single-commit canonical
history** in portfolio after Evolution Sandbox.

> Disposition uses strict `origin/main` verification.
> **Second single-commit history; first public-facing self-hosted
> service sub-shape.**

---

## Verification posture

Only `origin` (`saagpatel/DNSWatcher`). Clean migration state.

`origin/main`:

- Tip: `7a0b31a` Initial DNSWatcher release candidate
- **Only 1 commit total.** Like Evolution Sandbox, the operator
  squashed all features into a single "release candidate" commit
  for git-history hygiene.
- Repo tree has substantive content:
  - `backend/` — Go trace engine, deterministic DNS lab, HTTP API
  - `frontend/` — React UI, generated types from OpenAPI, fixtures,
    tests
  - `contracts/` — OpenAPI contract + canonical trace examples
  - `docs/` — product / architecture / implementation constraints
    / runtime verification checklist
  - `scripts/` — including `runtime-smoke.sh`
  - **Three deployment manifests on canonical main**: `Dockerfile`
    + `fly.toml` + `render.yaml`
  - `.dockerignore` + `Makefile` + `README.md` + `AGENTS.md`
- Default branch: `main`

---

## Current state in one paragraph

DNSWatcher is a **truth-first DNS trace viewer**. Runs a **real
backend iterative trace** from root toward an answer, referral, or
terminal failure, and renders the trace in an educational UI. Per
README: "**This is a backend-run iterative trace service. Timings
reflect the backend service's network path, not the user's device
or resolver path. This is not a packet capture tool. This is not a
browser-only DoH viewer with animation.**" The operator's
truth-first framing extends to runtime posture: conservative
timeouts, explicit rate limiting, public-IP-only DNS egress.
Support lookups for nameserver addresses appear as expandable
substeps in the UI. QNAME minimization is implemented. Architecture
is **stateless HTTP backend (Go) + static frontend (React)** —
ideal for single-region container or VM deployment.

The operator has shipped three deploy-target configs on canonical
main (`Dockerfile`, `fly.toml`, `render.yaml`) plus a runtime
smoke script. **Recommended private-alpha host: Render Free Web
Service in `oregon`**. Active state because the operator hasn't
yet declared a single canonical deploy target.

---

## Why "Active (self-hosted service, public-facing RC)" — third cluster member, new sub-shape

The self-hosted service cluster gains a third member with a new
sub-shape:

| Member | Audience | Hosting | Sub-shape |
|---|---|---|---|
| RedditSentimentAnalyzer (R10) | External users | launchd + nginx | Personal self-hosted for external |
| IT Service Health (R17.4) | Operator's employer (Box IT) | launchd + Caddy + Cloudflare Tunnel | Corporate-context internal |
| **DNSWatcher** | **Public users (truth-first DNS education)** | **Container (Render / Fly / Koyeb / Docker)** | **Public-facing container-deployed** |

Distinct sub-shape characteristics:
- **Public-facing**: rate limiting + conservative timeouts +
  public-IP-only DNS egress = operator hardening for unknown
  caller volume
- **Container-deployed**: Dockerfile + fly.toml + render.yaml all
  on canonical main = operator considering multiple PaaS providers
- **Multi-deploy-target indecision**: three manifests + a local
  `koyeb-deploy.sh` script in stash = operator hasn't picked yet
- **Truth-first product positioning** — strong distinguishing
  marketing hook (vs marketing-fluffy DNS viewers)

State is Active because:
- Operator hasn't selected a deploy target yet
- README explicitly calls this a "release candidate"
- Multiple deploy manifests suggest decision-in-flight
- Recommended private-alpha is "Render Free Web Service" —
  evaluation tier, not committed production

---

## Cluster taxonomy update

| Cluster | Count | Sub-shapes |
|---|---|---|
| **Self-hosted service** | **3** | personal-for-external (RSA) / corporate-context-internal (ITSH) / **public-facing-container-deployed (DNSWatcher)** |
| (others unchanged) | | |

Self-hosted service cluster reaches 3 with 3 distinct sub-shapes
— matches the maturity pattern seen in iOS / static-host / Chrome
MV3 / operator-tool clusters where multiple sub-shapes emerge.

---

## Unblock trigger (operator)

1. **Pick a deployment target.** Decision tree:
   - **Render Free** (README's recommendation) — zero-friction
     private alpha, but Free tier has cold-start delays.
   - **Fly.io** — `fly.toml` ready, global edge network, good for
     a DNS service, but charges per VM-hour.
   - **Koyeb** (stash has deploy script) — Free tier with
     persistent presence, no cold start.
   - **Docker on operator VPS** — full control, more maintenance.
   Recommended: **start with Render Free for private alpha**, move
   to Fly or Koyeb once user volume justifies.
2. **Rate limiting tier sizing** — DNS trace is recursive +
   network-bound; per-IP rate limits should prevent abuse but
   allow educational use.
3. **DNS egress restrictions** — public-IP-only egress per
   README; verify on the chosen deploy target.
4. **Backend timeout budget** — iterative trace can hang on slow
   nameservers; conservative timeouts must balance correctness
   vs UX.
5. **OpenAPI contract stability** — `contracts/` is the
   front/backend boundary; treat as a contract.
6. **`docs/08-runtime-verification.md` checklist** before treating
   any host as production-ready (operator-written gate).

Estimated operator time: ~3-4 hours for deploy target decision +
private alpha rollout.

---

## Portfolio operating system instructions

| Aspect | Posture |
|---|---|
| Portfolio status | `Active (self-hosted service, public-facing release candidate)` |
| Distribution channel | **Container (Render / Fly / Koyeb / Docker)** — operator decision pending |
| Audience | **Public users** (truth-first DNS education) |
| Review cadence | Active — driven by deploy-target decision |
| Resurface conditions | (a) Deploy target selected, (b) private alpha goes live, (c) `docs/08-runtime-verification.md` passes, (d) Go / React major version, (e) DNS protocol change (DoH / DoT semantics), (f) graduation to Release Frozen once stable |
| Co-batch with | Self-hosted service cluster — **now 3 repos with 3 sub-shapes** |
| Sub-shape | **Public-facing container-deployed** (new) |
| Special concern | **Multi-deploy-target indecision.** Three manifests + a stash script = pick one. |
| Special concern | **Truth-first product positioning** is a strong differentiator. Lead marketing copy with it. |
| Special concern | **Rate limiting + DNS egress restrictions** are public-service posture defaults. Verify on chosen host. |
| Special concern | **Second single-commit canonical history** pattern (after Evolution Sandbox). Worth recognizing as operator workflow preference. |
| Special concern | **`docs/08-runtime-verification.md`** — operator-written gate must pass before production. |

---

## Reactivation procedure

1. Verify branch tracking.
2. Review stash `r18-dnsw-stash` (AGENTS.md + Makefile + README.md
   + `docs/09-deployment.md` mods + .codex/ + **`scripts/koyeb-
   deploy.sh`**). The koyeb deploy script suggests operator
   leaning toward Koyeb; inspect before discarding.
3. **Read `docs/08-runtime-verification.md`** — operator-written
   production-ready gate.
4. Run `make install && make generate && make test` per README
   workflow.
5. Decide deploy target (recommend Render Free for private alpha).
6. Run `scripts/runtime-smoke.sh` against the deployed instance.
7. Verify rate limiting + DNS egress posture against host.

---

## Last known reference

| Field | Value |
|---|---|
| `origin/main` tip | `7a0b31a` Initial DNSWatcher release candidate (single-commit canonical history) |
| Default branch | `main` |
| Build system | **Go** (backend) + **React + TypeScript** (frontend) + OpenAPI contract + Docker |
| Architecture | **Stateless HTTP backend + static frontend** — single-region container or VM |
| Audience | **Public users** (truth-first DNS education) |
| Deploy targets shipped | Dockerfile + fly.toml + render.yaml (+ local stash: `scripts/koyeb-deploy.sh`) |
| Recommended alpha host | Render Free Web Service in `oregon` |
| Distinguishing tech | **Real backend iterative trace from root** + **QNAME minimization** + **OpenAPI-typed front/back boundary** + **truth-first product positioning** + multi-deploy-target manifests |
| Production-ready gate | `docs/08-runtime-verification.md` (operator-written checklist) |
| Migration state | No `legacy-origin` remote |
| Distinguishing feature | **Third self-hosted service cluster member; first public-facing-container-deployed sub-shape.** Self-hosted service cluster reaches 3 with 3 sub-shapes. Second single-commit canonical history in portfolio. |
