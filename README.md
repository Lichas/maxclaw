# maxclaw

轻量级的个人 AI 助手框架（Go）。支持多渠道消息、工具调用、Web UI、定时任务与可选浏览器抓取。

<details open>
<summary>中文</summary>

## 亮点
- Go 原生实现的 Agent Loop 与工具系统
- 多渠道接入：Telegram、WhatsApp（Bridge）、Discord、WebSocket
- Web UI + API（同一端口，打包后静态托管）
- 定时任务（Cron/Once/Every）
- Heartbeat 上下文（`memory/heartbeat.md`）
- 每日 Memory 汇总（自动写入 `memory/MEMORY.md`）
- 可选浏览器抓取（Node + Playwright）
- 完整日志：`~/.maxclaw/logs`

## 快速开始
1. 安装依赖：Go 1.21+，Node.js 18+
2. 构建：`make build`
3. 初始化：`./build/maxclaw onboard`
4. 配置：编辑 `~/.maxclaw/config.json`
5. 启动：`./build/maxclaw gateway`

Agent CLI 常用参数：
- `--session/-s` 指定会话 ID（默认 `cli:direct`）
- `--markdown` / `--no-markdown` 控制输出渲染
- `--logs` / `--no-logs` 控制是否显示日志目录提示

## Linux / macOS 一键安装
可直接用自动分流安装器（会按系统选择 `install_linux.sh` 或 `install_mac.sh`）：

```bash
curl -fsSL https://raw.githubusercontent.com/Lichas/maxclaw/main/install.sh | bash
```

常用参数示例：

```bash
# 指定版本
curl -fsSL https://raw.githubusercontent.com/Lichas/maxclaw/main/install.sh | bash -s -- --version v0.1.0

# Linux 指定安装目录和端口
curl -fsSL https://raw.githubusercontent.com/Lichas/maxclaw/main/install.sh | bash -s -- --dir /opt/maxclaw --bridge-port 3001 --gateway-port 18890

# macOS 不安装 launchd（仅拷贝文件）
curl -fsSL https://raw.githubusercontent.com/Lichas/maxclaw/main/install.sh | bash -s -- --no-launchd
```

Linux 默认会安装并启动：
- `maxclaw-bridge.service`
- `maxclaw-gateway.service`

安装后请编辑 `~/.maxclaw/config.json` 填写 API Key 与模型。

## 配置文件
路径：`~/.maxclaw/config.json`

最小示例：
```json
{
  "providers": {
    "openrouter": { "apiKey": "your-api-key" }
  },
  "agents": {
    "defaults": {
      "model": "anthropic/claude-opus-4-5",
      "workspace": "/absolute/path/to/your/workspace"
    }
  }
}
```

### MiniMax 配置示例
`maxclaw` 通过 OpenAI 兼容接口使用 MiniMax：

```json
{
  "providers": {
    "minimax": {
      "apiKey": "your-minimax-key"
    }
  },
  "agents": {
    "defaults": {
      "model": "minimax/MiniMax-M2"
    }
  }
}
```

### Qwen（DashScope）配置示例
```json
{
  "providers": {
    "dashscope": {
      "apiKey": "your-dashscope-key"
    }
  },
  "agents": {
    "defaults": {
      "model": "qwen-max"
    }
  }
}
```

默认 API Base：`https://dashscope.aliyuncs.com/compatible-mode/v1`（可在 `providers.dashscope.apiBase` 覆盖）。

### 智谱 GLM（编码套餐端点）配置示例
```json
{
  "providers": {
    "zhipu": {
      "apiKey": "your-zhipu-key",
      "apiBase": "https://open.bigmodel.cn/api/coding/paas/v4"
    }
  },
  "agents": {
    "defaults": {
      "model": "glm-4.5"
    }
  }
}
```

提示：如果你使用中国大陆站点密钥（`minimaxi.com`），可显式设置：

```json
{
  "providers": {
    "minimax": {
      "apiKey": "your-minimax-key",
      "apiBase": "https://api.minimaxi.com/v1"
    }
  }
}
```

