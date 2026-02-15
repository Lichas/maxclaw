#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PID_DIR="$ROOT_DIR/.pids"
LOG_DIR="${LOG_DIR:-$HOME/.nanobot/logs}"
BRIDGE_PORT="${BRIDGE_PORT:-3001}"
GATEWAY_PORT="${GATEWAY_PORT:-18890}"
BRIDGE_PROXY="${BRIDGE_PROXY:-}"
FORCE_BRIDGE_KILL="${FORCE_BRIDGE_KILL:-}"

mkdir -p "$PID_DIR" "$LOG_DIR"

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
  echo "Bridge proxy enabled: $PROXY_RESOLVED"
fi

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
      echo "Stopping existing bridge on port $BRIDGE_PORT (PID $pid)"
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

if [ -f "$PID_DIR/bridge.pid" ] && kill -0 "$(cat "$PID_DIR/bridge.pid")" >/dev/null 2>&1; then
  echo "Bridge already running (PID $(cat "$PID_DIR/bridge.pid"))."
else
  stop_existing_bridge
  echo "==> Starting WhatsApp bridge on port $BRIDGE_PORT"
  make bridge-install >/dev/null
  make bridge-build >/dev/null
  nohup env BRIDGE_PORT="$BRIDGE_PORT" \
    PROXY_URL="${PROXY_URL:-}" \
    HTTPS_PROXY="${HTTPS_PROXY:-}" \
    HTTP_PROXY="${HTTP_PROXY:-}" \
    ALL_PROXY="${ALL_PROXY:-}" \
    node "$ROOT_DIR/bridge/dist/index.js" > "$LOG_DIR/bridge.log" 2>&1 &
  echo $! > "$PID_DIR/bridge.pid"
  echo "Bridge PID: $(cat "$PID_DIR/bridge.pid")"
fi

if [ -f "$PID_DIR/gateway.pid" ] && kill -0 "$(cat "$PID_DIR/gateway.pid")" >/dev/null 2>&1; then
  echo "Gateway already running (PID $(cat "$PID_DIR/gateway.pid"))."
else
  echo "==> Building nanobot"
  make build >/dev/null

  echo "==> Building web UI"
  make webui-install >/dev/null
  make webui-build >/dev/null

  echo "==> Starting gateway on port $GATEWAY_PORT"
  nohup "$ROOT_DIR/build/nanobot-go" gateway -p "$GATEWAY_PORT" > "$LOG_DIR/gateway.log" 2>&1 &
  echo $! > "$PID_DIR/gateway.pid"
  echo "Gateway PID: $(cat "$PID_DIR/gateway.pid")"
fi

printf "\nLogs:\n  %s\n  %s\n" "$LOG_DIR/bridge.log" "$LOG_DIR/gateway.log"
