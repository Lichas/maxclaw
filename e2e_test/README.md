# E2E Tests

端到端测试脚本，使用 Shell 脚本测试完整的用户流程。

## 运行测试

```bash
cd e2e_test
./run.sh
```

## 测试内容

### 基础功能
1. **版本命令** - 验证 `maxclaw version` 正常工作
2. **初始化流程** - 测试 `maxclaw onboard` 创建配置和工作空间
3. **状态命令** - 验证 `maxclaw status` 显示正确信息
4. **配置加载** - 测试自定义配置加载
5. **工作区限制** - 验证工作区路径显示
6. **Agent 命令** - 验证 Agent 命令可用
7. **网关命令** - 验证 Gateway 命令可用
8. **配置验证** - 验证配置 JSON 格式
9. **会话隔离** - 测试会话持久化
10. **环境变量** - 验证 HOME 环境变量

### Cron 定时任务
11. **Cron 命令** - 验证 `maxclaw cron` 命令可用
12. **添加任务** - 测试添加定时任务
13. **任务状态** - 验证 `cron status` 显示正确信息
14. **启用/禁用** - 测试任务的启用和禁用功能
15. **删除任务** - 测试删除定时任务

### 聊天频道
16. **频道配置** - 验证频道配置正确显示

## 添加新测试

在 `run.sh` 中添加新的测试用例：

```bash
# Test N: 描述
echo "Test N: Description"
if [ 测试条件 ]; then
    pass "描述"
else
    fail "描述"
fi
```

## 注意

- 测试使用临时 HOME 目录，不会影响真实配置
- 需要提前安装 `go` 和 `python3`（用于 JSON 验证）
- 不测试真实的 LLM API 调用（需要 API key）

## 智能插话功能测试 (Smart Interruption Testing)

### 单元测试验证
```bash
# 运行意图分析测试
go test ./internal/agent/... -v -run "TestIntentAnalyzer"

# 运行中断上下文测试
go test ./internal/agent/... -v -run "TestInterruptibleContext"
```

### 手动测试步骤

1. **启动 Gateway**
   ```bash
   maxclaw gateway
   ```

2. **启动 Electron 前端**
   ```bash
   cd electron && npm run dev
   ```

3. **测试打断模式 (Cancel)**
   - 在 ChatView 中发送一条消息
   - 在生成回复过程中，按 `Enter` 键
   - 或输入内容后点击"打断"按钮
   - 预期：当前生成被取消，新消息（如果有）被处理

4. **测试补充模式 (Append)**
   - 在 ChatView 中发送一条消息
   - 在生成回复过程中，输入补充内容
   - 按 `Shift+Enter` 键
   - 或点击"补充"按钮
   - 预期：当前生成继续，补充内容被添加到上下文中

5. **WebSocket 协议测试**
   ```bash
   # 使用 wscat 测试
   wscat -c ws://localhost:18890/ws
   
   # 发送普通消息
   {"type":"chat","session":"test","content":"你好"}
   
   # 发送打断请求
   {"type":"interrupt","session":"test","mode":"cancel"}
   
   # 发送补充请求
   {"type":"interrupt","session":"test","mode":"append","content":"记得补充代码"}
   ```

### E2E 自动化测试
```bash
# 需要设置 API key
export DEEPSEEK_API_KEY="your-key"
# 或
export OPENROUTER_API_KEY="your-key"

# 运行 E2E 测试
./e2e_test/interrupt_test.sh
```
