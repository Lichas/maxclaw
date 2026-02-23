# maxclaw 架构概览

## 部署架构

### 官网部署（Vercel）

静态官网托管于 Vercel，与主仓库共享代码：

```
website/
├── index.html          # 单页应用入口
├── vercel.json         # Vercel 静态托管配置
└── ...
```

**部署流程**：
```bash
cd website
vercel --prod --yes
```

**域名绑定**：
- 自动分配: `xxx.vercel.app`
- 自定义域名: `maxclaw.top`（通过 A 记录指向 Vercel）

---

## 组件分层

- **CLI (`cmd/maxclaw`)**：统一命令行入口（agent / gateway / cron / bind 等）。
- **Gateway (`internal/cli/gateway`)**：
  - 加载配置、创建 Provider、初始化 Agent Loop
  - 初始化 Message Bus / Channel Registry
  - 启动 Web UI Server（同端口）
- **Agent Loop (`internal/agent`)**：
  - 负责对话轮次与工具调用
  - 调用 `pkg/tools` 完成文件/命令/web 等动作
  - 会话与记忆保存在 workspace 目录
  - 自动注入长期记忆 `memory/MEMORY.md` 与短周期心跳 `memory/heartbeat.md`
  - **智能打断支持**：支持流式生成时的用户插话/打断（见下文）
- **Memory Summarizer (`internal/memory`)**：
  - Gateway 启动后按小时检查一次
  - 将”前一天会话摘要”幂等追加到 `memory/MEMORY.md`（`## Daily Summaries`）
  - 无会话则跳过，不写空摘要
  - **两层内存系统**：
    - `memory/MEMORY.md`：长期事实与偏好，始终注入系统上下文
    - `memory/HISTORY.md`：追加式历史摘要，适合 grep 检索，不自动注入
- **Skills (`internal/skills`)**：
  - 从 `<workspace>/skills` 发现并加载技能文档
  - 支持 `@skill:<name>` 与 `$<name>` 按需选择
  - 支持 `all/none` 特殊选择器
- **智能打断系统 (`internal/agent/interrupt.go`)**：
  - **InterruptibleContext**：支持上下文取消和消息追加队列
  - **意图分析器** (`internal/agent/intent.go`)：基于关键词识别用户意图（打断/补充/停止/继续）
  - **后台检查器**：支持 Telegram 等轮询渠道的消息检查
  - **双模式 UI**：
    - 打断重试（Enter）- 停止当前生成，重新回复
    - 补充上下文（Shift+Enter）- 不打断，追加到下一轮
  - **流式取消支持**：Provider 层支持上下文取消，立即停止 token 生成

- **Channels (`internal/channels`)**：
  - Telegram（Bot API 轮询，支持打断）
  - WhatsApp（Bridge WebSocket）
  - Discord（Bot API）
  - Slack（Socket Mode）
  - Email（IMAP/SMTP）
  - Feishu/Lark（Webhook + OpenAPI）
  - QQ（OneBot WebSocket）
  - WebSocket（自定义接入）
- **Web UI (`webui/` / `electron/`)**：
  - **Web 版本**：前端打包后由 Gateway 静态托管（同端口 18890）
  - **Electron 版本**：独立桌面应用，通过 HTTP API + WebSocket 与 Gateway 通信
  - **实时通信**：WebSocket 推送（`internal/webui/websocket.go`）
    - 连接管理：gorilla/websocket 库
    - 自动重连：指数退避，最多5次
    - 消息类型：通知、状态更新、流式事件
  - **API 端点**：
    - `/api/message` - 发送消息（支持 SSE 流式）
    - `/api/sessions` - 会话管理
    - `/api/cron` - 定时任务 CRUD
    - `/api/cron/history` - 执行历史
    - `/api/skills` - 技能管理
    - `/api/upload` - 文件上传
    - `/ws` - WebSocket 连接
  - **流式响应**：`stream=1` 或 `Accept: text/event-stream` 时返回 SSE 格式
    - 事件类型：`status`, `tool_start`, `tool_result`, `content_delta`, `final`, `error`
    - 非流式 JSON 路径保持兼容