### Workspace 设置
默认工作区：`~/.maxclaw/workspace`

建议使用绝对路径，也支持 `~` 或 `$HOME` 自动展开：
```json
{
  "agents": {
    "defaults": {
      "workspace": "~/maxclaw-workspace"
    }
  }
}
```

限制文件/命令只能在工作区内执行：
```json
{
  "tools": {
    "restrictToWorkspace": true
  }
}
```

### Heartbeat（短周期状态）
受 OpenClaw 的 `heartbeat.md` 思路启发，maxclaw 会在每轮对话自动加载：
- `<workspace>/memory/heartbeat.md`（优先）
- `<workspace>/heartbeat.md`（兼容）

用于记录当前优先级、阻塞项、下一步检查点。`onboard` 会自动创建模板文件。

### 每日 Memory 汇总
Gateway 启动后会开启每日汇总器（每小时检查一次），自动把“前一天会话摘要”追加到：
- `<workspace>/memory/MEMORY.md` 的 `## Daily Summaries` 小节

特性：
- 幂等：同一天只写一次（按 `### YYYY-MM-DD` 去重）
- 无会话不写入
- 用于长期记忆沉淀与跨天回顾

### 两层内存系统（重构版）
- `memory/MEMORY.md`：长期事实与偏好，始终注入系统上下文。
- `memory/HISTORY.md`：追加式历史摘要日志，不自动注入上下文，适合用 `grep` 检索。

行为：
- 当会话消息达到阈值时，会自动把旧消息摘要归档到 `HISTORY.md`。
- 执行 `/new` 时，会先归档当前会话再清空会话上下文。

### Skills 支持
技能目录位于 `<workspace>/skills`，支持两种结构：
- `skills/<name>.md`
- `skills/<name>/SKILL.md`

触发语法：
- `@skill:<name>`：只加载指定技能
- `$<name>`：只加载指定技能
- `@skill:all` / `$all`：加载全部技能
- `@skill:none` / `$none`：本轮禁用技能加载

管理命令：
```bash
./build/maxclaw skills list
./build/maxclaw skills show <name>
./build/maxclaw skills validate
./build/maxclaw skills add https://github.com/vercel-labs/agent-skills --path skills --skill react-best-practices
./build/maxclaw browser login https://x.com
```

在聊天里让 Agent 安装 skills 时，请明确说“调用 `exec` 执行 `maxclaw skills add ...`”；skills 安装位置固定为 `<workspace>/skills`，不是 Python 包安装。

## Web UI
Web UI 与 API 同端口，默认 `18890`：

1. 构建：`make webui-install && make webui-build`
2. 启动：`./build/maxclaw gateway`
3. 访问：`http://localhost:18890`

如果访问显示 `Web UI not built`，请先运行 `make webui-build`。

## WhatsApp（Bridge）
WhatsApp 通过 `bridge/`（Baileys）接入，Go 侧通过 WebSocket 连接 Bridge。

1. 构建 Bridge：`make bridge-install && make bridge-build`
2. 启动 Bridge：`BRIDGE_PORT=3001 BRIDGE_TOKEN=your-secret make bridge-run`
3. 绑定（命令行扫码）：
```bash
./build/maxclaw whatsapp bind --bridge ws://localhost:3001
```
4. Web UI：状态页显示二维码

代理（部分地区需要）：
- 设置 `BRIDGE_PROXY` 或 `PROXY_URL/HTTP_PROXY/HTTPS_PROXY/ALL_PROXY`

如果使用个人 WhatsApp 账号，希望手机发消息也触发机器人回复：
```json
{
  "channels": {
    "whatsapp": {
      "enabled": true,
      "bridgeUrl": "ws://localhost:3001",
      "allowSelf": true
    }
  }
}
```

## Telegram
1. 使用 @BotFather 创建 Bot，获取 Token
2. 绑定（命令行输出 QR）：
```bash
./build/maxclaw telegram bind --token "123456:AA..."
```
3. Web UI：状态页显示打开聊天的二维码
4. 如网络需要代理，可在配置中设置 `channels.telegram.proxy`（例如 `http://127.0.0.1:7897`）

