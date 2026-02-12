#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BRIDGE_PORT="${BRIDGE_PORT:-3001}"
BRIDGE_PROXY="${BRIDGE_PROXY:-${PROXY_URL:-${HTTPS_PROXY:-${HTTP_PROXY:-${ALL_PROXY:-}}}}}"

if [ -n "$BRIDGE_PROXY" ]; then
  export PROXY_URL="$BRIDGE_PROXY"
  export HTTPS_PROXY="$BRIDGE_PROXY"
  export HTTP_PROXY="$BRIDGE_PROXY"
  export ALL_PROXY="$BRIDGE_PROXY"
fi

exec env BRIDGE_PORT="$BRIDGE_PORT" node "$ROOT_DIR/bridge/dist/index.js"
