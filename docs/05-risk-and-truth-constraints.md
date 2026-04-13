# Risk and Truth Constraints

## Non-negotiable truth rules

- Do not fabricate hops.
- Do not imply the trace is the user's browser, OS, or ISP resolver path.
- Do not collapse distinct terminal failures into one generic error.
- Do not hide support nameserver-address lookups from advanced users or JSON export.

## Load-bearing risks

- Deployment runtime may not support truthful UDP/TCP DNS behavior
- Missing glue and out-of-bailiwick nameservers complicate the hop narrative
- Anonymous traces can become an abuse vector without rate limits and destination policy
- Live DNS behavior is nondeterministic, so tests must stay local and deterministic

## Public service safeguards

- Public-IP-only upstream destinations
- Conservative concurrency caps
- Conservative timeouts and hop budgets
- Redacted structured logs
