#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BRIDGE_PORT="${BRIDGE_PORT:-3001}"
BRIDGE_PROXY="${BRIDGE_PROXY:-}"
FORCE_BRIDGE_KILL="${FORCE_BRIDGE_KILL:-}"

cd "$ROOT_DIR"

if ! command -v npm >/dev/null 2>&1; then
  echo "Error: npm not found. Please install Node.js and npm." >&2
  exit 1
fi

if ! command -v go >/dev/null 2>&1; then
  echo "Error: go not found. Please install Go." >&2
  exit 1
fi

resolve_bridge_proxy() {
  if [ -n "${BRIDGE_PROXY:-}" ]; then
    echo "$BRIDGE_PROXY"
    return
  fi
  for var in PROXY_URL HTTPS_PROXY HTTP_PROXY ALL_PROXY https_proxy http_proxy all_proxy; do
    val="${!var-}"
    if [ -n "$val" ]; then
      echo "$val"
      return
    fi
  done
}

PROXY_RESOLVED="$(resolve_bridge_proxy)"
if [ -n "$PROXY_RESOLVED" ]; then
  export PROXY_URL="$PROXY_RESOLVED"
  export HTTPS_PROXY="$PROXY_RESOLVED"
  export HTTP_PROXY="$PROXY_RESOLVED"
  export ALL_PROXY="$PROXY_RESOLVED"
  echo "==> Bridge proxy enabled: $PROXY_RESOLVED"
fi

echo "==> Building nanobot"
make build

echo "==> Building web UI"
make webui-install
make webui-build

echo "==> Preparing WhatsApp bridge"
make bridge-install
make bridge-build

bridge_port_pids() {
  local port="$1"
  if command -v lsof >/dev/null 2>&1; then
    lsof -tiTCP:"$port" -sTCP:LISTEN 2>/dev/null || true
  elif command -v ss >/dev/null 2>&1; then
    ss -ltnp "sport = :$port" 2>/dev/null | awk 'NR>1 {print $NF}' | sed 's/.*pid=//;s/,.*//' | sort -u
  elif command -v netstat >/dev/null 2>&1; then
    netstat -anp 2>/dev/null | awk -v p=":$port" '$0 ~ p && $0 ~ /LISTEN/ {print $NF}' | sort -u
  else
    return 1
  fi
}

should_kill_pid() {
  local pid="$1"
  local cmd
  cmd="$(ps -p "$pid" -o command= 2>/dev/null || true)"
  if [ -z "$cmd" ]; then
    return 1
  fi
  if [ -n "$FORCE_BRIDGE_KILL" ]; then
    return 0
  fi
  case "$cmd" in
    *"/bridge/dist/index.js"*) return 0 ;;
    *"nanobot-whatsapp-bridge"*) return 0 ;;
    *"node dist/index.js"*"/bridge"*) return 0 ;;
  esac
  return 1
}

stop_existing_bridge() {
  local pids
  pids="$(bridge_port_pids "$BRIDGE_PORT" || true)"
  if [ -z "$pids" ]; then
    return 0
  fi
  for pid in $pids; do
    if should_kill_pid "$pid"; then
      echo "==> Stopping existing bridge on port $BRIDGE_PORT (PID $pid)"
      kill "$pid" >/dev/null 2>&1 || true
    fi
  done
  sleep 0.3
  pids="$(bridge_port_pids "$BRIDGE_PORT" || true)"
  if [ -n "$pids" ]; then
    echo "Error: port $BRIDGE_PORT is still in use (PID(s) $pids)."
    echo "Set BRIDGE_PORT to use a different port, or stop the process manually."
    exit 1
  fi
}

BRIDGE_PID=""
stop_existing_bridge
echo "==> Starting WhatsApp bridge on port $BRIDGE_PORT"
make bridge-run BRIDGE_PORT="$BRIDGE_PORT" &
BRIDGE_PID=$!

echo "==> Starting nanobot gateway"
cleanup() {
  if [ -n "$BRIDGE_PID" ] && kill -0 "$BRIDGE_PID" >/dev/null 2>&1; then
    kill "$BRIDGE_PID" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT INT TERM

./build/nanobot-go gateway
