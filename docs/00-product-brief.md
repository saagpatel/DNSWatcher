# DNSWatcher Product Brief

## Hero

- Product: DNSWatcher
- Flagship: DNS: Follow the Question
- Hero line: DNS: Follow the Question
- Subhead: Ask for a DNS record, then follow the real backend iterative path from root hints through referrals, glue, support lookups, aliases, answers, and failures.

## Product thesis

DNSWatcher makes DNS resolution understandable by turning a real backend iterative trace into an "I finally understand this" journey. The product explains the mechanics of delegation, glue, nameserver-address support lookups, CNAME restarts, TCP fallback, and terminal DNS outcomes without pretending to reproduce the user's resolver path.

## Target users

- Primary: technical learners, support engineers, sysadmin learners, junior platform engineers, and developers with shallow DNS intuition
- Secondary: demos, interview prep, and "show me why this failed" explanations

## Frozen v1 scope

- Query types: `A`, `AAAA`, `NS`
- Real backend iterative tracing
- Referral following from root to authoritative servers
- Support lookups for nameserver addresses when glue is missing
- Visible CNAME continuation
- Distinct terminal outcomes
- Visual states for referral, glue, support lookup, CNAME restart, TCP fallback, NODATA, NXDOMAIN, timeout, refused, unusable referral, and max-depth
- Vertical timeline with expandable support substeps
- Beginner and advanced detail modes
- JSON export of the normalized trace result
- Recent trace metadata in browser storage only
- Concise official source cards

## Explicit non-goals

- User-path resolver introspection
- Packet capture
- Propagation checking
- Collaboration, accounts, or sharing
- Backend trace persistence

## Truth note

Traces are performed by the backend trace service. Timings reflect that service's network path, not your device's resolver path.

## Source requirements

- DNS behavior must be grounded in official DNS sources: RFC 1034, RFC 1035, RFC 9210, and IANA root hints/root server data.
- Accessibility behavior must be grounded in WCAG and MDN platform documentation.
- Performance targets should use Core Web Vitals design goals: LCP under 2.5s, INP under 200ms, CLS under 0.1.
- UI copy must not overstate what the backend observed. It can explain the trace, but the raw `TraceResult` remains authoritative.
