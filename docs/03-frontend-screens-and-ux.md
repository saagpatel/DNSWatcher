# Frontend Screens and UX

## Query screen

Must include:

- DNSWatcher branding
- Systems Explainer Arcade flagship positioning
- "DNS: Follow the Question" hero and subhead
- Domain input
- Query type selector
- "Follow the question" run button
- Truth note
- Sample presets
- Recent trace metadata list

## Trace screen

Must include:

- Re-run trace
- Export JSON
- Beginner and advanced toggle
- Back to search
- Question path panel
- Vertical timeline
- Details panel
- Official source cards
- Raw JSON export button

## Support lookups

Support nameserver-address lookups are rendered as expandable substeps underneath the hop that triggered them. They remain visible in beginner mode, advanced mode, and JSON export.

## Visual states

The timeline must distinguish these states with text labels plus color/treatment, never color alone:

- Referral
- Referral with glue
- Referral via support lookup
- Support lookup
- CNAME restart
- TCP fallback
- Authoritative answer
- NODATA
- NXDOMAIN
- Timeout
- Refused
- Unusable referral or stopped error
- Max depth or upstream budget

## Beginner details

Each selected hop answers:

- What happened?
- Why this server?
- Why next?
- Why stop?

## Advanced details

Include:

- QNAME and QTYPE
- Queried server name and IP
- Transport and latency
- Response code
- Authoritative and truncated state
- Answer, authority, and additional summaries
- TTLs
- Next targets
- Technical note

## Accessibility and motion

- Query controls, presets, recent traces, timeline hops, support hops, mode toggle, export, and source links must be keyboard reachable.
- Timeline and detail surfaces use semantic headings, lists, buttons, `details`/`summary`, and screen-reader-readable labels.
- Focus-visible states must be obvious.
- State meaning cannot depend on color alone.
- Motion is non-essential and respects `prefers-reduced-motion`.

## Performance posture

- Use semantic DOM as the source of truth.
- Avoid Canvas or SVG for core mechanics unless a later module needs it.
- Avoid layout shifts in the trace workspace; fixed labels and stable panels should keep CLS low.
- Core Web Vitals design targets: LCP under 2.5s, INP under 200ms, CLS under 0.1.
