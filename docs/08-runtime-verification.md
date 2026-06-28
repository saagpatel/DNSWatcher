# Runtime Verification

DNSWatcher is not ready for public runtime use until the chosen host proves that the backend can make outbound UDP and TCP DNS queries on port 53 from a single region.

If the live checks fail immediately with `refused` or `timeout` on the first public trace, treat that as a likely environment or egress-network problem first, not automatically as an application bug.

## Preconditions

- Deploy the Go API on a single-region container or VM style host.
- Serve the built frontend same-origin if possible.
- Keep default public-IP-only destination filtering enabled.
- Use production-like timeout and concurrency settings.
- Confirm the host permits outbound UDP and TCP traffic to public DNS
  nameserver IPs on destination port 53. A normal platform resolver is not
  enough; DNSWatcher performs its own iterative DNS exchanges.

## Required checks

1. Verify `GET /healthz` returns `200`.
2. Run a successful `A` trace for `example.com`.
3. Run a successful `AAAA` trace for `example.com`.
4. Run a successful `NS` trace for `example.com`.
5. Run a trace that visibly follows a `CNAME`, such as `www.github.com` with `A`.
6. Run an invalid-input request and confirm the API returns `400` with `invalid_domain_input`.
7. Confirm the trace screen still shows the truth note in production.
8. Confirm the JSON export contains the raw normalized `TraceResult` object.
9. Confirm `hop_purpose` values are only `delegation`, `nameserver_address_lookup`, or `cname_follow`.
10. Confirm source cards link only to official references.

## DNS transport checks

1. Confirm normal UDP DNS queries succeed.
2. Confirm TCP fallback works when a truncated UDP response is encountered.
3. Confirm the runtime can reach public nameserver IPs from the backend region.
4. Confirm blocked private or special-use destination IPs still terminate as `unusable_referral`.
5. If `BASE_URL=http://127.0.0.1:8080 make runtime-smoke` fails locally with an
   immediate `refused` or `timeout`, record it as local runtime-path evidence
   and continue with a deployed host proof. Do not use local public DNS failure
   as CI evidence against the deterministic DNS lab tests.

## Frontend accessibility and performance checks

1. Keyboard through the domain input, qtype selector, run button, presets, recent traces, timeline hops, support hops, mode toggle, export button, and source links.
2. Confirm timeline states include visible text labels and are not color-only.
3. Confirm a screen reader can reach the trace status, timeline buttons, support details, and truth notes.
4. Confirm `prefers-reduced-motion` disables non-essential hover/transition motion.
5. Treat Core Web Vitals as release design targets: LCP under 2.5s, INP under 200ms, CLS under 0.1. Measure them on the chosen host before public launch.

## Logging checks

1. Confirm request logs contain a hashed client key, not the raw client address.
2. Confirm logs do not contain raw queried domains in normal success or failure paths.
3. Confirm timeout and refusal outcomes still log outcome metadata and duration.

## Rate limit and load checks

1. Trigger rate limiting and confirm the API returns `429`.
2. Trigger concurrency limiting and confirm the API returns `429`.
3. Confirm the service remains responsive after repeated one-off client requests.

For private alpha, run:

```sh
BASE_URL=<candidate> make private-alpha-check
```

That command runs the normal runtime smoke, then verifies hosted rate limiting
with a synthetic client and checks `/healthz` remains responsive afterward.

## Release gate

DNSWatcher is only runtime-ready when all checks above pass on the chosen host without changing the product truth note or relaxing the public-IP-only DNS egress policy.

## Public-readiness evidence packet

Before public launch, capture:

- The exact candidate host, region, and URL.
- `make test` and `make build` results from the release commit.
- `BASE_URL=<candidate> make runtime-smoke` output.
- Browser evidence for query screen, trace screen, truth notes, official source
  links, beginner mode, advanced mode, and raw JSON export.
- One successful `A`, `AAAA`, `NS`, and CNAME trace JSON sample with no
  `hop_purpose: terminal` values and no `null` RRset/next-target arrays.
- One invalid-input `400` response.
- Logging and rate-limit evidence.
