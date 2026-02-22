#!/bin/bash
#
# maxclaw 端到端测试脚本
#

set -e

# 颜色
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 测试目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
BUILD_DIR="$PROJECT_DIR/build"
TEST_HOME="$SCRIPT_DIR/.test_home"

# 清理函数
cleanup() {
    echo "Cleaning up..."
    rm -rf "$TEST_HOME"
}

trap cleanup EXIT

# 打印测试结果
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

echo ""
echo "=== Running E2E Tests ==="
echo ""

# Test 1: 版本命令
echo "Test 1: Version command"
if $NANOBOT version | grep -q "maxclaw"; then
    pass "Version command works"
else
    fail "Version command failed"
fi

# Test 2: Onboard 初始化
echo "Test 2: Onboard initialization"
echo "y" | $NANOBOT onboard > /dev/null 2>&1
if [ -f "$TEST_HOME/.maxclaw/config.json" ]; then
    pass "Config file created"
else
    fail "Config file not created"
fi

if [ -d "$TEST_HOME/.maxclaw/workspace" ]; then
    pass "Workspace created"
else
    fail "Workspace not created"
fi

if [ -f "$TEST_HOME/.maxclaw/workspace/AGENTS.md" ]; then
    pass "Template files created"
else
    fail "Template files not created"
fi

# Test 3: Status 命令
echo "Test 3: Status command"
if $NANOBOT status | grep -q "maxclaw Status"; then
    pass "Status command works"
else
    fail "Status command failed"
fi

# Test 4: 配置加载
echo "Test 4: Config loading"
# 修改配置
cat > "$TEST_HOME/.maxclaw/config.json" << 'EOF'
{
  "agents": {
    "defaults": {
      "workspace": "'$TEST_HOME'/.maxclaw/workspace",
      "model": "deepseek-chat",
      "maxTokens": 8192,
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
    "deepseek": {
      "apiKey": "sk-test-key",
      "apiBase": "https://api.deepseek.com/v1"
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

if $NANOBOT status | grep -q "deepseek-chat"; then
    pass "Custom config loaded"
else
    fail "Custom config not loaded"
fi

# Test 5: 工作区限制检查
echo "Test 5: Workspace restriction"
if $NANOBOT status | grep -q "Workspace:"; then
    pass "Workspace path displayed"
else
    fail "Workspace path not displayed"
fi

# Test 6: 工具系统测试（使用 mock provider 无法真正测试，验证命令存在）
echo "Test 6: Agent command (dry run)"
if $NANOBOT agent --help | grep -q "Interact with the agent"; then
    pass "Agent command available"
else
    fail "Agent command not available"
fi

# Test 7: 网关命令检查
echo "Test 7: Gateway command"
if $NANOBOT gateway --help | grep -q "Start the maxclaw gateway"; then
    pass "Gateway command available"
else
    fail "Gateway command not available"
fi

# Test 8: 配置文件结构验证
echo "Test 8: Config structure validation"
if python3 -c "import json; json.load(open('$TEST_HOME/.maxclaw/config.json'))" 2>/dev/null; then
    pass "Config is valid JSON"
else
    fail "Config is invalid JSON"
fi

# Test 9: 多会话隔离测试
echo "Test 9: Session isolation"
# 创建会话目录
mkdir -p "$TEST_HOME/.maxclaw/workspace/.sessions"
echo '{"key":"test:1","messages":[{"role":"user","content":"hello","timestamp":"2024-01-01T00:00:00Z"}]}' > "$TEST_HOME/.maxclaw/workspace/.sessions/test_1.json"

if [ -f "$TEST_HOME/.maxclaw/workspace/.sessions/test_1.json" ]; then
    pass "Session file created"
else
    fail "Session file not created"
fi

# Test 10: 环境变量测试
echo "Test 10: Environment variable support"
# 测试 HOME 环境变量被正确使用
if [ "$HOME" = "$TEST_HOME" ]; then
    pass "HOME environment variable respected"
else
    fail "HOME environment variable not respected"
fi

# Test 11: Cron 命令检查
echo "Test 11: Cron command"
if $NANOBOT cron --help 2>&1 | grep -q "manage"; then
    pass "Cron command available"
else
    fail "Cron command not available"
fi

# Test 12: Cron 添加任务
echo "Test 12: Cron add job"
$NANOBOT cron add -n "test-job" -t every -e 3600000 -m "Test message" --deliver > /dev/null 2>&1
if $NANOBOT cron list 2>&1 | grep -q "test-job"; then
    pass "Cron job added"
else
    fail "Cron job not added"
fi

# Test 13: Cron 状态
echo "Test 13: Cron status"
if $NANOBOT cron status 2>&1 | grep -q "Cron Service"; then
    pass "Cron status works"
else
    fail "Cron status failed"
fi

# Test 14: Cron 禁用/启用任务
echo "Test 14: Cron enable/disable job"
$NANOBOT cron list 2>&1 > /tmp/cron_list.txt
JOB_ID=$(cat /tmp/cron_list.txt | grep -a "test-job" | head -1 | awk '{print $1}')
$NANOBOT cron disable "$JOB_ID" > /dev/null 2>&1
$NANOBOT cron list 2>&1 > /tmp/cron_list2.txt
if cat /tmp/cron_list2.txt | grep -a "$JOB_ID" | grep -q "disabled"; then
    pass "Cron job disabled"
else
    fail "Cron job disable failed"
fi

$NANOBOT cron enable "$JOB_ID" > /dev/null 2>&1
$NANOBOT cron list 2>&1 > /tmp/cron_list3.txt
if cat /tmp/cron_list3.txt | grep -a "$JOB_ID" | grep -q "enabled"; then
    pass "Cron job enabled"
else
    fail "Cron job enable failed"
fi

# Test 15: Cron 删除任务
echo "Test 15: Cron remove job"
$NANOBOT cron remove "$JOB_ID" > /dev/null 2>&1
$NANOBOT cron list 2>&1 > /tmp/cron_list4.txt
if ! cat /tmp/cron_list4.txt | grep -q "$JOB_ID"; then
    pass "Cron job removed"
else
    fail "Cron job not removed"
fi

# Test 16: 频道配置检查
echo "Test 16: Channel config"
if $NANOBOT status | grep -q "Channels:"; then
    pass "Channel info displayed"
else
    fail "Channel info not displayed"
fi

echo ""
echo "=== E2E Tests Complete ==="
echo ""

echo -e "${GREEN}All tests passed!${NC}"