## 频道配置示例
```json
{
  "channels": {
    "telegram": {
      "enabled": true,
      "token": "your-bot-token",
      "allowFrom": [],
      "proxy": ""
    },
    "discord": {
      "enabled": true,
      "token": "your-discord-token",
      "allowFrom": []
    },
    "whatsapp": {
      "enabled": true,
      "bridgeUrl": "ws://localhost:3001",
      "bridgeToken": "shared-secret-optional",
      "allowFrom": [],
      "allowSelf": false
    },
    "websocket": {
      "enabled": false,
      "host": "0.0.0.0",
      "port": 18791,
      "path": "/ws",
      "allowOrigins": []
    },
    "slack": {
      "enabled": false,
      "botToken": "xoxb-...",
      "appToken": "xapp-...",
      "allowFrom": []
    },
    "email": {
      "enabled": false,
      "consentGranted": false,
      "imapHost": "imap.example.com",
      "imapPort": 993,
      "imapUsername": "bot@example.com",
      "imapPassword": "your-imap-password",
      "smtpHost": "smtp.example.com",
      "smtpPort": 587,
      "smtpUsername": "bot@example.com",
      "smtpPassword": "your-smtp-password",
      "allowFrom": []
    },
    "qq": {
      "enabled": false,
      "wsUrl": "ws://localhost:3002",
      "accessToken": "",
      "allowFrom": []
    },
    "feishu": {
      "enabled": false,
      "appId": "cli_xxx",
      "appSecret": "xxx",
      "verificationToken": "",
      "listenAddr": "0.0.0.0:18792",
      "webhookPath": "/feishu/events",
      "allowFrom": []
    }
  }
}
```

## Docker

仓库已内置 `Dockerfile`，可直接构建运行：

```bash
make docker-build
make docker-run
```

安全建议：生产环境为 Go 与 Bridge 配置相同的 `bridgeToken`，启用共享密钥认证。

## Web Fetch（浏览器/Chrome 模式）
适合需要真实浏览器行为或复用登录态的站点：
```json
{
  "tools": {
    "web": {
      "fetch": {
        "mode": "chrome",
        "scriptPath": "/absolute/path/to/maxclaw/webfetcher/fetch.mjs",
        "nodePath": "node",
        "timeout": 30,
        "waitUntil": "domcontentloaded",
        "chrome": {
          "cdpEndpoint": "http://127.0.0.1:9222",
          "profileName": "chrome",
          "userDataDir": "~/.maxclaw/browser/chrome/user-data",
          "channel": "chrome",
          "headless": true,
          "autoStartCDP": true,
          "launchTimeoutMs": 15000
        }
      }
    }
  }
}
```
说明：
- `mode=browser`：临时无状态 Chromium（不复用登录态）。
- `mode=chrome`：优先使用 `chrome.cdpEndpoint` 接管现有 Chrome；若不配置 `cdpEndpoint`，则使用持久化 profile 目录。
- `chrome.autoStartCDP=true`：`cdpEndpoint` 不可用时自动拉起 Host Chrome 并重连。
- 若要复用你正在使用的 Chrome 登录态，请先以远程调试端口启动 Chrome（示例：`--remote-debugging-port=9222`）。
- 推荐登录流程（X/Twitter 等需登录站点）：
  - 先运行 `maxclaw browser login https://x.com`，在打开的受管 profile 里手动登录一次。
  - 登录完成后返回对话，继续使用 `web_fetch`（`mode=chrome`）即可复用该 profile 登录态。
- `chrome.takeoverExisting` 已废弃，不再用于 AppleScript 接管本地标签页。
安装 Playwright：`make webfetch-install`

## Browser 工具（交互式页面控制）
`browser` 工具用于多步骤页面交互，支持：
- `navigate`：打开页面
- `snapshot`：抓取页面文本与可交互元素引用（`[ref]`）
- `act`：点击、输入、按键、等待
- `tabs`：列出/切换/关闭/新建标签页
- `screenshot`：保存截图

