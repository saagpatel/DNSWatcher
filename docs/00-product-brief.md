# DNSWatcher Product Brief

## Hero

- Product: DNSWatcher
- Hero line: DNS at a glance
- Subhead: Run a real iterative DNS trace and follow referrals from root to answer.

## Product thesis

DNSWatcher makes DNS resolution understandable by showing a real backend iterative trace from delegation to answer, not by pretending to reproduce the user's resolver path.

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
- Vertical timeline with expandable support substeps
- Beginner and advanced detail modes
- JSON export of the normalized trace result
- Recent trace metadata in browser storage only

## Explicit non-goals

- User-path resolver introspection
- Packet capture
- Propagation checking
- Collaboration, accounts, or sharing
- Backend trace persistence

## Truth note

Traces are performed by the backend trace service. Timings reflect that service's network path, not your device's resolver path.
