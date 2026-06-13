# Electron Desktop PRD

> Status: current architecture/product snapshot
> Last refreshed: 2026-06-13

本文档不再记录早期竞品分析和假设性技术选型，改为描述当前桌面端的目标、边界和已采用方案。

## 目标

Electron Desktop 是 `maxclaw` 的桌面封装层，不重写 Agent 核心逻辑。

桌面端负责：

- 启动、停止、重启并守护 Go Gateway
- 提供桌面级体验：托盘、通知、快捷键、文件访问
- 通过 HTTP / SSE / WebSocket 复用 Gateway 已有能力
- 在设置页里管理本地 UI 配置与 Gateway 配置

不在桌面端重复实现：

- Agent Loop
- 会话持久化
- 定时任务执行器
- Provider 调用链
- Channel 后端适配

## 当前技术栈

### Desktop shell

| 技术 | 当前实现 |
|------|----------|
| Electron | `^28.0.0` |
| TypeScript | `^5.3.3` |
| Vite | `^5.0.10` |

### Renderer

| 技术 | 当前实现 |
|------|----------|
| React | `^18.2.0` |
| Redux Toolkit | `^2.0.1` |
| react-redux | `^9.0.4` |
| Tailwind CSS | `^3.4.0` |

### Content rendering

| 技术 | 当前实现 |
|------|----------|
| react-markdown | Markdown 渲染 |
| remark-gfm | GFM 支持 |
| react-syntax-highlighter | 代码高亮 |
| mermaid | Mermaid 图表 |

### Desktop integrations

| 技术 | 当前实现 |
|------|----------|
| electron-store | 本地 UI 配置存储 |
| auto-launch | 开机自启动 |
| electron-updater | 自动更新 |
| node-pty + xterm | 内嵌终端 |
| jszip | 数据导入导出 |

依赖来源：
- [electron/package.json](/Users/lua/git/maxclaw/electron/package.json:1)

## 架构

```text
Electron Main Process
  |- create/manage BrowserWindow
  |- start/stop/restart Gateway child process
  |- tray / notifications / shortcuts / updater
  |- secure IPC handlers
          |
          v
Renderer (React)
  |- Chat / Sessions / Settings / Skills / MCP / Scheduled Tasks
  |- fetch Gateway HTTP APIs
  |- consume SSE stream and WebSocket events
          |
          v
Go Gateway
  |- /api/*
  |- /ws
  |- Agent Loop
  |- Cron Service
  |- Session storage
  |- Channel registry
```

## Gateway lifecycle requirements

桌面端对 Gateway 的约束如下：

1. 启动前清理目标端口占用
2. 等待 `READY:host:port` 就绪协议，而不是只靠固定 sleep
3. 启动失败时仍然打开窗口，让用户看到错误状态
4. 异常退出后自动重试，使用指数退避
5. 应用退出时优雅停止 Gateway

对应实现：
- [electron/src/main/gateway.ts](/Users/lua/git/maxclaw/electron/src/main/gateway.ts:1)
- [electron/src/main/index.ts](/Users/lua/git/maxclaw/electron/src/main/index.ts:166)
- [internal/cli/gateway.go](/Users/lua/git/maxclaw/internal/cli/gateway.go:34)

## 功能范围

### 已交付

- 聊天界面与流式响应
- 会话列表、删除、重命名、搜索
- 技能管理与技能多选发送
- Provider 配置编辑
- 渠道配置页
- Scheduled Tasks 视图与执行历史
- 文件上传与文件预览侧边栏
- 内嵌终端面板
- Mermaid 渲染
- 系统通知
- 数据导入导出
- 自动更新
- 全局快捷键

### 当前边界

- 不承诺自定义协议注册
- 不承诺 Linux `rpm` / `snap`
- 不承诺 Windows 缩略图工具栏
- 不承诺本地 SQLite 缓存层
- 不承诺数学公式渲染

如果未来补这些能力，应先改代码，再同步刷新本文档。

## 状态存储

当前存在两类配置来源：

- Gateway 配置：`~/.maxclaw/config.json`
- Desktop 本地配置：`electron-store`

Desktop 本地配置主要包含：

- theme
- language
- renderThinkTags
- autoLaunch
- minimizeToTray
- notificationsEnabled
- shortcuts

相关代码：
- [electron/src/preload/index.ts](/Users/lua/git/maxclaw/electron/src/preload/index.ts:1)
- [electron/src/renderer/views/SettingsView.tsx](/Users/lua/git/maxclaw/electron/src/renderer/views/SettingsView.tsx:1)

## 主要页面

```text
+-------------------+------------------------------+
| Sidebar           | Main View                    |
|-------------------|------------------------------|
| New Chat          | ChatView                     |
| Sessions          | SessionsView                 |
| Scheduled Tasks   | ScheduledTasksView           |
| Skills            | SkillsView                   |
| MCP               | MCPView                      |
| Settings          | SettingsView                 |
+-------------------+------------------------------+
```

## 验证要求

桌面端相关改动至少应覆盖：

- `cd electron && npm run build`
- 相关 Go 测试 / E2E 回归
- `make build`

如果改动影响会话创建、聊天路由、打断恢复、spawn/callback 等核心流，还应补或运行 `e2e_test/` 下对应回归脚本。

## 相关文档

- [electron/README.md](/Users/lua/git/maxclaw/electron/README.md:1)
- [docs/Electron_STATUS.md](/Users/lua/git/maxclaw/docs/Electron_STATUS.md:1)
- [docs/CROSS_PLATFORM.md](/Users/lua/git/maxclaw/docs/CROSS_PLATFORM.md:1)
