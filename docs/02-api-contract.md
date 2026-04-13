# DNSWatcher API Contract

## Endpoint

- `POST /api/v1/traces`
- `GET /healthz`

## Request

```json
{
  "domain": "www.github.com",
  "qtype": "A"
}
```

## Validation rules

- ASCII domain names only in v1
- Reject IP literals
- Accept optional trailing dot on input and strip it in normalized output
- Reject empty labels, overlong labels, and unsupported query types
- Reject special-use domains on the public service path

## Response model

- `TraceResult` always includes:
  - top-level timing
  - normalized domain
  - stable status and outcome fields
  - flat hop array with `parent_index` and `hop_purpose`
  - truth notes

## Hop truth fields

- `transport`: `udp` or `tcp`
- `authoritative`: whether the responder set the authoritative answer flag
- `truncated`: whether the response required TCP fallback
- `hop_purpose`: `delegation`, `nameserver_address_lookup`, `cname_follow`, or `terminal`

## Status code rules

- `200`: trace completed and returned a modeled DNS outcome
- `400`: invalid request
- `405`: wrong method
- `415`: wrong content type
- `429`: rate-limited or concurrency-limited
- `500`: unexpected service failure
