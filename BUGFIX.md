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
