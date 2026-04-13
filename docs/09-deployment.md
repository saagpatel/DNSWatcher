# Deployment

## Recommended host

DNSWatcher should ship first on Render as a private-alpha web service.

Why this is the default:

- It avoids the Fly billing blocker for the first live proof stage.
- It supports the app's same-origin container shape using the existing Dockerfile.
- It is good enough for a controlled alpha even though it is not the final public-launch host by default.

Useful official references:

- [Render free tier](https://render.com/docs/free)
- [Render web services](https://render.com/docs/web-services)
- [Render Docker deploys](https://render.com/docs/docker)
- [Render Blueprint spec](https://render.com/docs/blueprint-spec)

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
