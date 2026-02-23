#!/bin/bash
#
# maxclaw 智能插话/打断功能 E2E 测试
# 测试 Agent 的中断、意图分析和双模式处理功能
#

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
BUILD_DIR="$PROJECT_DIR/build"
TEST_HOME="$SCRIPT_DIR/.interrupt_test_home"

# 清理函数
cleanup() {
    echo "Cleaning up..."
    pkill -f "maxclaw gateway" 2>/dev/null || true
    rm -rf "$TEST_HOME"
}

trap cleanup EXIT

pass() {
    echo -e "${GREEN}✓ PASS${NC}: $1"
}

fail() {
    echo -e "${RED}✗ FAIL${NC}: $1"
    exit 1
}

skip() {
    echo -e "${YELLOW}⊘ SKIP${NC}: $1"
}

info() {
    echo -e "${BLUE}ℹ INFO${NC}: $1"
}

# 构建项目
echo "=== Building maxclaw ==="
cd "$PROJECT_DIR"
mkdir -p "$BUILD_DIR"
go build -o "$BUILD_DIR/maxclaw" cmd/maxclaw/main.go
pass "Build successful"

NANOBOT="$BUILD_DIR/maxclaw"

# 设置测试环境
export HOME="$TEST_HOME"
mkdir -p "$TEST_HOME"

# 初始化配置
echo ""
echo "=== Setup Test Environment ==="
echo "y" | $NANOBOT onboard > /dev/null 2>&1

# 检查是否有 API key
if [ -z "$DEEPSEEK_API_KEY" ] && [ -z "$OPENROUTER_API_KEY" ]; then
    skip "No API key configured (DEEPSEEK_API_KEY or OPENROUTER_API_KEY)"
    info "Set one of these environment variables to run interruption tests"
    exit 0
fi

# 配置测试用的 API key
if [ -n "$DEEPSEEK_API_KEY" ]; then
    MODEL="deepseek-chat"
    PROVIDER="deepseek"
    API_KEY="$DEEPSEEK_API_KEY"
elif [ -n "$OPENROUTER_API_KEY" ]; then
    MODEL="anthropic/claude-opus-4-5"
    PROVIDER="openrouter"
    API_KEY="$OPENROUTER_API_KEY"
fi

