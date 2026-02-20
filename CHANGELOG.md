# Changelog

## [Unreleased]

### Added

#### Electron Desktop App 实现
- **全新的桌面应用程序** (`electron/`)
  - 项目结构：package.json, tsconfig.json, Vite 配置, electron-builder.yml
  - 主进程：窗口管理 (window.ts)、Gateway 进程管理 (gateway.ts)、系统托盘 (tray.ts)
  - 渲染进程：React 18 + Redux Toolkit + Tailwind CSS
  - 安全预加载脚本与 IPC 通信桥接 (ipc.ts, preload/index.ts)
  - 聊天界面支持 SSE 流式响应 (ChatView.tsx)
  - 设置面板：主题、语言、自动启动、Gateway 状态管理
  - 跨平台支持（macOS、Windows、Linux）
- **Makefile 新增目标**
  - `electron-install` - 安装 Electron 依赖
  - `electron-dev` - 开发模式运行
  - `electron-build` - 构建 Electron 应用
  - `electron-dist` - 创建可分发的安装包
- **验证**
  - `cd electron && npm install`
  - `cd electron && npm run build:main`
  - `cd electron && npm run build:preload`
  - `cd electron && npm run build:renderer`
  - `make build`

### 新增功能

#### 竞品分析与 Electron PRD 文档
- **新增桌面 Agent CoWork App 竞品特性分析与 Electron 开发需求文档** (`docs/Electron_PRD.md`)
  - 梳理核心交互层、任务系统、技能系统、模型配置、集成通知、系统设置六大模块特性
  - 基于 LobsterAI 技术栈优化选型：Electron 40.2.1 + React 18.2.0 + TypeScript 5.7.3 + Vite 5.1.4 + Redux Toolkit + better-sqlite3
  - **关键架构决策**：Electron App 作为 nanobot-go Gateway 的桌面端封装，复用现有 Agent Loop、Cron Service、Channels 能力
  - 设计进程架构：Main Process 管理 Gateway 子进程，Renderer Process 通过 HTTP API + WebSocket 与 Gateway 通信
  - 规划与 nanobot-go 集成方案：Gateway 进程管理、API 客户端封装、实时消息推送、配置同步机制
  - 制定开发里程碑（4 个 Phase）与 Gateway API 清单
- **验证**
  - 文档 Review

#### Web Fetch 新增 Chrome 会话打通模式
- **新增 `web_fetch` 的 `mode=chrome`，支持复用本机 Chrome 登录态与持久化 profile**（`pkg/tools/web.go`, `webfetcher/fetch.mjs`, `internal/config/schema.go`, `internal/agent/web_fetch.go`）
  - 支持通过 `chrome.cdpEndpoint` 连接现有 Chrome（CDP）
  - 支持通过 `chrome.userDataDir/profileName` 使用持久化用户数据目录
  - 默认补齐 `~/.nanobot/browser/<profile>/user-data` 并增加常用 Chrome 自动化启动参数
- **补充配置/文档/提示词与测试**（`README.md`, `internal/agent/prompts/system_prompt.md`, `internal/agent/web_fetch_test.go`, `pkg/tools/web_test.go`, `internal/config/config_test.go`）
  - README 增加 Chrome 模式配置示例和使用说明
  - 系统提示词明确 `web_fetch` 可用于浏览器/Chrome 抓取，避免误判“无浏览器能力”
- **验证**
  - `go test ./internal/agent ./pkg/tools ./internal/config`
  - `make build`

#### Web Fetch 新增 Host Chrome 全自动接管链路
- **新增 Chrome CDP 自动接管参数**（`internal/config/schema.go`, `internal/agent/web_fetch.go`, `pkg/tools/web.go`）
  - `chrome.autoStartCDP`：CDP 不可用时自动尝试拉起 Host Chrome
  - `chrome.takeoverExisting`：允许接管前优雅退出当前 Chrome（macOS）
  - `chrome.hostUserDataDir`：指定 Host Chrome 用户数据目录
  - `chrome.launchTimeoutMs`：控制 Host Chrome 启动并就绪等待时长
- **增强 `webfetcher/fetch.mjs` 自动接管执行流**（`webfetcher/fetch.mjs`）
  - `CDP attach 失败 -> 自动拉起 Host Chrome -> 重连 CDP -> 失败再回退 managed profile`
  - 优先复用系统 Chrome 用户数据目录，实现“已有登录态直连”
