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

readonly REQUIRED_GO_TOOLCHAIN=go1.26.2
readonly GO_TOOLCHAIN="${REQUIRED_GO_TOOLCHAIN}+auto"

node_major="$(node -p 'process.versions.node.split(".")[0]')"
node_minor="$(node -p 'process.versions.node.split(".")[1]')"
node_patch="$(node -p 'process.versions.node.split(".")[2]')"
node_version="$(node --version)"
# Match the committed jsdom engine: ^22.22.2 || ^24.15.0 || >=26.0.0.
if ! (( (node_major == 22 && (node_minor > 22 || (node_minor == 22 && node_patch >= 2))) ||
        (node_major == 24 && node_minor >= 15) || node_major >= 26 )); then
  echo "Unsupported Node.js runtime ${node_version}; use Node 22.22.2+, Node 24.15+, or Node 26+." >&2
  exit 1
fi

test -f backend/go.mod
test -f backend/go.sum
test -f frontend/package-lock.json
test "$(awk '$1 == "go" { print $2; exit }' backend/go.mod)" = "1.26.2"

# Cursor Cloud may provide an older system Go. Ask the Go launcher to
# bootstrap and select the exact toolchain required by go.mod, while allowing
# the module's own minimum to remain authoritative.
export GOTOOLCHAIN="$GO_TOOLCHAIN"
selected_go_version="$(go env GOVERSION)"
if [[ "$selected_go_version" != "$REQUIRED_GO_TOOLCHAIN" ]]; then
  echo "Unexpected Go toolchain ${selected_go_version}; expected ${REQUIRED_GO_TOOLCHAIN}." >&2
  exit 1
fi

# npm ci and go.mod/go.sum are the dependency locks. The pinned GOTOOLCHAIN
# above makes the backend checks self-contained on fresh Cursor images.
(cd frontend && npm ci --no-audit --no-fund)

(cd backend && go test ./... && go build -buildvcs=false ./...)

(cd frontend && npm run generate:types && git diff --exit-code -- src/lib/api/generated.ts)
(cd frontend && npm run lint && npm test -- --run && npm run build)
