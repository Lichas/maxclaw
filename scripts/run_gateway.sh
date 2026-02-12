#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GATEWAY_PORT="${GATEWAY_PORT:-18890}"

BIN="$ROOT_DIR/nanobot-go"
if [ ! -x "$BIN" ]; then
  BIN="$ROOT_DIR/build/nanobot-go"
fi

if [ ! -x "$BIN" ]; then
  echo "Error: nanobot-go binary not found in $ROOT_DIR/nanobot-go or $ROOT_DIR/build/nanobot-go" >&2
  exit 1
fi

exec "$BIN" gateway -p "$GATEWAY_PORT"