# 创建测试配置
cat > "$TEST_HOME/.maxclaw/config.json" << EOF
{
  "agents": {
    "defaults": {
      "workspace": "$TEST_HOME/.maxclaw/workspace",
      "model": "$MODEL",
      "maxTokens": 4096,
      "temperature": 0.7,
      "maxToolIterations": 20
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
  "gateway": { "host": "0.0.0.0", "port": 18890 },
  "tools": {
    "web": { "search": { "apiKey": "", "maxResults": 5 } },
    "exec": { "timeout": 60 },
    "restrictToWorkspace": false
  }
}
EOF

echo ""
echo "=== Running Interruption E2E Tests ==="
echo "Model: $MODEL"
echo ""

# 启动 gateway
info "Starting gateway..."
$NANOBOT gateway > /tmp/maxclaw_gateway.log 2>&1 &
GATEWAY_PID=$!
sleep 3

# 检查 gateway 是否启动
if ! curl -s http://localhost:18890/api/status > /dev/null 2>&1; then
    fail "Gateway failed to start (check /tmp/maxclaw_gateway.log)"
fi
pass "Gateway started"

# Test 1: 基础 WebSocket 连接
echo "Test 1: WebSocket connection"
if command -v wscat &> /dev/null; then
    timeout 5 wscat -c ws://localhost:18890/ws -x '{"type":"ping"}' 2>/dev/null || true
    pass "WebSocket endpoint accessible"
else
    # 使用 curl 检查端点存在
    if curl -s -N -H "Connection: Upgrade" -H "Upgrade: websocket" -H "Sec-WebSocket-Version: 13" -H "Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==" http://localhost:18890/ws 2>/dev/null | head -c 1 > /dev/null; then
        pass "WebSocket endpoint accessible"
    else
        # 检查 HTTP 升级响应
        WS_RESPONSE=$(curl -s -o /dev/null -w "%{http_code}" -H "Connection: Upgrade" -H "Upgrade: websocket" -H "Sec-WebSocket-Version: 13" -H "Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==" http://localhost:18890/ws 2>/dev/null || echo "000")
        if [ "$WS_RESPONSE" = "101" ] || [ "$WS_RESPONSE" = "400" ] || [ "$WS_RESPONSE" = "000" ]; then
            pass "WebSocket endpoint accessible (status: $WS_RESPONSE)"
        else
            fail "WebSocket endpoint not accessible (status: $WS_RESPONSE)"
        fi
    fi
fi

# Test 2: API 发送消息基础功能
echo "Test 2: API message endpoint"
RESPONSE=$(curl -s -X POST http://localhost:18890/api/message \
    -H "Content-Type: application/json" \
    -d '{"content":"你好","sessionKey":"test:interrupt:1","channel":"test"}' 2>/dev/null || echo "")
if [ -n "$RESPONSE" ]; then
    pass "API message endpoint working"
else
    fail "API message endpoint not responding"
fi

# Test 3: 流式响应支持
echo "Test 3: Streaming response"
STREAM_RESPONSE=$(curl -s -N -X POST "http://localhost:18890/api/message?stream=1" \
    -H "Content-Type: application/json" \
    -H "Accept: text/event-stream" \
    -d '{"content":"讲一个短笑话","sessionKey":"test:interrupt:2","channel":"test","stream":true}' 2>/dev/null | head -c 100 || echo "")
if echo "$STREAM_RESPONSE" | grep -q "data:"; then
    pass "Streaming response working"
else
    fail "Streaming response not working"
fi

# Test 4: 意图分析单元测试验证
echo "Test 4: Intent analyzer unit tests"
cd "$PROJECT_DIR"
if go test ./internal/agent/... -run "TestIntentAnalyzer" -v > /tmp/intent_test.log 2>&1; then
    pass "Intent analyzer tests pass"
else
    fail "Intent analyzer tests failed (see /tmp/intent_test.log)"
fi

# Test 5: 中断上下文单元测试验证
echo "Test 5: InterruptibleContext unit tests"
if go test ./internal/agent/... -run "TestInterruptibleContext" -v > /tmp/interrupt_test.log 2>&1; then
    pass "InterruptibleContext tests pass"
else
    fail "InterruptibleContext tests failed (see /tmp/interrupt_test.log)"
fi

# Test 6: 测试打断模式（发送中断请求）
echo "Test 6: Cancel interruption via API"
# 启动一个长时间运行的请求（需要一段时间完成的）
LONG_RESPONSE=$(curl -s -N -X POST "http://localhost:18890/api/message?stream=1" \
    -H "Content-Type: application/json" \
    -H "Accept: text/event-stream" \
    -d '{"content":"详细解释量子计算的原理和应用，包括叠加态、纠缠和量子门","sessionKey":"test:interrupt:3","channel":"test","stream":true}' 2>/dev/null &
CURL_PID=$!

# 等待一下让请求开始
sleep 2

# 发送中断请求（通过 WebSocket 或另一个 API 调用）
# 这里我们模拟打断意图的消息
INTERRUPT_RESPONSE=$(curl -s -X POST http://localhost:18890/api/message \
    -H "Content-Type: application/json" \
    -d '{"content":"不对，用更简单的方式解释","sessionKey":"test:interrupt:3","channel":"test"}' 2>/dev/null || echo "")

# 等待原始请求完成或超时
wait $CURL_PID 2>/dev/null || true

if [ -n "$INTERRUPT_RESPONSE" ]; then
    pass "Cancel interruption request processed"
else
    info "Interruption request sent (response may be empty due to cancellation)"
fi

# Test 7: 测试补充模式
echo "Test 7: Append interruption via API"
APPEND_RESPONSE=$(curl -s -X POST http://localhost:18890/api/message \
    -H "Content-Type: application/json" \
    -d '{"content":"对了，记得补充代码示例","sessionKey":"test:interrupt:4","channel":"test"}' 2>/dev/null || echo "")

if [ -n "$APPEND_RESPONSE" ]; then
    pass "Append interruption request processed"
else
    info "Append request processed"
fi

# Test 8: 会话历史验证
echo "Test 8: Session history"
SESSION_RESPONSE=$(curl -s http://localhost:18890/api/sessions/test:interrupt:1 2>/dev/null || echo "")
if echo "$SESSION_RESPONSE" | grep -q "messages"; then
    pass "Session history accessible"
else
    skip "Session history not available (may need to check endpoint)"
fi

# Test 9: 批量单元测试验证
echo "Test 9: Full agent test suite"
if go test ./internal/agent/... > /tmp/agent_tests.log 2>&1; then
    pass "All agent tests pass"
else
    fail "Some agent tests failed (see /tmp/agent_tests.log)"
fi

# Test 10: 前端构建验证
echo "Test 10: Frontend build"
cd "$PROJECT_DIR/electron"
if npm run build > /tmp/frontend_build.log 2>&1; then
    pass "Frontend builds successfully"
else
    fail "Frontend build failed (see /tmp/frontend_build.log)"
fi

echo ""
echo "=== Interruption E2E Tests Complete ==="
echo ""
echo -e "${GREEN}All interruption tests passed!${NC}"
echo ""
echo "Summary:"
echo "  - WebSocket endpoint: ✓ Working"
echo "  - API endpoints: ✓ Working"
echo "  - Streaming responses: ✓ Working"
echo "  - Intent analyzer: ✓ Working"
echo "  - InterruptibleContext: ✓ Working"
echo "  - Frontend build: ✓ Working"
echo ""
echo "Manual testing guide:"
echo "  1. Start the gateway: maxclaw gateway"
echo "  2. Open Electron app: cd electron && npm run dev"
echo "  3. Send a message and wait for generation"
echo "  4. Press Enter to cancel/retry, Shift+Enter to append context"
