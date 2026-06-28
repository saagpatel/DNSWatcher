#!/usr/bin/env bash
set -euo pipefail

if [[ -z "${BASE_URL:-}" ]]; then
  echo "BASE_URL is required, for example: BASE_URL=https://dnswatcher.example.com $0" >&2
  exit 1
fi

"$(dirname "$0")/runtime-smoke.sh"

python3 - "$BASE_URL" <<'PY'
import json
import random
import sys
import time
import urllib.error
import urllib.request

base_url = sys.argv[1].rstrip("/")
client_ip = f"198.51.100.{random.randint(1, 254)}"


def request(method, path, payload=None, expected_status=None, headers=None):
    data = None
    request_headers = dict(headers or {})
    if payload is not None:
        data = json.dumps(payload).encode("utf-8")
        request_headers["Content-Type"] = "application/json"
    req = urllib.request.Request(base_url + path, data=data, headers=request_headers, method=method)
    started = time.perf_counter()
    try:
        with urllib.request.urlopen(req, timeout=20) as resp:
            body = resp.read().decode("utf-8")
            status = resp.status
    except urllib.error.HTTPError as err:
        body = err.read().decode("utf-8")
        status = err.code
    latency_ms = int((time.perf_counter() - started) * 1000)
    if expected_status is not None and status != expected_status:
        raise SystemExit(f"{method} {path} expected {expected_status}, got {status}: {body}")
    try:
        parsed = json.loads(body)
    except json.JSONDecodeError:
        parsed = {"raw": body}
    return status, parsed, latency_ms


status, health, latency = request("GET", "/healthz", expected_status=200)
if health.get("status") != "ok":
    raise SystemExit(f"unexpected health response: {health}")
print(f"ok: /healthz responsive before rate check ({latency}ms)")

statuses = []
for _ in range(6):
    status, body, latency = request(
        "POST",
        "/api/v1/traces",
        {"domain": "bad..domain", "qtype": "A"},
        headers={"X-Forwarded-For": client_ip},
    )
    statuses.append((status, body.get("error"), latency))

if not any(status == 429 and error == "rate_limited" for status, error, _ in statuses):
    raise SystemExit(f"expected one request to hit rate_limited 429, got {statuses}")
print(f"ok: rate limit returned 429 for synthetic client {client_ip}")

status, health, latency = request("GET", "/healthz", expected_status=200)
if health.get("status") != "ok":
    raise SystemExit(f"unexpected post-rate health response: {health}")
print(f"ok: /healthz responsive after rate check ({latency}ms)")
PY