- **文档与提示词同步**（`README.md`, `internal/agent/prompts/system_prompt.md`）
  - 增加全自动接管配置示例与行为说明
  - 明确要求登录/JS站点优先走 chrome mode
- **验证**
  - `go test ./internal/agent ./pkg/tools ./internal/config`
  - `make build`

### Bug 修复

#### 修复 Host Chrome 自动接管启动时的警告空白页
- **调整 Host Chrome CDP 自动拉起参数，避免注入自动化告警标志**（`webfetcher/fetch.mjs`）
  - Host 接管启动不再带 `--disable-blink-features=AutomationControlled`
  - Host 接管启动不再强制打开 `about:blank`
  - 仅保留 CDP 接管所需参数，降低对你日常 Chrome 会话的干扰
- **验证**
  - `make build`

#### 修复 X.com 等 SPA 站点在 Chrome 抓取下的“空页面误判成功”
- **增强 `webfetcher/fetch.mjs` 的 Chrome 抓取容错与内容判定**（`webfetcher/fetch.mjs`）
  - `chrome.cdpEndpoint` 连接失败时自动回退到持久化 profile，而不是直接失败
  - 页面提取改为多选择器聚合并等待 SPA hydrate，减少只拿到空壳 DOM 的概率
  - 当 `title/text` 同时为空时返回明确错误，避免误报“访问成功”
- **增强代理提示约束**（`internal/agent/prompts/system_prompt.md`）
  - 明确禁止在 `web_fetch` 失败/空结果时宣称“已打开浏览器查看内容”
- **验证**
  - `make build`

#### 修复 takeoverExisting 模式静默回退导致无法复用本地登录态
- **收紧 Host Chrome 接管失败语义**（`webfetcher/fetch.mjs`）
  - `chrome.takeoverExisting=true` 且 CDP/AppleScript 接管失败时，直接返回错误，不再悄悄回退到 managed profile
  - 增加 AppleScript 常见失败原因映射（未开启 `Allow JavaScript from Apple Events`、macOS Automation 权限未授权）
  - 仅在非 takeover 模式保留原有“失败后回退 managed profile”路径
- **验证**
  - `node --check webfetcher/fetch.mjs`
  - `go test ./internal/agent ./pkg/tools ./internal/config`
  - `make build`

#### 调整 Chrome 登录态方案为受管 Profile 登录（对齐 OpenClaw 流程）
- **移除 `web_fetch` 中的 AppleScript 接管路径，改为稳定的 CDP/受管 profile 双路径**（`webfetcher/fetch.mjs`）
  - 不再尝试 AppleScript 注入与本地标签页接管
  - `chrome.takeoverExisting` 保留兼容但标记为废弃，并给出迁移提示
- **新增手动登录入口 `nanobot browser login`**（`internal/cli/browser.go`, `internal/cli/root.go`, `webfetcher/login.mjs`）
  - 直接打开 `~/.nanobot/browser/<profile>/user-data` 对应的受管 Chrome profile
  - 用户完成一次手动登录后，`web_fetch(mode=chrome)` 可持续复用该登录态
- **文档与提示词同步**（`README.md`, `internal/agent/prompts/system_prompt.md`）
  - 增加 X/Twitter 推荐登录流程：先 `nanobot browser login https://x.com` 再进行抓取
- **验证**
  - `node --check webfetcher/fetch.mjs`
  - `node --check webfetcher/login.mjs`
  - `go test ./internal/agent ./pkg/tools ./internal/config`
  - `make build`

#### 新增 browser 工具与完整操作手册（多步骤页面自动化）
- **新增交互式 `browser` 工具**（`pkg/tools/browser.go`, `webfetcher/browser.mjs`, `internal/agent/loop.go`）
  - 支持 `navigate/snapshot/screenshot/act/tabs` 五类操作
  - 复用现有 Chrome 配置（CDP 优先，失败回退受管 profile）
  - 按 `channel+chat_id` 维护会话状态（活动 tab、snapshot refs）