推荐流程（X/Twitter）：
1. 先执行 `./build/maxclaw browser login https://x.com` 并手动登录受管 profile。
2. 在聊天中让 agent 使用 `browser` 工具：
   - `action="navigate", url="https://x.com/home"`
   - `action="snapshot"`
   - `action="act", act="click", ref=3`
3. 需要证据时使用 `action="screenshot"` 保存截图路径。

完整操作手册：`BROWSER_OPS.md`

## MCP（Model Context Protocol）
支持把外部 MCP 服务器工具接入为原生 Agent 工具，配置格式兼容 Claude Desktop / Cursor 的 `mcpServers` 条目（可直接复制每个 server 的配置块）。

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

兼容写法：也支持将 `mcpServers` 放在配置文件顶层（Claude Desktop/Cursor 风格），启动时会自动合并到 `tools.mcpServers`。

## 一键启动
前台启动（Bridge + Gateway）：
```bash
make up
```
`make up` 会自动尝试清理占用 `BRIDGE_PORT`（默认 `3001`）和 `GATEWAY_PORT`（默认 `18890`）的旧进程，避免端口冲突导致启动失败。

后台常驻：
```bash
make up-daemon
```

重启：
```bash
make restart-daemon
```

停止后台服务：
```bash
make down-daemon
```

可用环境变量：
- `BRIDGE_PORT`（默认 `3001`）
- `GATEWAY_PORT`（默认 `18890`）
- `BRIDGE_TOKEN`（可选，Bridge 认证密钥）
- `BRIDGE_PROXY`（代理）

## 日志
日志目录：`~/.maxclaw/logs`

文件包括：
- `gateway.log`
- `session.log`
- `tools.log`
- `channels.log`
- `cron.log`
- `webui.log`

## 架构说明
详见 `ARCHITECTURE.md`。

## 维护与排障
统一维护手册：`MAINTENANCE.md`

</details>

<details>
<summary>English</summary>

## Highlights
- Go-native agent loop and tool system
- Multi-channel: Telegram, WhatsApp (Bridge), Discord, WebSocket
- Web UI + API on the same port (static bundle served by gateway)
- Cron/Once/Every scheduler
- Heartbeat context (`memory/heartbeat.md`)
- Daily memory digest written to `memory/MEMORY.md`
- Optional browser fetch (Node + Playwright)
- Structured logs in `~/.maxclaw/logs`

## Quick Start
1. Install Go 1.21+ and Node.js 18+
2. Build: `make build`
3. Init: `./build/maxclaw onboard`
4. Configure: edit `~/.maxclaw/config.json`
5. Run: `./build/maxclaw gateway`

## One-Command Install (Linux / macOS)
Use the auto-switch installer (it dispatches to `install_linux.sh` or `install_mac.sh`):

```bash
curl -fsSL https://raw.githubusercontent.com/Lichas/maxclaw/main/install.sh | bash
```

Common examples:

```bash
# Pin a specific release tag
curl -fsSL https://raw.githubusercontent.com/Lichas/maxclaw/main/install.sh | bash -s -- --version v0.1.0

# Linux custom install dir and ports
curl -fsSL https://raw.githubusercontent.com/Lichas/maxclaw/main/install.sh | bash -s -- --dir /opt/maxclaw --bridge-port 3001 --gateway-port 18890

# macOS install files only (skip launchd)
curl -fsSL https://raw.githubusercontent.com/Lichas/maxclaw/main/install.sh | bash -s -- --no-launchd
```

On Linux, installer enables and starts:
- `maxclaw-bridge.service`
- `maxclaw-gateway.service`

After install, edit `~/.maxclaw/config.json` and set your API key/model.

## Config File
Path: `~/.maxclaw/config.json`

