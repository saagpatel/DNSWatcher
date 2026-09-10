#!/usr/bin/env bash
set -euo pipefail

# Keep Cursor Cloud setup deterministic and offline with respect to DNS. The
# backend tests use their in-process DNS lab; the frontend uses committed
# fixtures. No deployed endpoint or live nameserver is contacted here.

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "DNSWatcher Cursor Cloud setup requires $1." >&2
    exit 1
  fi
}

require_command node
require_command npm
require_command go

node_major="$(node -p 'process.versions.node.split(".")[0]')"
node_minor="$(node -p 'process.versions.node.split(".")[1]')"
node_version="$(node --version)"
if (( node_major < 22 )) || (( node_major == 22 && node_minor < 13 )); then
  echo "Unsupported Node.js runtime ${node_version}; use Node 22.13+ or Node 24+." >&2
  exit 1
fi
if (( node_major == 23 )); then
  echo "Unsupported odd Node.js runtime ${node_version}; use Node 22.13+ or Node 24+." >&2
  exit 1
fi

test -f backend/go.mod
test -f backend/go.sum
test -f frontend/package-lock.json
test "$(awk '$1 == "go" { print $2; exit }' backend/go.mod)" = "1.26.2"

# npm ci and go.mod/go.sum are the dependency locks. GOTOOLCHAIN=local keeps
# Go from silently downloading a different toolchain during a cloud run.
(cd frontend && npm ci --no-audit --no-fund)

(cd backend && GOTOOLCHAIN=local go test ./... && GOTOOLCHAIN=local go build -buildvcs=false ./...)

(cd frontend && npm run generate:types && git diff --exit-code -- src/lib/api/generated.ts)
(cd frontend && npm run lint && npm test -- --run && npm run build)