## Electron Desktop App 架构

### 应用结构

```
electron/
├── src/
│   ├── main/               # 主进程
│   │   ├── index.ts        # 入口 + 自动更新
│   │   ├── gateway.ts      # Gateway 子进程管理
│   │   ├── ipc.ts          # IPC 处理器
│   │   ├── notifications.ts # 系统通知
│   │   └── windows-integration.ts # Windows 平台集成
│   ├── renderer/           # 渲染进程
│   │   ├── views/
│   │   │   ├── ChatView.tsx       # 聊天主界面
│   │   │   ├── SettingsView.tsx   # 设置页面
│   │   │   ├── SkillsView.tsx     # 技能市场
│   │   │   └── ScheduledTasksView.tsx # 定时任务
│   │   ├── components/
│   │   │   ├── MarkdownRenderer.tsx   # Markdown 渲染
│   │   │   ├── FilePreviewSidebar.tsx # 文件预览侧边栏
│   │   │   ├── TerminalPanel.tsx      # 终端面板
│   │   │   └── Sidebar.tsx            # 侧边栏
│   │   └── hooks/
│   │       └── useGateway.ts   # Gateway API 封装
│   └── preload/            # 预加载脚本
└── electron-builder.yml    # 打包配置
```

### 文件预览侧边栏

- **会话级文件隔离**：文件工具在有会话上下文时，默认解析到 `<workspace>/.sessions/<sessionKey>/`
- **支持的格式**：PDF、Word (docx)、Excel (xlsx)、PowerPoint (pptx)、图片、Markdown、代码文件
- **操作按钮**：消息中自动识别文件链接，显示"预览"按钮
- **拖拽调整**：预览栏左侧支持拖拽调整宽度
- **打开目录**：文件操作统一为"打开所在目录"（而非直接打开文件）

### 终端集成

基于 **node-pty + @xterm/xterm** 实现真终端：

```
┌─────────────────────────────────────────────┐
│  ChatView 聊天界面                            │
│  ┌─────────────────────────────────────┐   │
│  │  消息流                              │   │
│  │  ...                                │   │
│  └─────────────────────────────────────┘   │
│  ┌─────────────────────────────────────┐   │
│  │  TerminalPanel (可折叠)              │   │
│  │  ┌──────────────────────────────┐  │   │
│  │  │  node-pty 伪终端              │  │   │
│  │  │  + xterm.js 渲染              │  │   │
│  │  │  + 主题跟随应用               │  │   │
│  │  └──────────────────────────────┘  │   │
│  └─────────────────────────────────────┘   │
└─────────────────────────────────────────────┘
```

- **按任务隔离**：不同 `sessionKey` 对应独立 PTY 会话
- **多 shell 兜底**：自动尝试 `$SHELL` → `/bin/zsh` → `/bin/bash` → `/bin/sh`
- **IPC 接口**：
  - `terminal:start` - 启动终端
  - `terminal:input` - 发送输入
  - `terminal:resize` - 调整窗口大小

### 执行历史追踪

定时任务的执行历史持久化：

- **存储位置**：`<workspace>/.cron/history.jsonl`
- **记录内容**：任务 ID、执行时间、状态、输出、错误信息
- **API 端点**：
  - `GET /api/cron/history` - 全部历史
  - `GET /api/cron/history/{id}` - 单个任务历史
- **UI 展示**：任务卡片展开显示执行时间线

### 数据导入/导出

配置和会话数据的备份与恢复：

**导出流程**：
```
SettingsView 点击导出
    ↓
IPC `data:export` → Gateway API
    ↓
打包 config.json + sessions.json + metadata.json
    ↓
JSZip 生成带日期文件名 ZIP
    ↓
用户选择保存路径
```

