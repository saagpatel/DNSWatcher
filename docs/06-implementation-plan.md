# Implementation Plan

## Phase 0

- Freeze the OpenAPI contract
- Write canonical examples
- Document runtime class, destination policy, and logging rules

## Phase 1

- Build deterministic DNS lab
- Implement iterative trace engine
- Implement classification logic and guardrails

## Phase 2

- Add hardened HTTP API
- Add validation, limits, and structured logging

## Phase 3

- Build React UI against fixtures
- Bind to the live API
- Add client-side export and recent metadata

## Phase 4

- Package the app for same-origin serving
- Verify runtime DNS behavior in a single-region deployment
- Run the runtime checklist in `docs/08-runtime-verification.md`
