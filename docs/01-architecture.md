# DNSWatcher Architecture

## Top-level shape

- Frontend: React + TypeScript + Vite single-page app
- Backend: Go stateless HTTP service
- Contract: `contracts/openapi.yaml`
- Persistence: browser `localStorage` for recent metadata only

## Runtime model

- Development: Vite frontend and Go backend run separately with `/api` proxying
- Production: same-origin deployment preferred, with the Go service serving API traffic and static frontend assets
- Preferred runtime class: single-region container or VM

## Backend modules

- `internal/contracts`: API structs shared across handlers and tests
- `internal/trace`: iterative trace engine and normalization
- `internal/classify`: response classification helpers
- `internal/policy`: input validation and destination filtering
- `internal/httpapi`: HTTP handlers, response writing, and server configuration
- `internal/testkit`: deterministic DNS lab for local integration tests

## Security and privacy defaults

- Public-IP-only upstream querying in the public service path
- No raw domain logging by default
- Strict JSON-only POST endpoint
- Client-side JSON export only

## QNAME minimization

V1 does not implement QNAME minimization. The backend sends the full query name during iterative tracing and documents that choice explicitly.