**导入流程**：
```
用户选择 ZIP 文件
    ↓
解压验证结构
    ↓
IPC `data:import` → Gateway API
    ↓
恢复配置并重启 Gateway
```

### Windows 平台支持

- **任务栏集成**：跳转列表、进度条、缩略图工具栏
- **NSIS 安装程序**：自定义向导、协议注册 (`maxclaw://`)
- **自动启动**：注册表方式（Windows）/ launchd（macOS）
- **CI/CD**：GitHub Actions 多平台构建

### 自动更新机制

使用 **electron-updater** 实现自动更新，从 GitHub Releases 获取新版本。

#### 配置方式

**electron-builder.yml**：
```yaml
publish:
  provider: github
  owner: Lichas
  repo: maxclaw
  releaseType: release
```

#### 更新流程

```
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────┐
│   启动后5秒     │────▶│  每小时检查      │────▶│  用户手动检查   │
└─────────────────┘     └──────────────────┘     └─────────────────┘
          │                       │                       │
          ▼                       ▼                       ▼
┌─────────────────────────────────────────────────────────────────┐
│                     autoUpdater.checkForUpdates()                 │
└─────────────────────────────────────────────────────────────────┘
          │
          ▼
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────┐
│ update-available│────▶│ 用户点击下载    │────▶│ downloadUpdate  │
└─────────────────┘     └──────────────────┘     └─────────────────┘
                                                      │
          ┌───────────────────────────────────────────┘
          ▼
┌─────────────────┐     ┌──────────────────┐
│ update-downloaded│────▶│ quitAndInstall  │
└─────────────────┘     └──────────────────┘
```

#### 关键实现

**主进程** (`electron/src/main/index.ts`)：
```typescript
function setupAutoUpdater() {
  // 开发模式跳过
  if (isDev) return;

  autoUpdater.autoDownload = false; // 手动下载

  // 事件处理
  autoUpdater.on('update-available', (info) => {
    mainWindow?.webContents.send('update:available', info);
  });

  autoUpdater.on('update-downloaded', () => {
    mainWindow?.webContents.send('update:downloaded');
  });

  // 定时检查
  setTimeout(() => autoUpdater.checkForUpdates(), 5000);   // 启动后5秒
  setInterval(() => autoUpdater.checkForUpdates(), 3600000); // 每小时
}
```

**IPC 接口** (`electron/src/main/ipc.ts`)：
- `update:check` - 手动检查更新
- `update:download` - 下载更新
- `update:install` - 安装并重启

**渲染进程** (`electron/src/renderer/views/SettingsView.tsx`)：
- 设置页面提供检查/下载/安装按钮
- 显示当前更新状态（checking/available/downloading/downloaded）
- 展示新版本信息

#### 发布流程

1. 打包应用：`npm run build && npm run dist`
2. 创建 GitHub Release
3. 上传 `dist/` 目录中的安装包（.dmg, .exe, .AppImage）
4. 客户端自动检测到新版本并提示用户

#### 技术实现细节

**谁提供"有新版本"的接口？**

GitHub Releases 托管的 `latest.yml`（Windows）、`latest-mac.yml`（macOS）、`latest-linux.yml`（Linux）元数据文件。

**打包时生成的元数据文件**（electron-builder 自动生成）：
```yaml
# latest-mac.yml
version: 1.0.1
files:
  - url: Maxclaw-1.0.1-mac.zip
    sha512: abc123...  # 文件哈希校验
    size: 52567890
path: Maxclaw-1.0.1-mac.zip
sha512: abc123...
releaseDate: '2026-02-23T00:00:00.000Z'
```

**检查更新的完整流程**：

