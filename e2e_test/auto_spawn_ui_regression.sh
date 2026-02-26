#!/usr/bin/env bash
#
# maxclaw UI 回归脚本：executionMode=auto + spawn + monorepo context 发现
# 用途：快速准备可回归环境，并输出可直接在 UI 里执行的一组验收 Prompt。
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
TEST_HOME="$SCRIPT_DIR/.auto_spawn_ui_home"
PORT="${PORT:-18890}"
SETUP_ONLY=0
KEEP_HOME=0

usage() {
	cat <<EOF
Usage: $(basename "$0") [--port <port>] [--setup-only] [--keep-home]

Options:
  --port <port>   Gateway port (default: 18890)
  --setup-only    不要求真实 API key，使用占位 key 仅做环境准备/连通性验证
  --keep-home     结束时保留测试 HOME（默认会清理）
EOF
}

while [[ $# -gt 0 ]]; do
	case "$1" in
	--port)
		PORT="${2:-}"
		shift 2
		;;
	--setup-only)
		SETUP_ONLY=1
		shift
		;;
	--keep-home)
		KEEP_HOME=1
		shift
		;;
	-h | --help)
		usage
		exit 0
		;;
	*)
		echo -e "${RED}Unknown argument:${NC} $1"
		usage
		exit 1
		;;
	esac
done

if ! [[ "$PORT" =~ ^[0-9]+$ ]]; then
	echo -e "${RED}Invalid port:${NC} $PORT"
	exit 1
fi

pass() {
	echo -e "${GREEN}PASS${NC}: $1"
}

warn() {
	echo -e "${YELLOW}WARN${NC}: $1"
}

info() {
	echo -e "${BLUE}INFO${NC}: $1"
}

cleanup() {
	if [[ -n "${GATEWAY_PID:-}" ]] && kill -0 "$GATEWAY_PID" >/dev/null 2>&1; then
		kill "$GATEWAY_PID" >/dev/null 2>&1 || true
		wait "$GATEWAY_PID" 2>/dev/null || true
	fi
	if [[ "$KEEP_HOME" -ne 1 ]]; then
		rm -rf "$TEST_HOME"
	fi
}

trap cleanup EXIT

mkdir -p "$BUILD_DIR"
mkdir -p "$TEST_HOME/.maxclaw/workspace"

info "Building maxclaw binary"
cd "$PROJECT_DIR"
go build -o "$BUILD_DIR/maxclaw" cmd/maxclaw/main.go
BIN="$BUILD_DIR/maxclaw"
pass "Build complete"

MODEL=""
PROVIDER=""
API_KEY=""
if [[ -n "${DEEPSEEK_API_KEY:-}" ]]; then
	MODEL="deepseek-chat"
	PROVIDER="deepseek"
	API_KEY="$DEEPSEEK_API_KEY"
elif [[ -n "${OPENROUTER_API_KEY:-}" ]]; then
	MODEL="anthropic/claude-opus-4-5"
	PROVIDER="openrouter"
	API_KEY="$OPENROUTER_API_KEY"
else
	if [[ "$SETUP_ONLY" -eq 1 ]]; then
		MODEL="deepseek-chat"
		PROVIDER="deepseek"
		API_KEY="sk-test-key"
		warn "No API key found. Running setup-only mode with placeholder key."
	else
		echo -e "${RED}Missing API key.${NC} Export DEEPSEEK_API_KEY or OPENROUTER_API_KEY, or use --setup-only."
		exit 1
	fi
fi

export HOME="$TEST_HOME"
WORKSPACE="$TEST_HOME/.maxclaw/workspace"
LOG_FILE="$TEST_HOME/gateway.log"

mkdir -p "$WORKSPACE/apps/api" "$WORKSPACE/packages/web"

cat >"$WORKSPACE/AGENTS.md" <<'EOF'
# Root AGENTS

- ROOT_RULE_TOKEN: root-context-visible
- 如果涉及 monorepo 子模块，优先读取最近的 AGENTS.md / CLAUDE.md。
EOF

cat >"$WORKSPACE/apps/api/AGENTS.md" <<'EOF'
# API Module AGENTS

