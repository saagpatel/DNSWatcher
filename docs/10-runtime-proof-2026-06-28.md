# Runtime Proof Attempt - 2026-06-28

## Scope

Mode: deploy-approved runtime proof for DNSWatcher only.

Goal: prove whether the current **DNS: Follow the Question** flagship can run
truthfully on a real host without weakening product truth.

## Local Source State

- Branch: `feat/add-ci`
- Remote: `https://github.com/saagpatel/DNSWatcher.git`
- Upstream: `origin/feat/add-ci`
- Current flagship implementation is local and uncommitted.
- Render's normal Git-backed deploy path cannot deploy these local uncommitted
  changes. Deploying the current remote branch would not prove the current
  flagship implementation.

## Render Access Check

- Render CLI installed: `render v2.20.0`
- Authenticated user: Saagar / `saagar210@gmail.com`
- Active workspace: `Saagar's workspace`
- Existing services listed: one unrelated `mcpaudit-chatgpt-app-service`
- DNSWatcher service found: none
- `render blueprints validate render.yaml`: valid, one planned service
  (`dnswatcher`)

## Verification Commands

Passed:

```sh
make generate
make test
make build
make docker-build
render blueprints validate render.yaml
curl -fsS http://127.0.0.1:8081/healthz
```

Failed as runtime-path evidence:

```sh
BASE_URL=http://127.0.0.1:8081 make runtime-smoke
```

Failure:

```text
ok: /healthz
example.com A expected outcome success, got refused.
```

## Local Container Evidence

The locally built container started successfully on `127.0.0.1:8081`.

`GET /healthz` returned:

```json
{"status":"ok"}
```

`POST /api/v1/traces` for `example.com A` returned a modeled DNS failure:

- `final_outcome.kind`: `refused`
- first hop server: `a.root-servers.net.` / `198.41.0.4`
- first hop `hop_purpose`: `delegation`
- RRset and next-target fields serialized as arrays, not `null`
- no `hop_purpose: terminal`
- truth notes present, including backend trace service, not user resolver path,
  live results vary, and QNAME minimization deferred

Container log evidence:

```text
"msg":"trace_completed","client":"346840d5a3d9","qtype":"A","outcome":"refused","hop_count":1
```

The log uses a hashed client key and does not include the raw queried domain.

## Result

Runtime proof is not complete.

The current local environment and local Docker container can build and boot the
app, but outbound iterative DNS to public root servers returns `REFUSED`.
That is a runtime-path proof failure, not evidence that the deterministic trace
engine is broken.

Render was not deployed because the current flagship implementation is not
available to Render's Git-backed deploy path and no prebuilt registry image URL
was supplied. A deploy from the current remote branch would not prove this local
flagship state.

## Next Required Move

Choose one source-publication path, then deploy:

1. Approve commit and push of the current DNSWatcher branch, then create a
   Render Web Service from `render.yaml` and run `BASE_URL=<render-url> make
   runtime-smoke`.
2. Or provide/approve a registry image path and push `dnswatcher:local` to that
   registry, then create an image-backed Render service.

If Render passes health but returns immediate `refused` or `timeout` from the
first public DNS trace, stop and record Render as incompatible for this product
truth lane. The next candidate should be Koyeb, Fly.io, or a small Docker VM
where outbound UDP/TCP destination port 53 can be explicitly allowed.