```
┌─────────────────────────────────────────────────────────────┐
│  1. autoUpdater.checkForUpdates()                           │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│  2. 请求 GitHub API                                         │
│     GET /repos/{owner}/{repo}/releases/latest               │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│  3. 从 Release Assets 下载 latest-{platform}.yml            │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│  4. 解析 YAML，获取 version                                 │
│     对比当前版本 app.getVersion()                           │
│     1.0.1 > 1.0.0 → 有新版本                                │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│  5. 触发 'update-available' 事件                            │
│     返回 { version, files, releaseDate, ... }               │
└─────────────────────────────────────────────────────────────┘
```

**版本比较规则**（使用 semver）：

| 当前版本 | 远程版本 | 结果 |
|---------|---------|------|
| 1.0.0 | 1.0.1 | ✅ 有更新（patch）|
| 1.0.0 | 1.1.0 | ✅ 有更新（minor）|
| 1.0.0 | 2.0.0 | ✅ 有更新（major）|
| 1.0.0 | 1.0.0 | ❌ 无更新 |
| 1.0.1 | 1.0.0 | ❌ 无更新（本地更新）|

**下载和安装流程**：

```typescript
// 1. 用户点击"下载更新"
autoUpdater.downloadUpdate()
  ├─▶ 根据 latest-mac.yml 中的 url 下载 .zip/.dmg
  ├─▶ 校验 sha512 哈希（防篡改）
  ├─▶ 保存到本地缓存目录
  └─▶ 触发 'update-downloaded' 事件

// 2. 用户点击"安装并重启"
autoUpdater.quitAndInstall()
  ├─▶ 退出应用
  ├─▶ 解压/替换旧版本（electron-updater 内置逻辑）
  └─▶ 启动新版本
```

**配置关键点**：

```yaml
# electron-builder.yml
publish:
  provider: github
  owner: Lichas      # GitHub 用户名
  repo: maxclaw      # 仓库名
  releaseType: release  # 只检查正式 release，不包括 draft/prerelease
```

```json
// package.json（版本号来源）
{
  "name": "maxclaw",
  "version": "1.0.1"  // 这个版本号会被打包进应用
}
```

**安全机制**：
- **SHA512 校验**：下载完成后校验文件哈希，防止中间人攻击或文件损坏
- **HTTPS 传输**：所有下载通过 HTTPS，防止窃听和篡改
- **签名验证**（macOS/Windows）：安装包需要有效的代码签名证书

### 全局快捷键

使用 Electron `globalShortcut` API 注册系统级快捷键：

```typescript
// 默认快捷键
CommandOrControl+Shift+Space  // 显示/隐藏窗口
CommandOrControl+N            // 新建对话
```

支持在设置页面自定义快捷键组合。

### 数据导入/导出

使用 **JSZip** 实现配置备份：

- **导出**：打包 `config.json` + `sessions.json` + `metadata.json` 为 ZIP
- **导入**：解压 ZIP 并通过 Gateway API 恢复配置

## Web Fetch 方案

### HTTP 模式（默认）

- 直接由 Go `net/http` 抓取页面
- 轻量、无额外依赖
- 适合文档/API/静态页面

### 浏览器模式（推荐复杂站点）

为了模拟真实浏览器行为（真实 UA、JS 渲染、反爬策略），使用 **Node + Playwright** 作为可选抓取引擎：

- **实现位置**：`webfetcher/fetch.mjs`
- **工作方式**：
  1. `web_fetch` 工具根据配置判断 `mode=browser`
  2. Go 侧启动 Node 进程，向 `fetch.mjs` 传入 JSON 请求（stdin）
  3. Playwright 打开无头浏览器、加载页面、提取 `document.body.innerText`
  4. Go 侧截断并返回结果

### 配置入口

`~/.maxclaw/config.json`：

```json
{
  "tools": {
    "web": {
      "fetch": {
        "mode": "browser",
        "scriptPath": "/absolute/path/to/maxclaw/webfetcher/fetch.mjs",
        "nodePath": "node",
        "timeout": 30,
        "userAgent": "Mozilla/5.0 ...",
        "waitUntil": "domcontentloaded"
      }
    }
  }
}
```

