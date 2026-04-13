# Runtime Verification

DNSWatcher is not ready for public runtime use until the chosen host proves that the backend can make outbound UDP and TCP DNS queries on port 53 from a single region.

If the live checks fail immediately with `refused` or `timeout` on the first public trace, treat that as a likely environment or egress-network problem first, not automatically as an application bug.

## Preconditions

- Deploy the Go API on a single-region container or VM style host.
- Serve the built frontend same-origin if possible.
- Keep default public-IP-only destination filtering enabled.
- Use production-like timeout and concurrency settings.

## Required checks

1. Verify `GET /healthz` returns `200`.
2. Run a successful `A` trace for `example.com`.
3. Run a successful `AAAA` trace for `example.com`.
4. Run a successful `NS` trace for `example.com`.
5. Run a trace that visibly follows a `CNAME`, such as `www.github.com` with `A`.
6. Run an invalid-input request and confirm the API returns `400` with `invalid_domain_input`.
7. Confirm the trace screen still shows the truth note in production.
8. Confirm the JSON export contains the raw normalized `TraceResult` object.

## DNS transport checks

1. Confirm normal UDP DNS queries succeed.
2. Confirm TCP fallback works when a truncated UDP response is encountered.
3. Confirm the runtime can reach public nameserver IPs from the backend region.
4. Confirm blocked private or special-use destination IPs still terminate as `unusable_referral`.

## Logging checks

1. Confirm request logs contain a hashed client key, not the raw client address.
2. Confirm logs do not contain raw queried domains in normal success or failure paths.
3. Confirm timeout and refusal outcomes still log outcome metadata and duration.

## Rate limit and load checks

1. Trigger rate limiting and confirm the API returns `429`.
2. Trigger concurrency limiting and confirm the API returns `429`.
3. Confirm the service remains responsive after repeated one-off client requests.

## Release gate

DNSWatcher is only runtime-ready when all checks above pass on the chosen host without changing the product truth note or relaxing the public-IP-only DNS egress policy.
