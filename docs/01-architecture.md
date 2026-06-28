# DNSWatcher Architecture

## Top-level shape

- Frontend: React + TypeScript + Vite single-page app
- Backend: Go stateless HTTP service
- Contract: `contracts/openapi.yaml`
- Persistence: browser `localStorage` for recent metadata only
- Explainer surface: semantic DOM timeline and detail panels; no Canvas/WebGPU dependency in the flagship

## Runtime model

- Development: Vite frontend and Go backend run separately with `/api` proxying
- Production: same-origin deployment preferred, with the Go service serving API traffic and static frontend assets
- Preferred runtime class: single-region container or VM

## Backend modules

- `internal/contracts`: API structs shared across handlers and tests
- `internal/trace`: iterative trace engine, normalization, and response classification
- `internal/policy`: input validation and destination filtering
- `internal/httpapi`: HTTP handlers, response writing, and server configuration
- `internal/testkit`: deterministic DNS lab for local integration tests

## Frontend modules

- `src/App.tsx`: flagship query journey, trace timeline, detail modes, source cards, and export action
- `src/app/state.ts`: reducer-owned UI state
- `src/lib/presenters.ts`: grouping and failure-copy presentation helpers
- `src/lib/api/generated.ts`: OpenAPI-generated API types
- `src/lib/exportTrace.ts`: client-side raw `TraceResult` export

## Security and privacy defaults

- Public-IP-only upstream querying in the public service path
- No raw domain logging by default
- Strict JSON-only POST endpoint
- Client-side JSON export only
- Source links point to official references only; source cards do not fetch or proxy third-party content

## Contract truth

`hop_purpose` is limited to `delegation`, `nameserver_address_lookup`, and `cname_follow`. Terminal state is modeled by `final_outcome.terminal_hop_index` plus the terminal hop's response fields so examples, generated types, frontend assumptions, and backend output stay aligned.

## QNAME minimization

V1 does not implement QNAME minimization. The backend sends the full query name during iterative tracing and documents that choice explicitly.