## Browser 工具（交互式页面控制）

`browser` 工具支持多步骤页面自动化：

```javascript
// webfetcher/browser.mjs
{
  "action": "navigate",    // 打开页面
  "url": "https://x.com/home"
}
{
  "action": "snapshot",    // 抓取页面文本与可交互元素引用 [ref]
}
{
  "action": "act",         // 执行操作
  "act": "click",          // click | type | press | wait
  "ref": 3                 // 元素引用 ID
}
{
  "action": "screenshot"   // 保存截图
}
{
  "action": "tabs"         // list | switch | close | new
}
```

**会话状态管理**：按 `channel+chat_id` 维护活动标签页、snapshot refs

**推荐流程**（X/Twitter）：
1. `maxclaw browser login https://x.com` - 手动登录保存状态
2. Agent 使用 `browser` 工具自动交互
3. `screenshot` 保存证据

## Skills 机制

- **发现路径**：`<workspace>/skills`
  - `skills/<name>.md`
  - `skills/<name>/SKILL.md`
- **过滤规则**：
  - 未指定选择器时，默认加载全部技能
  - `@skill:<name>` 或 `$<name>` 时仅加载匹配技能
  - `@skill:all` / `$all`：加载全部
  - `@skill:none` / `$none`：本轮不加载
- **管理命令**：
  - `maxclaw skills list`
  - `maxclaw skills show <name>`
  - `maxclaw skills validate`

## MCP 支持（Model Context Protocol）

支持接入外部 MCP 服务器，扩展 Agent 工具能力：

```json
{
  "tools": {
    "mcpServers": {
      "filesystem": {
        "command": "npx",
        "args": ["-y", "@modelcontextprotocol/server-filesystem", "/path/to/dir"]
      },
      "remote-http": {
        "url": "https://mcp.example.com/sse"
      }
    }
  }
}
```

- **配置兼容**：支持 Claude Desktop / Cursor 风格的 `mcpServers` 配置
- **工具透传**：MCP 服务器工具作为原生 Agent 工具使用
- **超时保护**：`initialize`/`list_tools`/`tools/call` 默认超时防止阻塞

## WhatsApp / Telegram 绑定

- **WhatsApp**：由 `bridge/` (Baileys) 维护登录态，Gateway 通过 WebSocket 接入。
  - CLI：`maxclaw whatsapp bind --bridge ws://localhost:3001`
  - Web UI：状态页显示二维码
- **Telegram**：使用 Bot Token，Web UI 显示 Bot 链接二维码用于快速打开聊天。

## Heartbeat 机制（参考 OpenClaw）

- 文件位置优先级：
  1. `<workspace>/memory/heartbeat.md`
  2. `<workspace>/heartbeat.md`（兼容）
- 注入时机：每次 `ContextBuilder.BuildMessages` 构造 system prompt 时
- 用途：存放短周期状态（当前重点、阻塞、下一检查点），与长期记忆 `MEMORY.md` 分层管理

## 每日 Memory 汇总机制

- 扫描来源：`<workspace>/.sessions/*.json`
- 汇总窗口：默认“昨天”本地时间
- 写入位置：`<workspace>/memory/MEMORY.md`
- 幂等策略：检测 `### YYYY-MM-DD` 标题，存在则不重复写入

## 开发工作流

### 一键启动（本地 all-in-one）

```bash
# 构建 + 启动 Gateway + 启动 Electron 桌面应用
make build && make restart-daemon && make electron-start
```

**命令分解**：
- `make build` - 编译 Go 二进制到 `build/maxclaw`
- `make restart-daemon` - 重启 Gateway 服务（清理旧进程 + 启动新进程）
- `make electron-start` - 启动 Electron 桌面应用（会自动连接 Gateway）

**其他常用命令**：
```bash
# 仅构建
make build

# 开发模式（热重载）
make dev

# 停止后台服务
make down-daemon

# 运行测试
make test
```
