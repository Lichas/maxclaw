#!/bin/bash
#
# Regression test: interrupt must target the selected/running session only.
#

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
BUILD_DIR="$PROJECT_DIR/build"
TEST_HOME=""
PROVIDER_LOG="$TEST_HOME/fake_provider.log"
GATEWAY_LOG="$TEST_HOME/gateway.log"
PROVIDER_PID=""
GATEWAY_PID=""

export NO_PROXY="127.0.0.1,localhost"
export no_proxy="$NO_PROXY"

pass() {
    echo -e "${GREEN}✓ PASS${NC}: $1"
}

fail() {
    echo -e "${RED}✗ FAIL${NC}: $1"
    if [ -f "$PROVIDER_LOG" ]; then
        echo "----- fake provider log -----"
        cat "$PROVIDER_LOG"
    fi
    if [ -f "$GATEWAY_LOG" ]; then
        echo "----- gateway log -----"
        cat "$GATEWAY_LOG"
    fi
    exit 1
}

info() {
    echo -e "${BLUE}ℹ INFO${NC}: $1"
}

make_temp_home() {
    mktemp -d "${TMPDIR:-/tmp}/maxclaw-concurrent-interrupt-e2e.XXXXXX"
}

cleanup() {
    if [ -n "$GATEWAY_PID" ] && kill -0 "$GATEWAY_PID" 2>/dev/null; then
        kill "$GATEWAY_PID" 2>/dev/null || true
        wait "$GATEWAY_PID" 2>/dev/null || true
    fi
    if [ -n "$PROVIDER_PID" ] && kill -0 "$PROVIDER_PID" 2>/dev/null; then
        kill "$PROVIDER_PID" 2>/dev/null || true
        wait "$PROVIDER_PID" 2>/dev/null || true
    fi
    if [ -n "$TEST_HOME" ] && [ -d "$TEST_HOME" ]; then
        rm -rf "$TEST_HOME"
    fi
}

trap cleanup EXIT

find_free_port() {
    python3 - <<'PY'
import socket
s = socket.socket()
s.bind(("127.0.0.1", 0))
print(s.getsockname()[1])
s.close()
PY
}

wait_for_url() {
    local url="$1"
    local label="$2"
    local attempts="${3:-60}"
    local delay="${4:-0.25}"

    for _ in $(seq 1 "$attempts"); do
        if curl --noproxy '*' -fsS "$url" >/dev/null 2>&1; then
            return 0
        fi
        sleep "$delay"
    done
    fail "Timed out waiting for $label at $url"
}

stream_message() {
    local session_key="$1"
    local content="$2"
    local out_file="$3"
    local payload
    payload="$(python3 - "$session_key" "$content" <<'PY'
import json
import sys
print(json.dumps({
    "sessionKey": sys.argv[1],
    "channel": "desktop",
    "chatId": sys.argv[1],
    "content": sys.argv[2],
    "stream": True,
}))
PY
)"
    curl --noproxy '*' -fsS -N \
        -H 'Content-Type: application/json' \
        -H 'Accept: text/event-stream, application/json' \
        -d "$payload" \
        "http://127.0.0.1:${GATEWAY_PORT}/api/message?stream=1" >"$out_file"
}

send_interrupt() {
    local session_key="$1"
    local mode="$2"
    local payload
    payload="$(python3 - "$session_key" "$mode" <<'PY'
import json
import sys
print(json.dumps({
    "type": "interrupt",
    "session": sys.argv[1],
    "mode": sys.argv[2],
    "content": "",
}))
PY
)"
    node -e '
const port = process.argv[1];
const payload = process.argv[2];
const ws = new WebSocket(`ws://127.0.0.1:${port}/ws`);
ws.addEventListener("open", () => {
  ws.send(payload);
  ws.close();
});
ws.addEventListener("error", (err) => {
  console.error(err);
  process.exit(1);
});
' "$GATEWAY_PORT" "$payload"
}

echo "=== Concurrent Interrupt Regression E2E ==="

