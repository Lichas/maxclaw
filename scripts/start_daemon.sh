#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PID_DIR="$ROOT_DIR/.pids"
LOG_DIR="${LOG_DIR:-$HOME/.maxclaw/logs}"
BRIDGE_PORT="${BRIDGE_PORT:-3001}"
GATEWAY_PORT="${GATEWAY_PORT:-18890}"
BRIDGE_PROXY="${BRIDGE_PROXY:-}"
FORCE_BRIDGE_KILL="${FORCE_BRIDGE_KILL:-}"
FORCE_GATEWAY_KILL="${FORCE_GATEWAY_KILL:-}"

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

read_pid_file() {
  local file="$1"
  if [ ! -f "$file" ]; then
    return 1
  fi
  local pid
  pid="$(cat "$file" 2>/dev/null || true)"
  if ! [[ "$pid" =~ ^[0-9]+$ ]]; then
    return 1
  fi
  printf "%s" "$pid"
}

is_pid_running() {
  local pid="$1"
  kill -0 "$pid" >/dev/null 2>&1
}

pid_listening_on_port() {
  local pid="$1"
  local port="$2"
  local pids
  pids="$(bridge_port_pids "$port" || true)"
  for p in $pids; do
    if [ "$p" = "$pid" ]; then
      return 0
    fi
  done
  return 1
}

wait_service_ready() {
  local name="$1"
  local pid="$2"
  local port="$3"
  local log_file="$4"

  local tries=40
  while [ "$tries" -gt 0 ]; do
    if is_pid_running "$pid" && pid_listening_on_port "$pid" "$port"; then
      return 0
    fi
    sleep 0.25
    tries=$((tries - 1))
  done

  echo "Error: $name failed to start (pid=$pid port=$port)" >&2
  if [ -f "$log_file" ]; then
    echo "--- $name log tail ---" >&2
    tail -n 80 "$log_file" >&2 || true
  fi
  return 1
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
    *"maxclaw-whatsapp-bridge"*) return 0 ;;
    *"node dist/index.js"*"/bridge"*) return 0 ;;
  esac
  return 1
}

should_kill_gateway_pid() {
  local pid="$1"
  local cmd
  cmd="$(ps -p "$pid" -o command= 2>/dev/null || true)"
  if [ -z "$cmd" ]; then
    return 1
  fi
  if [ -n "$FORCE_GATEWAY_KILL" ]; then
    return 0
  fi
  case "$cmd" in
    *"/maxclaw gateway"*) return 0 ;;
    *"/build/maxclaw gateway"*) return 0 ;;
    *"maxclaw gateway -p"*) return 0 ;;
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

stop_existing_gateway() {
  local pids
  pids="$(bridge_port_pids "$GATEWAY_PORT" || true)"
  if [ -z "$pids" ]; then
    return 0
  fi
  for pid in $pids; do
    if should_kill_gateway_pid "$pid"; then
      echo "Stopping existing gateway on port $GATEWAY_PORT (PID $pid)"
      kill "$pid" >/dev/null 2>&1 || true
    fi
  done
  sleep 0.3
  pids="$(bridge_port_pids "$GATEWAY_PORT" || true)"
  if [ -n "$pids" ]; then
    echo "Error: port $GATEWAY_PORT is still in use (PID(s) $pids)."
    echo "Set GATEWAY_PORT to use a different port, or stop the process manually."
    exit 1
  fi
}

bridge_pid="$(read_pid_file "$PID_DIR/bridge.pid" || true)"
if [ -n "$bridge_pid" ] && is_pid_running "$bridge_pid"; then
  if [ -n "$FORCE_BRIDGE_KILL" ]; then
    echo "Stopping existing bridge from PID file (PID $bridge_pid)"
    kill "$bridge_pid" >/dev/null 2>&1 || true
  else
    echo "Bridge already running (PID $bridge_pid)."
  fi
fi

bridge_pid="$(read_pid_file "$PID_DIR/bridge.pid" || true)"
if [ -z "$bridge_pid" ] || ! is_pid_running "$bridge_pid"; then
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
  wait_service_ready "bridge" "$(cat "$PID_DIR/bridge.pid")" "$BRIDGE_PORT" "$LOG_DIR/bridge.log"
fi

gateway_pid="$(read_pid_file "$PID_DIR/gateway.pid" || true)"
if [ -n "$gateway_pid" ] && is_pid_running "$gateway_pid"; then
  if [ -n "$FORCE_GATEWAY_KILL" ]; then
    echo "Stopping existing gateway from PID file (PID $gateway_pid)"
    kill "$gateway_pid" >/dev/null 2>&1 || true
  else
    echo "Gateway already running (PID $gateway_pid)."
  fi
fi

gateway_pid="$(read_pid_file "$PID_DIR/gateway.pid" || true)"
if [ -z "$gateway_pid" ] || ! is_pid_running "$gateway_pid"; then
  stop_existing_gateway
  echo "==> Building maxclaw"
  make build >/dev/null

  echo "==> Building web UI"
  make webui-install >/dev/null
  make webui-build >/dev/null

  echo "==> Starting gateway on port $GATEWAY_PORT"
  nohup env GATEWAY_PORT="$GATEWAY_PORT" \
    PROXY_URL="${PROXY_URL:-}" \
    HTTPS_PROXY="${HTTPS_PROXY:-}" \
    HTTP_PROXY="${HTTP_PROXY:-}" \
    ALL_PROXY="${ALL_PROXY:-}" \
    https_proxy="${https_proxy:-}" \
    http_proxy="${http_proxy:-}" \
    all_proxy="${all_proxy:-}" \
    "$ROOT_DIR/build/maxclaw" gateway -p "$GATEWAY_PORT" > "$LOG_DIR/gateway.log" 2>&1 &
  echo $! > "$PID_DIR/gateway.pid"
  echo "Gateway PID: $(cat "$PID_DIR/gateway.pid")"
  wait_service_ready "gateway" "$(cat "$PID_DIR/gateway.pid")" "$GATEWAY_PORT" "$LOG_DIR/gateway.log"
fi

printf "\nLogs:\n  %s\n  %s\n" "$LOG_DIR/bridge.log" "$LOG_DIR/gateway.log"
