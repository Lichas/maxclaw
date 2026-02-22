#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PID_DIR="$ROOT_DIR/.pids"
BRIDGE_PORT="${BRIDGE_PORT:-3001}"
GATEWAY_PORT="${GATEWAY_PORT:-18890}"

stop_pid() {
  local name="$1"
  local pid_file="$PID_DIR/$name.pid"
  if [ -f "$pid_file" ]; then
    local pid
    pid="$(cat "$pid_file")"
    if kill -0 "$pid" >/dev/null 2>&1; then
      echo "Stopping $name (PID $pid)"
      kill "$pid" >/dev/null 2>&1 || true
    else
      echo "$name not running (stale PID $pid)"
    fi
    rm -f "$pid_file"
  else
    echo "$name PID file not found"
  fi
}

port_pids() {
  local port="$1"
  if command -v lsof >/dev/null 2>&1; then
    lsof -tiTCP:"$port" -sTCP:LISTEN 2>/dev/null || true
  elif command -v ss >/dev/null 2>&1; then
    ss -ltnp "sport = :$port" 2>/dev/null | awk 'NR>1 {print $NF}' | sed 's/.*pid=//;s/,.*//' | sort -u
  elif command -v netstat >/dev/null 2>&1; then
    netstat -anp 2>/dev/null | awk -v p=":$port" '$0 ~ p && $0 ~ /LISTEN/ {print $NF}' | sort -u
  fi
}

should_kill_bridge_pid() {
  local pid="$1"
  local cmd
  cmd="$(ps -p "$pid" -o command= 2>/dev/null || true)"
  case "$cmd" in
    *"/bridge/dist/index.js"*) return 0 ;;
    *"maxclaw-whatsapp-bridge"*) return 0 ;;
    *"nanobot-whatsapp-bridge"*) return 0 ;;
    *"node dist/index.js"*"/bridge"*) return 0 ;;
  esac
  return 1
}

should_kill_gateway_pid() {
  local pid="$1"
  local cmd
  cmd="$(ps -p "$pid" -o command= 2>/dev/null || true)"
  case "$cmd" in
    *"/maxclaw gateway"*) return 0 ;;
    *"/build/maxclaw gateway"*) return 0 ;;
    *"maxclaw gateway -p"*) return 0 ;;
    *"/nanobot-go gateway"*) return 0 ;;
    *"/build/nanobot-go gateway"*) return 0 ;;
    *"nanobot-go gateway -p"*) return 0 ;;
    *"/nanobot gateway"*) return 0 ;;
    *"/build/nanobot gateway"*) return 0 ;;
    *"nanobot gateway -p"*) return 0 ;;
  esac
  return 1
}

kill_stale_by_port() {
  local name="$1"
  local port="$2"
  local pids
  pids="$(port_pids "$port" || true)"
  [ -z "$pids" ] && return 0

  for pid in $pids; do
    if [ "$name" = "bridge" ] && should_kill_bridge_pid "$pid"; then
      echo "Stopping stale $name on port $port (PID $pid)"
      kill "$pid" >/dev/null 2>&1 || true
    elif [ "$name" = "gateway" ] && should_kill_gateway_pid "$pid"; then
      echo "Stopping stale $name on port $port (PID $pid)"
      kill "$pid" >/dev/null 2>&1 || true
    fi
  done
}

stop_pid bridge
stop_pid gateway
kill_stale_by_port bridge "$BRIDGE_PORT"
kill_stale_by_port gateway "$GATEWAY_PORT"