TEST_HOME="$(make_temp_home)"
PROVIDER_LOG="$TEST_HOME/fake_provider.log"
GATEWAY_LOG="$TEST_HOME/gateway.log"
mkdir -p "$BUILD_DIR" "$TEST_HOME"

if [ ! -x "$BUILD_DIR/maxclaw" ]; then
    info "Building maxclaw CLI"
    (cd "$PROJECT_DIR" && go build -o "$BUILD_DIR/maxclaw" cmd/maxclaw/main.go)
fi
if [ ! -x "$BUILD_DIR/maxclaw-gateway" ]; then
    info "Building maxclaw gateway"
    (cd "$PROJECT_DIR" && go build -o "$BUILD_DIR/maxclaw-gateway" cmd/maxclaw-gateway/main.go)
fi

export HOME="$TEST_HOME"
mkdir -p "$TEST_HOME/.maxclaw"
echo "y" | "$BUILD_DIR/maxclaw" onboard >/dev/null 2>&1

PROVIDER_PORT="$(find_free_port)"
GATEWAY_PORT="$(find_free_port)"

cat > "$TEST_HOME/.maxclaw/config.json" <<EOF
{
  "agents": {
    "defaults": {
      "workspace": "$TEST_HOME/.maxclaw/workspace",
      "model": "openai/gpt-5.1",
      "maxTokens": 256,
      "temperature": 0,
      "maxToolIterations": 6,
      "executionMode": "ask"
    }
  },
  "channels": {
    "telegram": { "enabled": false, "token": "", "allowFrom": [] },
    "discord": { "enabled": false, "token": "", "allowFrom": [] },
    "whatsapp": { "enabled": false, "bridgeUrl": "ws://localhost:3001", "allowFrom": [] }
  },
  "providers": {
    "openai": {
      "apiKey": "sk-e2e",
      "apiBase": "http://127.0.0.1:$PROVIDER_PORT/v1",
      "apiFormat": "openai"
    }
  },
  "gateway": { "host": "127.0.0.1", "port": $GATEWAY_PORT },
  "tools": {
    "web": { "search": { "apiKey": "", "maxResults": 5 } },
    "exec": { "timeout": 30 },
    "restrictToWorkspace": true
  }
}
EOF

info "Starting fake OpenAI provider on :$PROVIDER_PORT"
python3 "$SCRIPT_DIR/fake_openai_server.py" --port "$PROVIDER_PORT" >"$PROVIDER_LOG" 2>&1 &
PROVIDER_PID=$!
wait_for_url "http://127.0.0.1:$PROVIDER_PORT/healthz" "fake provider"

info "Starting gateway on :$GATEWAY_PORT"
"$BUILD_DIR/maxclaw-gateway" maxclaw-gateway -p "$GATEWAY_PORT" >"$GATEWAY_LOG" 2>&1 &
GATEWAY_PID=$!
wait_for_url "http://127.0.0.1:$GATEWAY_PORT/api/status" "gateway"

echo "Test 1: interrupt should only cancel the targeted running session"
A_OUT="$TEST_HOME/session_a.sse"
B_OUT="$TEST_HOME/session_b.sse"

stream_message "desktop:e2e-concurrent-a" "Reply with exactly SLOW_A after waiting." "$A_OUT" &
PID_A=$!
stream_message "desktop:e2e-concurrent-b" "Reply with exactly SLOW_B after waiting." "$B_OUT" &
PID_B=$!

sleep 0.3
send_interrupt "desktop:e2e-concurrent-b" "cancel"

wait "$PID_A"
wait "$PID_B"

if ! grep -q 'SLOW_A' "$A_OUT"; then
    fail "Expected untargeted session A to complete normally"
fi
if grep -q 'SLOW_B' "$B_OUT"; then
    fail "Expected targeted session B to be interrupted before completion"
fi
if ! grep -q '"type":"error"' "$B_OUT"; then
    fail "Expected interrupted session B to emit an error event"
fi
pass "Interrupt only affected the targeted running session"

echo ""
echo "=== Concurrent Interrupt Regression Complete ==="
echo -e "${GREEN}All tests passed!${NC}"