- API_RULE_TOKEN: api-context-visible
- 所有 API 变更需要考虑向后兼容与错误码稳定性。
EOF

cat >"$WORKSPACE/packages/web/CLAUDE.md" <<'EOF'
# Web Module CLAUDE

- WEB_RULE_TOKEN: web-context-visible
- UI 变更优先保证可读性，再考虑微交互。
EOF

cat >"$TEST_HOME/.maxclaw/config.json" <<EOF
{
  "agents": {
    "defaults": {
      "workspace": "$WORKSPACE",
      "model": "$MODEL",
      "maxTokens": 4096,
      "temperature": 0.4,
      "maxToolIterations": 80,
      "executionMode": "auto"
    }
  },
  "channels": {
    "telegram": { "enabled": false, "token": "", "allowFrom": [] },
    "discord": { "enabled": false, "token": "", "allowFrom": [] },
    "whatsapp": { "enabled": false, "bridgeUrl": "ws://localhost:3001", "allowFrom": [] }
  },
  "providers": {
    "$PROVIDER": {
      "apiKey": "$API_KEY"
    }
  },
  "gateway": { "host": "0.0.0.0", "port": $PORT },
  "tools": {
    "web": { "search": { "apiKey": "", "maxResults": 5 } },
    "exec": { "timeout": 60 },
    "restrictToWorkspace": false
  }
}
EOF
pass "Prepared isolated HOME at $TEST_HOME"

info "Starting gateway on :$PORT"
"$BIN" gateway -p "$PORT" >"$LOG_FILE" 2>&1 &
GATEWAY_PID=$!

for _ in $(seq 1 30); do
	if curl -fsS "http://127.0.0.1:$PORT/api/status" >/dev/null 2>&1; then
		break
	fi
	sleep 1
done

STATUS_JSON="$(curl -fsS "http://127.0.0.1:$PORT/api/status")"
python3 - "$STATUS_JSON" <<'PY'
import json
import sys
payload = json.loads(sys.argv[1])
if payload.get("executionMode") != "auto":
    raise SystemExit("executionMode is not auto")
PY
pass "Gateway is healthy and executionMode=auto"

PARENT_SESSION="desktop:auto-spawn-regression"
CHILD_SESSION="desktop:auto-spawn-regression:child"
PARENT_FILE="$WORKSPACE/.sessions/desktop_auto-spawn-regression.json"
CHILD_FILE="$WORKSPACE/.sessions/desktop_auto-spawn-regression_child.json"

cat <<EOF

============================================================
UI Regression Ready
============================================================
Gateway:
  http://127.0.0.1:$PORT

Suggested UI path:
  1) Open Electron app and ensure gateway points to :$PORT.
  2) In Settings, verify "Execution Mode" is "auto".
  3) Open a chat session and set session key to: $PARENT_SESSION
  4) Run the prompts below in order.

Prompt A (project context discovery):
请只返回你在系统提示里看到的 "Project Context Files" 列表（每行一个路径，不要解释）。

Expected contains:
  - AGENTS.md
  - apps/api/AGENTS.md
  - packages/web/CLAUDE.md

Prompt B (spawn with explicit params):
请调用 spawn 工具，参数必须严格使用下面 JSON（不要改 key）：
{
  "task": "读取 $WORKSPACE/apps/api/AGENTS.md 与 $WORKSPACE/packages/web/CLAUDE.md，提炼两条规则并输出 JSON。",
  "label": "auto-spawn-regression",
  "model": "$MODEL",
  "selected_skills": [],
  "enabled_sources": ["workspace"],
  "session_key": "$CHILD_SESSION",
  "notify_parent": true
}
调用后等待子任务完成，再给我三行总结。

Post-check commands:
  curl -fsS http://127.0.0.1:$PORT/api/status
  rg -n "\\[Spawn\\] (Started|Completed)" "$PARENT_FILE"
  test -f "$CHILD_FILE" && echo "child session file exists"

Gateway log:
  $LOG_FILE

EOF

if [[ "$SETUP_ONLY" -eq 1 ]]; then
	warn "Setup-only mode enabled: LLM response correctness was not exercised."
fi

pass "Manual regression environment is ready"
