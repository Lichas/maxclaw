# maxclaw 架构概览

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
- **Memory Summarizer (`internal/memory`)**：
  - Gateway 启动后按小时检查一次
  - 将“前一天会话摘要”幂等追加到 `memory/MEMORY.md`（`## Daily Summaries`）
  - 无会话则跳过，不写空摘要
- **Skills (`internal/skills`)**：
  - 从 `<workspace>/skills` 发现并加载技能文档
  - 支持 `@skill:<name>` 与 `$<name>` 按需选择
  - 支持 `all/none` 特殊选择器
- **Channels (`internal/channels`)**：
  - Telegram（Bot API 轮询）
  - WhatsApp（Bridge WebSocket）
  - Discord（Bot API）
  - WebSocket（自定义接入）
- **Web UI (`webui/`)**：
  - 前端打包后由 Gateway 静态托管
  - 通过 `/api/*` 与后端通讯

## Electron Desktop App 架构

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