Minimal example:
```json
{
  "providers": {
    "openrouter": { "apiKey": "your-api-key" }
  },
  "agents": {
    "defaults": {
      "model": "anthropic/claude-opus-4-5",
      "workspace": "/absolute/path/to/your/workspace"
    }
  }
}
```

### Workspace
Default workspace: `~/.maxclaw/workspace`

Absolute paths are recommended; `~` and `$HOME` are expanded automatically:
```json
{
  "agents": {
    "defaults": {
      "workspace": "~/maxclaw-workspace"
    }
  }
}
```

Restrict tools to workspace only:
```json
{
  "tools": {
    "restrictToWorkspace": true
  }
}
```

### Heartbeat (Short-Cycle Status)
Inspired by OpenClaw's `heartbeat.md`, maxclaw auto-loads heartbeat context on each turn:
- `<workspace>/memory/heartbeat.md` (preferred)
- `<workspace>/heartbeat.md` (fallback)

Use it to track current priorities, blockers, and next checkpoint. `onboard` creates a starter template automatically.

### Daily Memory Digest
When gateway starts, a daily summarizer runs (hourly check) and appends yesterday's conversation digest to:
- `<workspace>/memory/MEMORY.md` under `## Daily Summaries`

Behavior:
- Idempotent: one summary per day (`### YYYY-MM-DD`)
- No writes when there was no activity
- Designed for long-term memory consolidation

### Skills Support
Skills are loaded from `<workspace>/skills` with two supported layouts:
- `skills/<name>.md`
- `skills/<name>/SKILL.md`

Selectors:
- `@skill:<name>`: load only one skill
- `$<name>`: load only one skill
- `@skill:all` / `$all`: load all skills
- `@skill:none` / `$none`: disable skills for this turn

Management commands:
```bash
./build/maxclaw skills list
./build/maxclaw skills show <name>
./build/maxclaw skills validate
./build/maxclaw skills add https://github.com/vercel-labs/agent-skills --path skills --skill react-best-practices
./build/maxclaw browser login https://x.com
```

When asking the agent in chat to install skills, explicitly request `exec` with `maxclaw skills add ...`. Skills are installed into `<workspace>/skills` (not Python package installs).

## Web UI
Web UI and API share the same port (default `18890`).

1. Build: `make webui-install && make webui-build`
2. Run: `./build/maxclaw gateway`
3. Visit: `http://localhost:18890`

If you see `Web UI not built`, run `make webui-build` first.

## WhatsApp (Bridge)
WhatsApp is connected via a Node.js Bridge (Baileys) and a WebSocket link to Go.

1. Build Bridge: `make bridge-install && make bridge-build`
2. Run Bridge: `BRIDGE_PORT=3001 BRIDGE_TOKEN=your-secret make bridge-run`
3. Bind (CLI QR):
```bash
./build/maxclaw whatsapp bind --bridge ws://localhost:3001
```
4. Web UI shows QR on the status page

Proxy (for restricted regions):
- Set `BRIDGE_PROXY` or `PROXY_URL/HTTP_PROXY/HTTPS_PROXY/ALL_PROXY`

If you use a personal WhatsApp account and want phone messages to trigger replies:
```json
{
  "channels": {
    "whatsapp": {
      "enabled": true,
      "bridgeUrl": "ws://localhost:3001",
      "bridgeToken": "shared-secret-optional",
      "allowSelf": true
    }
  }
}
```

## Telegram
1. Create a bot with @BotFather and get the token
2. Bind (CLI outputs QR):
```bash
./build/maxclaw telegram bind --token "123456:AA..."
```
3. Web UI shows a QR that opens the bot chat
4. If your network requires a proxy, set `channels.telegram.proxy` (for example `http://127.0.0.1:7897`)

## Channel Config Example
```json
{
  "channels": {
    "telegram": {
      "enabled": true,
      "token": "your-bot-token",
      "allowFrom": [],
      "proxy": ""
    },
    "discord": {
      "enabled": true,
      "token": "your-discord-token",
      "allowFrom": []
    },
    "whatsapp": {
      "enabled": true,
      "bridgeUrl": "ws://localhost:3001",
      "bridgeToken": "shared-secret-optional",
      "allowFrom": [],
      "allowSelf": false
    },
    "websocket": {
      "enabled": false,
      "host": "0.0.0.0",
      "port": 18791,
      "path": "/ws",
      "allowOrigins": []
    }
  }
}
```

