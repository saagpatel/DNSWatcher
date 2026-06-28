# DNSWatcher

DNSWatcher is the first Systems Explainer Arcade flagship: **DNS: Follow the Question**. It runs a real backend iterative trace from root hints toward an answer, referral, CNAME restart, or terminal failure and renders the path as a truthful educational experience.

## Product truth

- This is a backend-run iterative trace service.
- Timings reflect the backend service's network path, not the user's device or resolver path.
- This is not a packet capture tool.
- This is not a browser-only DoH viewer with animation.
- Support lookups for nameserver addresses are real trace steps and appear as expandable substeps in the UI.
- V1 supports only `A`, `AAAA`, and `NS`.
- QNAME minimization is deferred and called out in the product surface.
- Terminal state is represented by `final_outcome.terminal_hop_index` and the terminal hop response fields, not by a separate `hop_purpose`.

## Flagship experience

- Beginner mode explains what happened, why that server was queried, why the trace continues, and why it stops.
- Advanced mode preserves raw protocol fields: qname, qtype, server, IP, transport, latency, rcode, AA, TC, RRsets, and next targets.
- Visual states distinguish referral, glue, support lookup, CNAME restart, TCP fallback, NODATA, NXDOMAIN, timeout, refused, unusable referral, and max-depth stops.
- Source cards link to official DNS and accessibility references without replacing the raw trace as the source of truth.
- JSON export remains client-side and exports the raw normalized `TraceResult`.

## Repository layout

- `contracts/` holds the OpenAPI contract and canonical trace examples.
- `docs/` holds the product, architecture, implementation constraints, and runtime verification checklist.
- `backend/` holds the Go trace engine, deterministic DNS lab, and HTTP API.
- `frontend/` holds the React UI, generated types, fixtures, and tests.

## Local workflow

1. `make install`
2. `make generate`
3. `make test`
4. `cd frontend && npm run dev`
5. `cd backend && go run ./cmd/dnswatcher-api`

During local development, the Vite frontend proxies `/api` to the Go backend.

## Frontend test note

- Frontend tests run under Vitest + jsdom.
- Shared browser test shims live in `frontend/src/test/setup.ts`.
- Storage tests rely on the shared in-memory `localStorage` mock from that setup file rather than a real browser runtime.

## Runtime posture

- Preferred runtime class: single-region container or VM
- Backend serving model: stateless HTTP
- Frontend serving model: static assets, ideally same-origin in production
- Public service posture: conservative timeouts, explicit rate limiting, and public-IP-only DNS egress
- Deployment proof: follow `docs/08-runtime-verification.md` before treating a host as production-ready
- Recommended private-alpha host: Render Free Web Service in `oregon`
- Deployment assets: `Dockerfile`, `render.yaml`, and `scripts/runtime-smoke.sh`

## QNAME minimization

V1 defers QNAME minimization. The backend sends the full query name during iterative tracing. This is documented explicitly so the product remains technically honest and the behavior can be revised later without surprise.

## Official references

- [RFC 1034](https://www.rfc-editor.org/rfc/rfc1034)
- [RFC 1035](https://www.rfc-editor.org/rfc/rfc1035)
- [RFC 9210](https://www.rfc-editor.org/rfc/rfc9210)
- [IANA root hints and root servers](https://www.iana.org/domains/root/files)
- [WCAG 2.2 Animation from Interactions](https://www.w3.org/TR/WCAG22/#animation-from-interactions)
- [MDN prefers-reduced-motion](https://developer.mozilla.org/en-US/docs/Web/CSS/@media/prefers-reduced-motion)
- [web.dev Core Web Vitals](https://web.dev/vitals/)

## Deployment path

Use the Render path in `docs/09-deployment.md` for private alpha unless Render proves incompatible with truthful outbound DNS behavior. The repo is packaged to build one container that serves both the API and the frontend.
