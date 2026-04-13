# Acceptance Criteria

## Product and UX

- Query screen includes the frozen branding, truth note, presets, and recent traces
- Trace screen includes timeline, details panel, rerun, and JSON export
- Beginner and advanced modes both work
- Support lookups render as expandable substeps

## Functionality

- Successful `A`, `AAAA`, and `NS` traces work
- Visible CNAME continuation works
- Distinct NXDOMAIN, timeout, refused, and unusable-referral behaviors render correctly
- Recent trace metadata is stored locally and reused for rerun

## Technical credibility

- Hops are not fabricated
- Response sections and TTLs are preserved
- Authoritative and truncation state are surfaced
- JSON export matches the normalized `TraceResult`
- CI tests do not depend on public internet DNS behavior
