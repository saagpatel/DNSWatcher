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
- `hop_purpose`: `delegation`, `nameserver_address_lookup`, or `cname_follow`.
  Terminal state is represented by `final_outcome.terminal_hop_index` and the
  terminal hop's `response_kind`/`response_code`, not by a separate hop
  purpose. This keeps the contract aligned with the backend runtime: a final
  answer, NODATA, NXDOMAIN, timeout, refused response, or unusable referral is
  still part of the delegation/CNAME/support path that produced it.

## Status code rules

- `200`: trace completed and returned a modeled DNS outcome
- `400`: invalid request
- `405`: wrong method
- `415`: wrong content type
- `429`: rate-limited or concurrency-limited
- `500`: unexpected service failure