- **新增浏览器操作手册并更新主文档**（`BROWSER_OPS.md`, `README.md`, `internal/agent/prompts/system_prompt.md`）
  - 增加从登录初始化到交互执行、截图留痕、故障排查的完整流程
  - 系统提示词新增 `browser` 工具使用约束
- **补充测试**（`pkg/tools/browser_test.go`）
  - 覆盖 browser 选项归一化、脚本路径推导、会话 ID 规范化
- **验证**
  - `node --check webfetcher/browser.mjs`
  - `go test ./internal/agent ./pkg/tools ./internal/config ./internal/cli`
  - `make build`

### Bug 修复

#### Cron 任务触发后未投递到正确会话
- **修复 Cron 投递链路，避免触发后丢失 chat_id**（`internal/cli/gateway.go`, `internal/cli/cron.go`, `internal/cli/cron_test.go`）
  - Gateway 模式下，可投递 Cron 任务改为直接进入主消息总线（携带 `job.Payload.To`），由现有出站分发器发送到真实频道会话
  - `executeCronJob` 修复入站消息 `chatID` 为空的问题，避免执行后响应落到空会话
- **增强 message 出站发送链路的可观测性与防呆**（`internal/cli/gateway.go`, `internal/cli/gateway_test.go`）
  - 出站消息增加空 `channel/chat_id` 校验，避免无效发送
  - `SendMessage` 失败不再静默吞掉，统一记录到日志便于定位送达问题
  - 新增网关出站处理单测，覆盖成功发送、空 chat 丢弃、失败后继续处理
- **增强 crond 执行日志覆盖**（`internal/cron/service.go`, `internal/cron/cron_test.go`）
  - `every/cron/once` 触发调度回调后统一记录 `attempt`，并补充 `skip/execute/completed/failed` 全链路日志到 `cron.log`
  - 对无效调度配置（如 `every<=0`、空 `cron expr`、`once` 过去时间）增加可观测日志，避免“看起来没执行”
  - 新增单测验证执行尝试与跳过原因日志
- **降低一次性提醒误建为周期任务的概率**（`pkg/tools/cron.go`, `pkg/tools/cron_test.go`, `internal/agent/prompts/system_prompt.md`）
  - `at` 增加 `HH:MM[:SS]` 解析（按本地下一个该时刻），并拒绝显式过去时间
  - 系统提示增加规则：一次性提醒必须使用 `at`，仅在用户明确要求循环时使用 `cron_expr`/`every_seconds`
- **验证**
  - `go test ./internal/cli ./pkg/tools ./internal/cron`
  - `make build`
- **补充排障文档**（`BUGFIX.md`）
  - 新增条目记录“Cron 已触发但 Telegram 未收到”的证据、根因和修复链路，明确 `message` 工具不是根因
  - 验证：`make build`

### 新增功能

#### 完成 PORTING_PLAN 全量里程碑（2026-02-04 ~ 2026-02-13）
- **新增多平台频道实现**（`internal/channels/slack.go`, `internal/channels/email.go`, `internal/channels/qq.go`, `internal/channels/feishu.go`, `internal/cli/gateway.go`, `internal/channels/channels_test.go`）
  - 新增 Slack Socket Mode、Email(IMAP/SMTP)、QQ 私聊（OneBot WebSocket）、Feishu(Webhook + OpenAPI) 接入
  - Gateway 增加四类频道注册与消息总线转发
- **CLI 交互体验增强**（`internal/cli/agent.go`）
  - 交互模式切换到支持输入编辑/历史记录的行编辑器
  - 会话历史落盘到 `~/.nanobot/.agent_history`
- **配置与状态扩展**（`internal/config/schema.go`, `internal/cli/status.go`）
  - 增加 Slack/Email/QQ/Feishu 配置模型与默认值
  - `status` 命令增加新频道状态显示
- **多 provider 与 Docker 对齐**（`internal/providers/registry.go`, `internal/config/config_test.go`, `Dockerfile`, `.dockerignore`, `Makefile`, `README.md`）
  - Moonshot 默认 API Base 调整为 `https://api.moonshot.ai/v1`
  - 增补 DeepSeek/Moonshot 默认路由测试
  - 新增 Docker 镜像构建与运行入口（`make docker-build` / `make docker-run`）
- **计划收敛**（`PORTING_PLAN.md`）
  - 所有未完成里程碑项已勾选完成
