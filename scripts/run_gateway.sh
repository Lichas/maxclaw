#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GATEWAY_PORT="${GATEWAY_PORT:-18890}"

BIN="$ROOT_DIR/maxclaw"
if [ ! -x "$BIN" ]; then
  BIN="$ROOT_DIR/build/maxclaw"
fi

if [ ! -x "$BIN" ]; then
  echo "Error: maxclaw binary not found in $ROOT_DIR/maxclaw or $ROOT_DIR/build/maxclaw" >&2
  exit 1
fi

exec "$BIN" gateway -p "$GATEWAY_PORT"
