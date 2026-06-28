# Deployment

## Recommended host

DNSWatcher should try Render first as a private-alpha web service, but only as
an evidence-gathering candidate. The product is not runtime-ready until the
host proves outbound UDP and TCP DNS queries to public nameserver IPs on port 53.

Why this is the default:

- It avoids the Fly billing blocker for the first live proof stage.
- It supports the app's same-origin container shape using the existing Dockerfile.
- It is good enough for a controlled alpha even though it is not the final public-launch host by default.
- Official Render docs confirm the web-service/Docker shape, but they do not
  by themselves prove that this app's iterative DNS egress requirements pass.

Useful official references:

- [Render free tier](https://render.com/docs/free)
- [Render web services](https://render.com/docs/web-services)
- [Render Docker deploys](https://render.com/docs/docker)
- [Render Blueprint spec](https://render.com/docs/blueprint-spec)
- [Render outbound IP addresses](https://render.com/docs/outbound-ip-addresses)
- [Fly UDP and TCP docs](https://fly.io/docs/networking/udp-and-tcp/)
- [Koyeb general FAQ](https://www.koyeb.com/docs/faqs/general)

## Default runtime shape

- One region: `oregon`
- One container serving both the frontend and backend
- Public HTTP entrypoint with `/healthz`
- No backend persistence
- Render Free for alpha only

## Repo assets

- `Dockerfile` builds the frontend, builds the Go API, and ships one runtime image.
- `render.yaml` defines the Render web service, region, health check, and environment defaults.
- `scripts/runtime-smoke.sh` runs the minimum live checks against a deployed base URL.

## Local packaging checks

1. Run `make build`
2. Run `make docker-build`
3. Confirm the image starts locally with `docker run --rm -p 8080:8080 dnswatcher:local`
4. Run `BASE_URL=http://127.0.0.1:8080 make runtime-smoke`

If the smoke check fails immediately with `refused` or `timeout`, assume local outbound DNS may be blocked and continue to the real host verification before judging the app broken.

## Runtime proof commands

Use these commands for any candidate host, replacing `BASE_URL` with the
candidate URL:

```sh
curl -fsS "$BASE_URL/healthz"
BASE_URL="$BASE_URL" make runtime-smoke
curl -fsS -X POST "$BASE_URL/api/v1/traces" \
  -H 'Content-Type: application/json' \
  -d '{"domain":"example.com","qtype":"A"}'
curl -fsS -X POST "$BASE_URL/api/v1/traces" \
  -H 'Content-Type: application/json' \
  -d '{"domain":"www.github.com","qtype":"A"}'
curl -fsS -X POST "$BASE_URL/api/v1/traces" \
  -H 'Content-Type: application/json' \
  -d '{"domain":"bad..domain","qtype":"A"}'
```

Evidence to capture:

- `/healthz` returns `{"status":"ok"}`.
- `example.com` succeeds for `A`, `AAAA`, and `NS`.
- `www.github.com A` includes at least one `response_kind: "cname"` hop.
- Every hop uses `hop_purpose` `delegation`, `nameserver_address_lookup`, or
  `cname_follow`; never `terminal`.
- Empty RRset and next-target fields serialize as arrays, not `null`.
- The browser trace screen shows the truth note, source cards, raw export
  button, beginner mode, and advanced raw protocol fields.
- Logs contain a hashed client key and outcome metadata, not raw queried
  domains.

## Render deploy steps

1. Install and log in to the Render CLI or use the Render dashboard.
2. Publish this project to a private GitHub repository.
3. Create a Render Web Service from the repo or sync `render.yaml`.
4. Use the default `onrender.com` hostname first.
5. Wait for `/healthz` to pass.
6. Run `BASE_URL=https://<your-service>.onrender.com make runtime-smoke`
7. Complete the full checklist in `docs/08-runtime-verification.md`

## Free-tier caveats

- Render Free can spin down on idle and may take time to wake up.
- Render Free is for alpha validation, not the default long-term public-launch posture.
- If Render Free proves incompatible with the DNS egress needs of this product, stop using it rather than weakening the product truth.
- Treat immediate `refused` or `timeout` from the first root-server hop as a
  host/network proof failure until another candidate host demonstrates success.

## Rollback posture

- If a deploy fails after a previously healthy Render service existed, roll back to the previous good deploy in Render.
- If Render never passes runtime smoke, treat that as a platform viability failure and stop the alpha there.
- Do not loosen the public-IP-only DNS egress policy to make a bad deploy pass.

## When to choose a different host

Use a different provider only if one of these is true:

- Render cannot support truthful outbound DNS behavior for this app
- a compliance constraint requires a different vendor
- pricing or org policy forces a different container/VM platform

If the host changes, keep the same runtime shape: one region, one normal container or VM, same-origin serving, and the same runtime verification checklist.

Candidate escalation order:

1. Render Web Service: simplest alpha path, but outbound DNS behavior must be
   proven live.
2. Koyeb Web Service: plausible second managed-container candidate because its
   public FAQ names outbound port 25 as the blocked port, but DNS egress still
   must be proven with this app.
3. Fly.io: viable only if billing and networking setup are acceptable; its
   official UDP/TCP docs are more about exposed services than this app's
   outbound iterative DNS need, so still verify live.
4. Docker on a small VM: strongest fallback because normal VM firewall rules can
   explicitly allow outbound UDP and TCP destination port 53.
