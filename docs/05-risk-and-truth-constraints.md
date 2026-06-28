# Risk and Truth Constraints

## Non-negotiable truth rules

- Do not fabricate hops.
- Do not imply the trace is the user's browser, OS, or ISP resolver path.
- Do not collapse distinct terminal failures into one generic error.
- Do not hide support nameserver-address lookups from advanced users or JSON export.
- Do not introduce decorative hops, resolver avatars, or animation-only states that are not backed by a real `TraceResult` hop.
- Do not reintroduce `hop_purpose: terminal`; terminal state belongs to `final_outcome.terminal_hop_index` and terminal hop response fields.
- Do not claim QNAME minimization in v1.

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

## Source-backed explanations

- Delegation, referrals, authority, DNS messages, RCODEs, truncation, and RRsets must be checked against RFC 1034 and RFC 1035.
- TCP fallback copy must be checked against RFC 9210 and the runtime trace's actual transport fields.
- Root-starting copy must be checked against the IANA root hints/root servers source.
- Accessibility and motion behavior must be checked against WCAG 2.2 and MDN platform references.
- Performance claims should remain design targets unless measured in the actual deployment environment.
