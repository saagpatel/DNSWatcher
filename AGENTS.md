# DNSWatcher Agent Rules

## Core posture

- Stay truth-first. Do not turn this into a lookup box with decorative hops.
- Keep scope disciplined. V1 is limited to `A`, `AAAA`, and `NS`.
- Preserve protocol facts needed to explain the trace honestly.

## Backend constraints

- Use direct DNS message exchange. Do not use the OS/system resolver in the trace path.
- Preserve per-hop transport, authoritative state, truncation state, and response code.
- Treat support nameserver-address lookups as real trace steps.
- Query only globally routable public IP destinations in the public service path.
- Block special-use/private IP destinations and terminate as `unusable_referral` when no safe next hop remains.

## Frontend constraints

- The UI must never imply this is the user's local resolver path.
- The truth note must remain visible on the query screen and trace screen.
- Support lookups render as expandable substeps beneath the triggering hop.
- JSON export is client-side only and must export the normalized `TraceResult` object without UI decoration.

## Product non-goals

- No browser/OS/ISP resolver introspection
- No packet capture
- No compare mode
- No backend persistence
- No extra RR types beyond `A`, `AAAA`, `NS`
- No DNSSEC workflow in v1

## Verification rules

- Prefer deterministic local DNS lab tests over live-network tests.
- Do not rely on public internet DNS behavior in CI.
- Keep docs, examples, generated types, and runtime output in sync.

<!-- portfolio-context:start -->
# Portfolio Context

## What This Project Is

DNSWatcher is an active local project in the /Users/d/Projects portfolio.

## Current State

Portfolio truth currently marks this project as `active` with `boilerplate` context. Phase 104 recovered minimum-viable context so future sessions can resume without rediscovery.

## Stack

- Stack still needs a deeper explicit handoff beyond this minimum context.

## How To Run

- Review the README and top-level scripts before the next session; this repo does not yet expose one canonical run command inside the new context block.

## Known Risks

- This repo only has minimum-viable recovery context today; deeper handoff details may still live in the README and supporting docs.

## Next Recommended Move

Use this context plus the README and supporting docs to resume the next active task, then promote the repo beyond minimum-viable by capturing a dedicated handoff, roadmap, or discovery artifact.

<!-- portfolio-context:end -->
