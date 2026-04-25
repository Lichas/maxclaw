# Bug 修复记录

## 概述

本文档记录 maxclaw 项目开发过程中发现的关键 bug 及其修复方案。

---

## 2026-04-25 - OpenRouter 模型路由到错误 API 端点 (401)

**问题**：
- 使用 OpenRouter 模型如 `tencent/hy3-preview:free` 时，LLM 请求返回 401 错误
- 错误信息显示 `provider=openai api_base=https://api.openai.com/v1`，但 API key 是 OpenRouter 的格式 (`sk-or-v1...`)
- OpenRouter 的 key 被发送到了 OpenAI 原生端点

**根因**：
- `config.GetAPIKey()` 对无法识别的模型 ID fallback 到 ProviderSpecs 中第一个配置了 key 的 provider（openrouter 排第一）
- `config.GetAPIBase()` 对无法识别的模型 ID 却 fallback 到 vllm 或返回空字符串，导致使用默认的 `https://api.openai.com/v1`
- 两者 fallback 逻辑不一致：key 拿到了 OpenRouter 的，base URL 却用了 OpenAI 的

**修复**：
- 在 `GetAPIBase()` 的 `looksLikeRawModelID` fallback 路径中，当 vllm 未配置时，按 ProviderSpecs 顺序返回第一个配置了 apiBase 的 provider
- 这样 `GetAPIBase` 和 `GetAPIKey` 的 fallback 行为保持一致
- vLLM 显式配置时仍优先保留（不影响现有 vLLM 用户）

**验证**：
```bash
go test ./internal/config/ -v -run "TestGetAPIBaseFallsBack|TestGetAPIBaseVLLM"
make build
```

**修复文件**：
- `internal/config/schema.go`
- `internal/config/schema_test.go`

---

## 2026-03-18 - React Error #310: Hooks 顺序错误

**问题**：
- 从 Starter 模式切换到 Chat 模式时，页面白屏，控制台报错：`Error #310: Rendered fewer hooks than expected`
- 点击任务详情等操作触发模式切换时也会报错

**根因**：
- 性能优化时将 `useMemo` 放置在条件提前 `return` 之后
- Starter 模式下提前 return，没有执行 useMemo（0 个 hook）
- 切换到 Chat 模式后执行 useMemo（1 个 hook）
- React hooks 计数不匹配导致 Error #310

**修复**：
- 将 `renderedMessages` useMemo 移至 `if (isStarterMode)` 条件之前
- 确保每次渲染 hooks 调用顺序一致

**验证**：
```bash
cd electron && npm run build  # 构建成功无报错
```

**修复文件**：
- `electron/src/renderer/views/ChatView.tsx`

---

## 2026-03-10 - Go 版本声明、CI、Docker 与文档相互矛盾

**问题**：
- `go.mod` 使用 `go 1.22`，但同时固定 `toolchain go1.24.2`。
- `Dockerfile`、`README`、`README.zh.md` 和桌面构建 workflow 仍然写着 Go 1.21。
- `go mod tidy -diff` 显示模块依赖声明和 `go.sum` 也不是当前 Go 1.24 toolchain 整理后的状态。

**根因**：
- 仓库在不同阶段分别升级过本地 toolchain、模块版本和外层构建文档，但没有把这些入口统一到同一个 Go 基线。
- 结果是用户、Docker、CI 和模块解析各自依赖不同版本来源，容易引发"go.mod 配错了"的判断和构建环境漂移。

**修复**：
- 将 `go.mod` 的语言版本升级到 Go 1.24，并保留 `toolchain go1.24.2` 作为本地精确 toolchain。
- 将桌面构建 workflow 改为直接从 `go.mod` 读取 Go 版本，避免再手写过期版本。
- 将 Docker builder 和中英文 README / 开发说明统一更新为 Go 1.24+。
- 重新执行 `go mod tidy`，清理过期间接依赖并修正直接依赖分组。
- 修正 `pkg/tools/mcp.go` 中对错误列表的聚合方式，改用 `errors.New`，避免 Go 1.24 下因 `fmt.Errorf` 非常量格式串检查导致测试失败。
- 将 `internal/cron/cron_history.json` 标记为运行时文件并从 Git 跟踪中移除，避免本地运行或测试继续制造无关 diff。

**修复文件**：
- `go.mod`
- `go.sum`

---

## 2026-04-04 - 新增 MCP Server 后运行中对话无法感知新工具

**问题**：
- 桌面端启动后，在 MCP 管理页新增、编辑或删除 MCP server，`config.json` 已经更新成功。
- 但随后直接发起新对话时，Agent 仍然只看到旧的 MCP 工具集合。
- 用户必须手动重启 app / gateway，新的 MCP tool 才会出现在对话工具调用里。

**根因**：
- 运行中的 `AgentLoop` 只在 gateway 启动时读取一次 `cfg.Tools.MCPServers` 并创建 `MCPConnector`。
- `/api/mcp` 的增删改接口只负责保存配置和更新 `Server.cfg`，没有把新的 MCP 配置同步到运行中的 `AgentLoop`。
- 因此磁盘配置和运行态工具注册发生了分离，导致“配置已保存，但当前会话看不到新 MCP”。

**修复**：
- 为 `AgentLoop` 增加运行态 MCP 刷新能力：关闭旧连接、移除旧 `mcp_` 工具、按最新配置重新连接并注册。
- 在 `/api/mcp` 的新增、更新、删除，以及 `/api/config` 的整体配置更新后，立即调用运行态 MCP 刷新逻辑。
- 增加回归测试，覆盖“新增 MCP 后立即可见工具”和“删除 MCP 后工具立即移除”。

**验证**：
```bash
go test ./internal/agent ./internal/webui ./pkg/tools
./e2e_test/gateway_agent_regression.sh
make build
```

**修复文件**：
- `internal/agent/loop.go`
- `internal/webui/server.go`
- `internal/webui/server_test.go`
- `pkg/tools/registry.go`

---

## 2026-04-11 - 聊天输入框输入/删除卡顿

**问题**：
- 在聊天输入框里连续输入或连续删除时，光标与文本更新有明显迟滞，体感不够实时。
- 输入 `@` 或 `/` 触发补全弹层时，卡顿更明显。

**根因**：
- `handleInputChange` 在 `@mention` / `/slash` 分支中会提前 `return`，导致部分按键路径没有及时写回受控 `value`。
- 输入过程中 `ChatView` 的消息渲染相关回调引用频繁变化，触发历史消息区域不必要重渲染，放大输入延迟体感。

**修复**：
- 调整输入处理顺序：先执行 `setInputForCurrentSession(value)`，再处理 `@mention` / `/slash` 的弹层状态逻辑。
- 将消息渲染链路相关函数改为稳定引用（`useCallback`），减少输入期间无关重渲染。

**验证**：
```bash
bash e2e_test/interrupt_test.sh
bash e2e_test/gateway_agent_regression.sh
cd electron && npm run build
make build
```

**修复文件**：
- `electron/src/renderer/views/ChatView.tsx`
