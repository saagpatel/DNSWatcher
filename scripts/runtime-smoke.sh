#!/usr/bin/env bash
set -euo pipefail

if [[ -z "${BASE_URL:-}" ]]; then
  echo "BASE_URL is required, for example: BASE_URL=https://dnswatcher.example.com $0" >&2
  exit 1
fi

python3 - "$BASE_URL" <<'PY'
import json
import sys
import urllib.error
import urllib.request

base_url = sys.argv[1].rstrip("/")


def request(method, path, payload=None, expected_status=200):
    data = None
    headers = {}
    if payload is not None:
        data = json.dumps(payload).encode("utf-8")
        headers["Content-Type"] = "application/json"
    req = urllib.request.Request(base_url + path, data=data, headers=headers, method=method)
    try:
        with urllib.request.urlopen(req, timeout=20) as resp:
            body = resp.read().decode("utf-8")
            status = resp.status
    except urllib.error.HTTPError as err:
        body = err.read().decode("utf-8")
        status = err.code
    if status != expected_status:
        raise SystemExit(f"{method} {path} expected {expected_status}, got {status}: {body}")
    return json.loads(body)


def assert_trace(domain, qtype, expected_outcome):
    body = request("POST", "/api/v1/traces", {"domain": domain, "qtype": qtype}, 200)
    outcome = body["final_outcome"]["kind"]
    if outcome != expected_outcome:
        raise SystemExit(
            f"{domain} {qtype} expected outcome {expected_outcome}, got {outcome}. "
            "If this host returns refused or timeout immediately, outbound DNS on port 53 may be blocked in this environment."
        )
    if not body["truth_notes"]:
        raise SystemExit(f"{domain} {qtype} returned no truth notes")
    print(f"ok: {domain} {qtype} -> {outcome} ({len(body['hops'])} hops)")
    return body


health = request("GET", "/healthz", expected_status=200)
if health.get("status") != "ok":
    raise SystemExit(f"unexpected health response: {health}")
print("ok: /healthz")

assert_trace("example.com", "A", "success")
assert_trace("example.com", "AAAA", "success")
assert_trace("example.com", "NS", "success")
cname = assert_trace("www.github.com", "A", "success")
if not any(hop["response_kind"] == "cname" for hop in cname["hops"]):
    raise SystemExit("expected www.github.com A trace to include a cname hop")
print("ok: www.github.com A includes cname hop")

invalid = request("POST", "/api/v1/traces", {"domain": "bad..domain", "qtype": "A"}, 400)
if invalid.get("error") != "invalid_domain_input":
    raise SystemExit(f"unexpected invalid input response: {invalid}")
print("ok: invalid domain rejected with invalid_domain_input")
PY