- **验证**
  - `go test ./...`
  - `make build`

#### Web UI 配置编辑与服务控制增强
- **配置 JSON 编辑器升级为语法高亮并支持全屏**（`webui/src/App.tsx`, `webui/src/styles.css`, `webui/package.json`, `webui/package-lock.json`）
  - Settings 页的配置编辑从普通文本框升级为 JSON 高亮编辑器
  - 新增全屏/退出全屏按钮，便于长配置编辑
- **Web UI 新增 Gateway 重启能力**（`internal/webui/server.go`, `webui/src/App.tsx`）
  - 新增 `POST /api/gateway/restart`，由 UI 触发后台重启脚本
  - Settings 页新增 “Restart Gateway” 操作按钮
- **验证**
  - `cd webui && npm run build`
  - `go test ./...`
  - `make build`

#### Web UI 紧凑化改版与 JSON 编辑滚动修复
- **重构页面为高密度控制台布局**（`webui/src/App.tsx`, `webui/src/styles.css`）
  - 将顶部大横幅改为紧凑控制条与状态摘要条，减少首屏空白
  - Settings 区改为侧栏操作 + 主编辑区布局，提升配置效率
- **修复配置 JSON 显示不全且无滚动条问题**（`webui/src/App.tsx`, `webui/src/styles.css`）
  - 为 JSON 编辑器增加稳定滚动容器，支持纵向/横向滚动
  - 全屏模式下编辑区高度自适应，避免内容被裁切
- **验证**
  - `cd webui && npm run build`
  - `go test ./...`
  - `make build`

#### Python 2026-02-03 里程碑对齐（vLLM + 自然语言调度）
- **Cron 工具新增一次性时间调度参数 `at`**（`pkg/tools/cron.go`, `pkg/tools/cron_test.go`）
  - `cron(action="add", at="ISO datetime")` 现在会创建 `once` 任务
  - 支持 RFC3339 与常见本地时间格式解析，并在列表中展示 `at` 调度信息
- **vLLM 原始模型 ID 路由补齐**（`internal/config/schema.go`, `internal/config/config_test.go`）
  - 当模型名为 `meta-llama/...` 这类未显式带 provider 前缀的本地模型 ID 时，若已配置 `providers.vllm.apiBase`，将自动路由到 vLLM API Base
- **验证**
  - `go test ./pkg/tools ./internal/config`
  - `make build`

#### Agent 自迭代与源码定位增强
- **支持自迭代命令约束**（`internal/agent/prompts/system_prompt.md`）
  - 明确允许在自我完善任务中通过 `exec` 调用本地 `claude` / `codex`
  - 增加安全约束：默认不使用 `--dangerously-skip-permissions`
- **新增源码根目录标记机制**（`.nanobot-source-root`, `internal/agent/context.go`, `internal/agent/prompts/environment.md`）
  - 引入 `.nanobot-source-root` 作为源码根标记
  - 环境上下文新增 Source Marker / Source Directory 字段
  - 解析优先级：`NANOBOT_SOURCE_DIR` 环境变量 > 向上查找 marker > workspace 回退
- **补充测试覆盖**（`internal/agent/context_test.go`）
  - 覆盖 marker 缺失、父目录 marker、环境变量覆盖、自迭代指令注入
  - 验证：`go test ./internal/agent` 与 `go test ./...` 均通过
- **新增代理执行规范**（`AGENTS.md`, `CLAUDE.md`）
  - 要求所有代理在完成会修改仓库的需求后，自动更新 `CHANGELOG.md` 的 `Unreleased` 条目
  - 新增要求：需求成功完成且有仓库变更时，先执行 `make build`，再执行 `git commit`
  - 新增并发开发规范：多 session 并发任务使用 `git worktree` 隔离，验证通过后再合并到 `main`
- **增强源码 marker 回退发现**（`internal/agent/context.go`, `internal/agent/context_test.go`）
  - 在 `NANOBOT_SOURCE_DIR` 与 workspace 向上查找失败后，支持通过 `NANOBOT_SOURCE_SEARCH_ROOTS` 指定搜索根目录
  - 当 workspace 为默认 `~/.nanobot/workspace` 时，自动扫描 `$HOME/git` 与 `$HOME/src` 查找 `.nanobot-source-root`
  - 增加单次解析缓存，避免重复扫描
  - 验证：`go test ./internal/agent`，`make build`
