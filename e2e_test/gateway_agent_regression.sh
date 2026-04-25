#!/bin/bash
#
# Gateway + Agent regression test using a local fake OpenAI server.
#

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
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
    mktemp -d "${TMPDIR:-/tmp}/maxclaw-gateway-agent-e2e.XXXXXX"
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
        if [ -d "$TEST_HOME" ]; then
            echo -e "${RED}✗ FAIL${NC}: failed to clean temporary directory $TEST_HOME" >&2
            exit 1
        fi
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

post_message() {
    local session_key="$1"
    local content="$2"
    local payload
    payload="$(python3 - "$session_key" "$content" <<'PY'
import json
import sys
print(json.dumps({
    "sessionKey": sys.argv[1],
    "channel": "webui",
    "content": sys.argv[2],
}))
PY
)"
    curl --noproxy '*' -fsS \
        -H 'Content-Type: application/json' \
        -d "$payload" \
        "http://127.0.0.1:${GATEWAY_PORT}/api/message"
}

json_field() {
    local field="$1"
    python3 -c '
import json
import sys
payload = json.load(sys.stdin)
value = payload
for part in sys.argv[1].split("."):
    value = value[part]
print(value)
' "$field"
}

session_count() {
    python3 -c '
import json
import sys
payload = json.load(sys.stdin)
sessions = payload.get("sessions", [])
print(len(sessions))
'
}

session_contains() {
    local session_key="$1"
    python3 -c '
import json
import sys
target = sys.argv[1]
payload = json.load(sys.stdin)
sessions = payload.get("sessions", [])
print("yes" if any(item.get("key") == target for item in sessions) else "no")
' "$session_key"
}

echo "=== Gateway Agent Regression E2E ==="

TEST_HOME="$(make_temp_home)"
PROVIDER_LOG="$TEST_HOME/fake_provider.log"
GATEWAY_LOG="$TEST_HOME/gateway.log"

mkdir -p "$BUILD_DIR"
mkdir -p "$TEST_HOME"

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

echo "Test 0: session list only gains the target session after first message"
initial_sessions="$(curl --noproxy '*' -fsS "http://127.0.0.1:${GATEWAY_PORT}/api/sessions")"
initial_count="$(printf '%s' "$initial_sessions" | session_count)"
if [ "$(printf '%s' "$initial_sessions" | session_contains "webui:e2e-session-create")" != "no" ]; then
    fail "Target regression session unexpectedly existed before first message"
fi
response="$(post_message "webui:e2e-session-create" "Reply with exactly E2E_PONG." | json_field response)"
if [ "$response" != "E2E_PONG" ]; then
    fail "Unexpected session creation response: $response"
fi
sessions_after_first_message="$(curl --noproxy '*' -fsS "http://127.0.0.1:${GATEWAY_PORT}/api/sessions")"
if [ "$(printf '%s' "$sessions_after_first_message" | session_contains "webui:e2e-session-create")" != "yes" ]; then
    fail "Expected new session to appear only after first message"
fi
after_count="$(printf '%s' "$sessions_after_first_message" | session_count)"
if [ "$after_count" -lt $((initial_count + 1)) ]; then
    fail "Expected session list count to grow after first message"
fi
pass "Gateway only gains the regression session after the first real message"

echo "Test 1: basic request/response"
response="$(post_message "webui:e2e-basic" "Reply with exactly E2E_PONG." | json_field response)"
if [ "$response" = "E2E_PONG" ]; then
    pass "Gateway returns provider text response"
else
    fail "Unexpected basic response: $response"
fi

echo "Test 2: arithmetic reasoning"
response="$(post_message "webui:e2e-reasoning" "Compute 17 + 28. Reply with only the number." | json_field response)"
if [ "$response" = "45" ]; then
    pass "Gateway handles one-step arithmetic reasoning"
else
    fail "Unexpected arithmetic response: $response"
fi

echo "Test 3: multi-step arithmetic reasoning"
response="$(post_message "webui:e2e-reasoning" "Compute (12 * 3) - 5. Reply with only the number." | json_field response)"
if [ "$response" = "31" ]; then
    pass "Gateway handles multi-step arithmetic reasoning"
else
    fail "Unexpected multi-step arithmetic response: $response"
fi

echo "Test 4: simple logic reasoning"
response="$(post_message "webui:e2e-reasoning" "If all bloops are razzies and all razzies are green, are all bloops green? Reply with only YES or NO." | json_field response)"
if [ "$response" = "YES" ]; then
    pass "Gateway handles simple syllogistic reasoning"
else
    fail "Unexpected logic response: $response"
fi

echo "Test 5: deterministic text counting"
response="$(post_message "webui:e2e-reasoning" 'How many letters are in the word "banana"? Reply with only the number.' | json_field response)"
if [ "$response" = "6" ]; then
    pass "Gateway handles deterministic counting"
else
    fail "Unexpected counting response: $response"
fi

echo "Test 6: write_file tool flow"
response="$(post_message "webui:e2e-file" "Use write_file to create note.txt with content hello-from-e2e. After the tool succeeds, reply with exactly DONE_WRITE." | json_field response)"
if [ "$response" != "DONE_WRITE" ]; then
    fail "Unexpected write_file response: $response"
fi

session_file="$TEST_HOME/.maxclaw/workspace/.sessions/webui_e2e-file/note.txt"
if [ ! -f "$session_file" ]; then
    fail "Expected note file to exist at $session_file"
fi
if [ "$(cat "$session_file")" != "hello-from-e2e" ]; then
    fail "Unexpected note file contents"
fi
pass "Gateway executed write_file and persisted output in session workspace"

echo "Test 7: read_file tool flow"
response="$(post_message "webui:e2e-file" "Use read_file to read note.txt. If the content is hello-from-e2e, reply with exactly READ_OK." | json_field response)"
if [ "$response" = "READ_OK" ]; then
    pass "Gateway executed read_file and returned final answer"
else
    fail "Unexpected read_file response: $response"
fi

echo "Test 8: multi-turn session memory"
response="$(post_message "webui:e2e-memory" "Remember that my codename is ORANGE_E2E." | json_field response)"
if [ "$response" != "ACK_ORANGE" ]; then
    fail "Unexpected memory ack response: $response"
fi
response="$(post_message "webui:e2e-memory" "What is my codename? Reply with exactly ORANGE_E2E." | json_field response)"
if [ "$response" = "ORANGE_E2E" ]; then
    pass "Gateway preserved session context across requests"
else
    fail "Unexpected memory recall response: $response"
fi

echo "Test 9: memory plus arithmetic reasoning"
response="$(post_message "webui:e2e-memory-math" "Remember this: my number is 14. Reply with only OK." | json_field response)"
if [ "$response" != "OK" ]; then
    fail "Unexpected memory math ack response: $response"
fi
response="$(post_message "webui:e2e-memory-math" "Add 6 to my number. Reply with only the number." | json_field response)"
if [ "$response" = "20" ]; then
    pass "Gateway combines session memory with basic arithmetic reasoning"
else
    fail "Unexpected memory math response: $response"
fi

echo ""
echo -e "${GREEN}Gateway agent regression E2E passed.${NC}"
