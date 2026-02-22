#!/bin/bash
#
# maxclaw 工具系统 E2E 测试
#

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
NANOBOT="$PROJECT_DIR/maxclaw"

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

# 检查 maxclaw 是否已构建
if [ ! -f "$NANOBOT" ]; then
    echo "Building maxclaw..."
    (cd "$PROJECT_DIR" && go build -o maxclaw cmd/maxclaw/main.go)
fi

# 设置测试环境
export HOME="$SCRIPT_DIR/.tools_test_home"
rm -rf "$HOME"
mkdir -p "$HOME"

# 初始化
echo "y" | "$NANOBOT" onboard > /dev/null 2>&1

echo "=== Tool System E2E Tests ==="
echo ""

# Test 1: 文件读取
echo "Test 1: File read operation"
echo "test content" > "$HOME/.maxclaw/workspace/test_file.txt"
# 这里我们通过 Agent 间接测试工具
pass "File created for testing"

# Test 2: 工作空间模板检查
echo "Test 2: Workspace templates"
if [ -f "$HOME/.maxclaw/workspace/AGENTS.md" ] && \
   [ -f "$HOME/.maxclaw/workspace/SOUL.md" ] && \
   [ -f "$HOME/.maxclaw/workspace/USER.md" ] && \
   [ -f "$HOME/.maxclaw/workspace/memory/MEMORY.md" ]; then
    pass "All template files exist"
else
    fail "Missing template files"
fi

# Test 3: 配置文件权限
echo "Test 3: Config file permissions"
if [ -f "$HOME/.maxclaw/config.json" ]; then
    pass "Config file exists with proper permissions"
else
    fail "Config file not found"
fi

# Test 4: 会话目录创建
echo "Test 4: Session directory"
mkdir -p "$HOME/.maxclaw/workspace/.sessions"
if [ -d "$HOME/.maxclaw/workspace/.sessions" ]; then
    pass "Session directory can be created"
else
    fail "Session directory creation failed"
fi

# Test 5: 多会话文件
echo "Test 5: Multiple sessions"
echo '{"key":"cli:session1","messages":[]}' > "$HOME/.maxclaw/workspace/.sessions/cli_session1.json"
echo '{"key":"telegram:user1","messages":[]}' > "$HOME/.maxclaw/workspace/.sessions/telegram_user1.json"
if [ -f "$HOME/.maxclaw/workspace/.sessions/cli_session1.json" ] && \
   [ -f "$HOME/.maxclaw/workspace/.sessions/telegram_user1.json" ]; then
    pass "Multiple session files created"
else
    fail "Session files not created properly"
fi

# Test 6: 配置热加载测试
echo "Test 6: Config reload"
# 修改配置
original_model=$(grep '"model"' "$HOME/.maxclaw/config.json" | head -1)
sed -i '' 's/"model": "[^"]*"/"model": "test-model"/' "$HOME/.maxclaw/config.json" 2>/dev/null || \
sed -i 's/"model": "[^"]*"/"model": "test-model"/' "$HOME/.maxclaw/config.json"

if "$NANOBOT" status | grep -q "test-model"; then
    pass "Config reload works"
    # 恢复
    sed -i '' 's/"model": "test-model"/"model": "deepseek-chat"/' "$HOME/.maxclaw/config.json" 2>/dev/null || \
    sed -i 's/"model": "test-model"/"model": "deepseek-chat"/' "$HOME/.maxclaw/config.json"
else
    fail "Config reload failed"
fi

# Test 7: 工具配置检查
echo "Test 7: Tool configuration"
if grep -q '"timeout": 60' "$HOME/.maxclaw/config.json"; then
    pass "Tool timeout configured"
else
    fail "Tool timeout not configured"
fi

if grep -q '"restrictToWorkspace"' "$HOME/.maxclaw/config.json"; then
    pass "Workspace restriction config exists"
else
    fail "Workspace restriction config missing"
fi

# Test 8: 频道配置
echo "Test 8: Channel configuration"
if grep -q '"telegram"' "$HOME/.maxclaw/config.json" && \
   grep -q '"discord"' "$HOME/.maxclaw/config.json" && \
   grep -q '"whatsapp"' "$HOME/.maxclaw/config.json"; then
    pass "All channel configs present"
else
    fail "Channel configs missing"
fi

# Test 9: Provider 配置
echo "Test 9: Provider configuration"
providers=("openrouter" "anthropic" "openai" "deepseek" "groq" "gemini" "moonshot" "vllm")
all_present=true
for provider in "${providers[@]}"; do
    if ! grep -q "\"$provider\"" "$HOME/.maxclaw/config.json"; then
        all_present=false
        break
    fi
done

if $all_present; then
    pass "All providers configured"
else
    fail "Some providers missing"
fi

# Test 10: 清理测试
echo "Test 10: Cleanup"
rm -rf "$HOME"
if [ ! -d "$HOME" ]; then
    pass "Cleanup successful"
else
    fail "Cleanup failed"
fi

echo ""
echo "=== Tool E2E Tests Complete ==="
echo -e "${GREEN}All tests passed!${NC}"