- **扩展常见路径的源码根发现**（`internal/agent/context.go`, `internal/agent/context_test.go`）
  - 新增常见源码路径候选：`/Users/*/(git|src|code)`、`/home/*/(git|src|code)`、`/data/*/(git|src|code)`、`/root/(git|src|code)`、`/usr/local/src`、`/usr/src`
  - 保持受限深度扫描（避免对整盘目录进行无限递归）
  - 验证：`go test ./internal/agent`，`make build`

### Bug 修复

#### 工具调用系统修复
- **修复 OpenAI Provider 消息格式错误** (`internal/providers/openai.go`)
  - 问题：第 101 行使用了 `convertToOpenAIMessages(messages)` 而不是已构建的 `openaiMessages`
  - 影响：导致 tool_calls 信息丢失，多轮工具调用无法正常工作
  - 修复：改用正确构建的 `openaiMessages` 变量

- **移除 DeepSeek 工具禁用逻辑** (`internal/providers/openai.go`)
  - 问题：代码明确跳过 DeepSeek 模型的工具传递
  - 影响：DeepSeek 模型无法使用任何工具（web_search, exec 等）
  - 修复：移除 `isDeepSeek` 检查，所有模型统一传递工具定义

- **增强系统提示强制工具使用** (`internal/agent/context.go`)
  - 问题：模型经常选择不调用工具，而是基于训练数据回答
  - 影响：搜索、文件操作等请求返回过时或虚构信息
  - 修复：添加强制性系统提示，要求必须使用工具获取实时信息

#### 新增工具
- **Spawn 子代理工具** (`pkg/tools/spawn.go`)
  - 支持后台任务执行
  - 任务状态跟踪
  - 5 个单元测试

- **Cron 定时任务工具** (`pkg/tools/cron.go`)
  - 集成内部 cron 服务
  - 支持 add/list/remove 操作
  - 完整的 CronService 接口适配

### 测试
- 新增 Spawn 工具测试
- 新增 Cron 工具测试
- 所有工具测试通过（共 9 个测试文件）

## [0.2.0] - 2026-02-07

### 新增功能

#### Cron 定时任务系统
- 实现完整的定时任务服务 (`internal/cron/`)
- 支持三种调度类型：
  - `every`: 周期性任务（按毫秒间隔）
  - `cron`: Cron 表达式任务（标准 cron 语法）
  - `once`: 一次性任务（指定时间执行）
- CLI 命令支持：`add`, `list`, `remove`, `enable`, `disable`, `status`, `run`
- 任务持久化存储到 JSON 文件
- 与 Agent 循环集成，任务执行时使用 Agent 处理消息
- 11 个单元测试覆盖

#### 聊天频道系统
- 实现频道系统 (`internal/channels/`)
- Telegram Bot API 集成：
  - 轮询模式接收消息
  - 支持发送消息到指定 Chat
  - HTML 格式解析
- Discord HTTP API 集成：
  - Webhook 和 Bot API 支持
  - Markdown 转义工具
- 统一 Channel 接口设计
- 注册表模式管理多频道
- 15 个单元测试覆盖

#### Gateway 集成增强
- Gateway 命令集成频道系统
- Gateway 集成 Cron 服务
- 出站消息处理器，自动转发到对应频道

### 测试
- 新增 6 个 E2E 测试用例（Cron 和频道相关）
- 所有 E2E 测试通过（共 16 个）
- 单元测试覆盖 5 个包：bus, channels, config, cron, session, tools

### 文档
- 更新 README.md，添加 Cron 和频道使用说明
- 更新 E2E 测试文档
- 新增 CHANGELOG.md

## [0.1.0] - 2026-02-07

### 初始功能
- 项目初始化
- 配置系统（支持多 LLM 提供商）
- 消息总线架构
- 工具系统（文件操作、Shell、Web 搜索）
- Agent 核心循环
- LLM Provider 支持（OpenRouter, Anthropic, OpenAI, DeepSeek等）
- CLI 命令（agent, gateway, status, onboard, version）
- 会话持久化
- 工作区限制（安全沙箱）
- E2E 测试脚本
