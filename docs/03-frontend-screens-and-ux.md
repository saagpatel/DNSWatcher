# Frontend Screens and UX

## Query screen

Must include:

- DNSWatcher branding
- Hero and subhead
- Domain input
- Query type selector
- Run trace button
- Truth note
- Sample presets
- Recent trace metadata list

## Trace screen

Must include:

- Re-run trace
- Export JSON
- Beginner and advanced toggle
- Back to search
- Vertical timeline
- Details panel

## Support lookups

Support nameserver-address lookups are rendered as expandable substeps underneath the hop that triggered them. They remain visible in advanced mode and in JSON export.

## Beginner details

Each selected hop answers:

- What was asked?
- Who answered?
- What came back?
- Why did the trace continue or stop?

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
