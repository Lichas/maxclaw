# Bug 修复记录

## 概述

本文档记录 maxclaw 项目开发过程中发现的关键 bug 及其修复方案。

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