## Web Fetch (Browser/Chrome Mode)
For sites that need real browser behavior or authenticated Chrome sessions:
```json
{
  "tools": {
    "web": {
      "fetch": {
        "mode": "chrome",
        "scriptPath": "/absolute/path/to/maxclaw/webfetcher/fetch.mjs",
        "nodePath": "node",
        "timeout": 30,
        "waitUntil": "domcontentloaded",
        "chrome": {
          "cdpEndpoint": "http://127.0.0.1:9222",
          "profileName": "chrome",
          "userDataDir": "~/.maxclaw/browser/chrome/user-data",
          "channel": "chrome",
          "headless": true,
          "autoStartCDP": true,
          "launchTimeoutMs": 15000
        }
      }
    }
  }
}
```
Notes:
- `mode=browser`: stateless Chromium fetch.
- `mode=chrome`: use `chrome.cdpEndpoint` to attach an existing Chrome session, or a persistent managed profile when `cdpEndpoint` is empty.
- `chrome.autoStartCDP=true`: auto-launch host Chrome and retry CDP when endpoint is unavailable.
- To reuse your live Chrome login state, start Chrome with remote debugging enabled (for example, `--remote-debugging-port=9222`).
- Recommended login flow for X/Twitter and similar sites:
  - Run `maxclaw browser login https://x.com` and complete manual login once in the managed profile.
  - Then continue with `web_fetch` in `mode=chrome` to reuse that managed profile state.
- `chrome.takeoverExisting` is deprecated and no longer used for AppleScript tab takeover.
Install Playwright: `make webfetch-install`

## Browser Tool (Interactive Control)
The `browser` tool supports multi-step page control:
- `navigate`: open URL
- `snapshot`: collect page text plus interactable refs (`[ref]`)
- `act`: click/type/press/wait
- `tabs`: list/switch/close/new tab
- `screenshot`: save screenshot to file

Recommended flow for X/Twitter:
1. Run `./build/maxclaw browser login https://x.com` and finish manual login in managed profile.
2. In chat, ask agent to use `browser` tool:
   - `action="navigate", url="https://x.com/home"`
   - `action="snapshot"`
   - `action="act", act="click", ref=3`
3. Use `action="screenshot"` when you need evidence artifacts.

Full runbook: `BROWSER_OPS.md`

## MCP (Model Context Protocol)
maxclaw can connect external MCP servers and expose their tools as native agent tools.
The server entry format is compatible with Claude Desktop / Cursor `mcpServers` blocks.

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

Compatibility note: top-level `mcpServers` (Claude Desktop/Cursor style) is also accepted and merged into `tools.mcpServers` at load time.

## One-Command Start
Foreground (Bridge + Gateway):
```bash
make up
```
`make up` automatically attempts to stop existing processes on both `BRIDGE_PORT` (default `3001`) and `GATEWAY_PORT` (default `18890`) to avoid startup failures from port conflicts.

Background daemon:
```bash
make up-daemon
```

Restart:
```bash
make restart-daemon
```

Stop background:
```bash
make down-daemon
```

Env vars:
- `BRIDGE_PORT` (default `3001`)
- `GATEWAY_PORT` (default `18890`)
- `BRIDGE_TOKEN` (optional, shared secret for bridge auth)
- `BRIDGE_PROXY` (proxy)

## Logs
Logs directory: `~/.maxclaw/logs`

Files:
- `gateway.log`
- `session.log`
- `tools.log`
- `channels.log`
- `cron.log`
- `webui.log`

## Architecture
See `ARCHITECTURE.md` for details.

## Operations & Troubleshooting
Unified runbook: `MAINTENANCE.md`
Browser-specific runbook: `BROWSER_OPS.md`

</details>
