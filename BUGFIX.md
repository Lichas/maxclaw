# Bug 修复记录

## 概述

本文档记录 maxclaw 项目开发过程中发现的关键 bug 及其修复方案。

---

## 目录

### 按类别索引

| 类别 | 数量 | Bug 列表 |
|------|------|---------|
| **UI/Frontend** | 6 | [架构图对比度](#2026-02-23---字符架构图代码块颜色对比度过低), [聊天窗口高度](#2026-02-23---electron-聊天窗口信息流高度异常), [流式事件](#2026-02-21---electron-聊天窗只见文本不见执行过程), [窗口双闪](#2026-02-21---electron-启动时窗口闪动两次), [SkillsView 循环](#2026-02-22---skillsview-无限循环), [Electron 安装](#2026-02-20---electron-安装后无法启动) |
| **LLM/Provider** | 4 | [消息格式错误](#bug-1-openai-provider-消息格式错误), [DeepSeek 禁用工具](#bug-2-deepseek-模型工具被禁用), [模型不使用工具](#bug-3-模型不使用工具), [DeepSeek 400](#bug-4-deepseek-返回-400) |
| **Channels** | 3 | [WhatsApp 自发消息](#2026-02-08---whatsapp-收不到回复), [Telegram 代理](#2026-02-15---telegram-收不到回复), [Telegram 间歇无回复](#2026-02-15--2026-02-16-事件总结telegram-间歇性无回复) |
| **Daemon/部署** | 3 | [未清理 Gateway](#2026-02-16---make-up-daemon-未清理旧-gateway-进程), [假启动](#2026-02-16---daemon-假启动未被检测), [Electron 安装](#2026-02-20---electron-安装后无法启动) |
| **Tools/Agent** | 2 | [Cron 缺上下文](#2026-02-16---agent-内-cron-工具提示缺少-channelchat_id), [Cron 触发未收到](#2026-02-17---cron-已触发但-telegram-未收到) |
| **性能** | 1 | [Agent 回复慢](#2026-02-23---agent-简单问候hi回复慢定位分析) |

### 按时间索引

| 日期 | Bug |
|------|-----|
| 2026-02-23 | [架构图对比度](#2026-02-23---字符架构图代码块颜色对比度过低), [聊天窗口高度](#2026-02-23---electron-聊天窗口信息流高度异常), [Agent 回复慢](#2026-02-23---agent-简单问候hi回复慢定位分析) |
| 2026-02-22 | [SkillsView 循环](#2026-02-22---skillsview-无限循环) |
| 2026-02-21 | [流式事件](#2026-02-21---electron-聊天窗只见文本不见执行过程), [窗口双闪](#2026-02-21---electron-启动时窗口闪动两次) |
| 2026-02-20 | [Electron 安装](#2026-02-20---electron-安装后无法启动) |
| 2026-02-17 | [DeepSeek 400](#bug-4-deepseek-返回-400), [Cron 触发未收到](#2026-02-17---cron-已触发但-telegram-未收到) |
| 2026-02-16 | [Cron 缺上下文](#2026-02-16---agent-内-cron-工具提示缺少-channelchat_id), [未清理 Gateway](#2026-02-16---make-up-daemon-未清理旧-gateway-进程), [假启动](#2026-02-16---daemon-假启动未被检测) |
| 2026-02-15 | [Telegram 代理](#2026-02-15---telegram-收不到回复), [Telegram 间歇无回复](#2026-02-15--2026-02-16-事件总结telegram-间歇性无回复) |
| 2026-02-08 | [WhatsApp 自发消息](#2026-02-08---whatsapp-收不到回复) |
| 2026-02-07 | [消息格式错误](#bug-1-openai-provider-消息格式错误), [DeepSeek 禁用工具](#bug-2-deepseek-模型工具被禁用), [模型不使用工具](#bug-3-模型不使用工具) |

### 验证命令速查

```bash
# 工具测试
go test ./pkg/tools/... -v

# Provider 测试
go test ./internal/providers/... -v

# Agent 测试
go test ./internal/agent/... -v

# 全量测试
go test ./...

# 构建验证
make build
cd electron && npm run build
```

---

## 2026-02-23 - 字符架构图代码块颜色对比度过低（难以阅读）

**问题**：
- 聊天消息中用文本字符（ASCII）表示的架构图在渲染后颜色过浅，内容接近不可读。

**根因**：
- 该类内容属于 Markdown 无语言代码块（`pre > code`）。
- `prose` 默认代码块文本色偏浅，而页面代码块背景为浅色，形成“浅色文字 + 浅色背景”的低对比组合。

**修复**：
- 在 Markdown 渲染器中为无语言代码块显式设置高对比文本色（`text-foreground`）。
- 为 `pre` 容器补充边框、背景和 `code` 子元素样式覆盖，避免被 `prose` 默认样式覆盖。

**修复文件**：
- `electron/src/renderer/components/MarkdownRenderer.tsx`

**验证**：
- `cd electron && npm run build`
- `make build`

---

## 2026-02-23 - Electron 聊天窗口信息流高度异常（底部大面积空白）

**问题**：
- 聊天窗口右侧信息流区域高度被明显压缩，仅顶部可见少量内容，底部出现大面积空白。

**根因**：
- 聊天态布局中，`FilePreviewSidebar` 被渲染到纵向容器底部而非与消息区同一行。
- 侧栏组件本身带 `h-full`，在错误布局下占满了整段可用高度，挤压了消息流滚动区域。

**修复**：
- 将 `FilePreviewSidebar` 放回消息区同级的横向 `flex` 容器中，恢复高度分配。
- 同步移除聊天态外层浅绿色叠底与内层留边卡片，改为单层铺满主容器，避免双层卡片视觉与高度计算干扰。

**修复文件**：
- `electron/src/renderer/views/ChatView.tsx`

**验证**：
- `cd electron && npm run build`
- `make build`

---

## 2026-02-21 - Electron 聊天窗“只见文本不见执行过程”与流式事件兼容问题

**问题**：
- Electron 端原先只消费 `delta/response` 文本，无法展示 Agent 的执行状态和工具调用过程。
- `/api/message` 流式链路在演进后，前端对结构化事件未解析，导致 UI 信息密度明显落后于竞品。

**根因**：
- 网关 Hook 仅按“纯文本增量”处理 SSE，未消费 `status/tool_start/tool_result/error` 等事件类型。
- 聊天视图只有气泡文本，没有事件轨迹容器，工具执行细节被丢失。

**修复**：
- 后端流式输出统一为结构化事件：`status/tool_start/tool_result/content_delta/final/error`，并保留原 JSON 非流式返回。
- Electron `useGateway` 增加结构化 SSE 解析与兼容分支（旧 `delta/response` 仍可工作）。
- Electron `ChatView` 新增执行轨迹卡片，流式展示状态与工具结果，同时保留打字机文本体验。

**兼容性结论**：
- Telegram 不依赖 `/api/message` 的 SSE 分支，仍走既有消息总线流程，不受本次改造影响。
- `/api/message` 的非流式 JSON 行为保持不变，旧客户端可继续使用。

---

## 2026-02-21 - Electron 启动时窗口闪动两次、DevTools 打开两份

**问题**：
- 启动 Desktop App 时，主窗口出现明显双闪。
- 开发模式下偶发出现两个 detached DevTools 窗口。

**根因**：
- 启动链路里 `initializeApp()` 需要等待 `gateway.startFresh()`，在此期间 `mainWindow` 仍为 `null`。
- macOS 可能在这段时间触发 `app.on('activate')`，导致与 `initializeApp()` 并发调用开窗逻辑。
- 两条路径同时执行 `openMainWindow()`，形成重复窗口创建与重复 `openDevTools()` 调用。

**修复**：
- 在主进程新增窗口创建去重入口 `ensureMainWindow()`（Promise 锁），统一给 `initializeApp()` 与 `activate` 复用。
- 已有窗口存在时直接 `show()`，不再重复创建。
- Dev 模式仅在未打开 DevTools 时调用 `openDevTools`。

**修复文件**：
- `electron/src/main/index.ts`

**验证**：
- `cd electron && npm run build`
- `make build`
- `cd electron && npm run dev`（观察启动阶段不再双闪、DevTools 不再重复弹出）

---

## Bug #1: OpenAI Provider 消息格式错误

**发现时间**: 2026-02-07

**影响范围**: 所有使用工具调用的场景

**问题描述**:

在 `internal/providers/openai.go` 的第 101 行，构建 ChatCompletionRequest 时使用了错误的函数：

```go
// 错误代码
req := openai.ChatCompletionRequest{
    Model:    model,
    Messages: convertToOpenAIMessages(messages),  // ❌ 错误！
}
```

问题：`convertToOpenAIMessages` 函数没有处理 `tool_calls` 字段，导致工具调用消息在多轮对话中丢失。

**正确代码**:

```go
// 修复后的代码
req := openai.ChatCompletionRequest{
    Model:    model,
    Messages: openaiMessages,  // ✅ 正确！使用前面构建好的消息
}
```

**影响**:
- LLM 无法看到之前的工具调用历史
- 多轮工具调用无法正常进行
- 工具结果无法正确传回给模型

**修复提交**: 修复消息格式，使用正确构建的 openaiMessages 变量

---

## Bug #2: DeepSeek 模型工具被禁用

**发现时间**: 2026-02-07

**影响范围**: 使用 DeepSeek 模型的所有用户

**问题描述**:

代码中明确检查 DeepSeek 模型并跳过工具传递：

```go
// 原代码
isDeepSeek := strings.Contains(model, "deepseek")

var openaiTools []openai.Tool
if !isDeepSeek && len(tools) > 0 {  // ❌ DeepSeek 被排除！
    // 构建工具定义...
}
```

这导致 DeepSeek 模型完全无法使用任何工具（web_search, exec, read_file 等）。

**修复方案**:

移除 DeepSeek 特殊处理，所有模型统一传递工具：

```go
// 修复后的代码
var openaiTools []openai.Tool
if len(tools) > 0 {  // ✅ 所有模型都传递工具
    // 构建工具定义...
}
```

**验证结果**:

```
$ ./maxclaw agent -m "搜索今日AI新闻"
[Agent] Executing tool: web_search (id: call_00_xxx, args: {"query": "AI 新闻 今日"})
```

DeepSeek 模型成功调用了 web_search 工具。

---

## Bug #3: 模型不使用工具（提示词问题）

**发现时间**: 2026-02-07

**影响范围**: 所有模型（特别是 DeepSeek）

**问题描述**:

即使工具正确定义和传递，模型也经常选择不调用工具，而是基于训练数据回答。例如：

- 用户问"搜索今日新闻"
- 模型回答："由于我无法直接访问实时网络，我会基于近期趋势..."
- 实际上 web_search 工具是可用的

**根本原因**:

系统提示不够明确，模型没有理解"必须使用工具"的重要性。

**修复方案**:

重写系统提示，使用强制性语言：

```go
// 修复前的提示（不够强烈）
"You have access to various tools... Always prefer using tools over guessing..."

// 修复后的提示（强制性）
`You are maxclaw, a lightweight AI assistant with access to tools.

ABSOLUTE REQUIREMENT: You MUST use tools when they are available.

MANDATORY RULES:
1. When user asks for news → YOU MUST CALL web_search tool
2. When user asks about files → YOU MUST CALL read_file/list_dir tools
3. NEVER say "I cannot access the internet" - you HAVE web_search tool
4. NEVER rely on training data for current information`
```

**验证结果**:

修复后，模型正确调用工具：

```
[Agent] LLM response - HasToolCalls: true, ToolCalls count: 1
[Agent] Executing tool: list_dir (args: {"path": "."})
[Agent] Tool result: [FILE] CHANGELOG.md...
```

---

## Bug #4: DeepSeek 返回 400（messages content 类型不兼容）

**发现时间**: 2026-02-07

**影响范围**: 使用 DeepSeek/OpenAI 兼容接口的所有用户（尤其是工具调用场景）

**问题描述**:

调用 DeepSeek 时出现报错：

```
invalid type: sequence, expected a string
```

原因是 `openai-go v0.1.0-alpha.61` 在发送请求时将 `messages[].content` 序列化为 **数组**（content parts），而 DeepSeek 的 OpenAI 兼容端点要求 `content` 为 **字符串**。因此请求被拒绝，导致工具无法被调用。

**修复方案**:

用轻量 OpenAI 兼容 HTTP 客户端替换 SDK 调用，强制使用字符串 `content` 并保留 `tool_calls`，保证 DeepSeek 能正常解析请求。

**修复结果**:

DeepSeek 可正常返回工具调用（web_search / exec / read_file 等）。

---

## 修复验证命令

测试工具调用：

```bash
# 测试 web_search（需要配置 BRAVE_API_KEY）
./maxclaw agent -m "搜索今日AI新闻"

# 测试 list_dir
./maxclaw agent -m "列出当前目录"

# 测试 read_file
./maxclaw agent -m "查看 README.md 内容"

# 测试 exec
./maxclaw agent -m "运行 pwd 命令"
```

---

## 相关文件

- `internal/providers/openai.go` - LLM Provider 实现
- `internal/agent/context.go` - 系统提示构建
- `internal/agent/loop.go` - Agent 循环和工具执行
- `pkg/tools/*.go` - 工具实现

---

## 测试覆盖

所有工具现在有完整的单元测试：

```bash
go test ./pkg/tools/... -v
# 测试包括：
# - TestReadFileTool
# - TestWriteFileTool
# - TestEditFileTool
# - TestListDirTool
# - TestExecTool
# - TestMessageTool
# - TestSpawnTool
# - TestCronTool
```

---

## 2026-02-08 - WhatsApp 收不到回复（自发消息）

**问题**：WhatsApp 已连接但手机发送消息无回复，Web UI 也无会话记录。  
**原因**：Baileys 标记手机发出的消息为 `fromMe=true`，原逻辑默认忽略该类型，导致入站消息被丢弃。  
**修复**：新增 `channels.whatsapp.allowSelf` 开关并默认关闭；Bridge 不再丢弃 `fromMe` 消息；启用时允许处理 `fromMe` 消息，并加入“最近出站消息”回环过滤避免自循环。  
**验证**：
- Bridge 输出 QR & 连接成功  
- CLI `whatsapp bind` 能收到并打印 QR  
- 启用 `allowSelf=true` 后，手机发消息能进入会话并触发回复  

## 2026-02-15 - Telegram 收不到回复（代理变量未生效）

**问题**：Telegram 机器人显示已绑定，但用户发送 `hi/how areyou` 无回复。  
**原因**：
- 网关进程未继承到可用代理，`getUpdates` 请求无法连通 Telegram。
- 启动脚本仅识别大写代理变量（`HTTP_PROXY/HTTPS_PROXY/ALL_PROXY`），忽略了常见小写变量（`http_proxy/https_proxy/all_proxy`）。
- Telegram 通道未使用 `channels.telegram.proxy` 配置，且轮询失败缺少日志，排查成本高。  
**修复**：
- 启动脚本支持大小写代理变量并统一导出给 bridge/gateway。
- Telegram 通道新增 `proxy` 配置接入（`channels.telegram.proxy`），HTTP 客户端支持显式代理。
- 补充 `getUpdates` 失败日志与状态错误信息。  
**验证**：
- `api/status` 显示 `channels` 包含 `telegram` 且状态为 `ready`。
- Telegram 入站消息可被消费，不再堆积在 `getUpdates` pending 队列中。

## 2026-02-16 - `make up-daemon` 未清理旧 Gateway 进程

**问题**：`make up-daemon` 仅强制清理 Bridge 端口，Gateway 端口被旧进程占用时可能出现僵持或启动失败。  
**原因**：启动脚本只实现了 `FORCE_BRIDGE_KILL`，缺少 `GATEWAY_PORT` 清理逻辑。  
**修复**：
- 在 `start_daemon.sh` 和 `start_all.sh` 增加 `FORCE_GATEWAY_KILL` 与 Gateway 端口清理。
- `make up` / `make up-daemon` 默认同时启用 `FORCE_BRIDGE_KILL=1` 和 `FORCE_GATEWAY_KILL=1`。  
**验证**：
- 先用测试进程占用 `18890`，执行 `make up-daemon` 后占用进程被清理并成功拉起 Gateway。

## 2026-02-16 - daemon “假启动”未被检测

**问题**：`make up-daemon` 输出启动成功并写入 PID，但进程可能很快退出，用户继续发 Telegram 消息无回复。  
**原因**：启动脚本只记录 PID，不验证“进程仍存活且端口已监听”。  
**修复**：
- 在 `start_daemon.sh` 增加服务健康检查：
  - 校验 PID 存活
  - 校验对应端口已监听
  - 失败时打印日志 tail 并返回错误  
**验证**：
- 启动后立即检查 `18890` / `3001` 监听与 `/api/status` 返回正常。

## 2026-02-15 ~ 2026-02-16 事件总结：Telegram 间歇性无回复

**用户现象**：
- Telegram 发送 `hi` / `how areyou` / 搜索请求后，偶发无回复。
- `make up-daemon` 显示“启动成功”，但过一会儿又收不到消息。  

**排查过程（关键证据）**：
1. 先看运行态：`/api/status` 与 `lsof -iTCP:18890`，发现 PID 文件存在但端口未监听（网关已退出）。
2. 查 Telegram 服务端队列：`getWebhookInfo` / `getUpdates`，确认 `pending_update_count` 增长且存在未消费消息。
3. 查本地日志：`channels.log` 在网关存活时可看到 `telegram inbound/send`，离线期间无新记录。
4. 对比环境变量：发现代理变量在 daemon 场景未稳定传递，导致 Telegram 轮询偶发不可用。  

**最终根因（组合问题）**：
- 代理变量传递不稳定（大小写变量与 daemon 启动环境差异）。
- `start_daemon.sh` 早期仅写 PID，不验证进程与端口健康，出现“假启动”。
- 仅清理 Bridge 端口，旧 Gateway 进程/占用问题会干扰重启。  

**最终修复集合**：
- Telegram 通道支持 `channels.telegram.proxy`，并增加轮询错误日志。
- 启动脚本支持大小写代理变量并传递给 gateway/bridge。
- `make up` / `make up-daemon` 同时强制清理 Bridge + Gateway 端口占用。
- `start_daemon.sh` 增加启动后健康检查（PID 存活 + 端口监听），失败即报错并打印日志。  

**回归检查清单**：
```bash
make restart-daemon
lsof -nP -iTCP:3001 -sTCP:LISTEN
lsof -nP -iTCP:18890 -sTCP:LISTEN
curl -sS http://127.0.0.1:18890/api/status
tail -f /Users/lua/.maxclaw/logs/channels.log
```
预期：`channels` 包含 `telegram`，`telegram.status=ready`，并能看到 `telegram inbound` 与 `telegram send`。

## 2026-02-16 - Agent 内 `cron` 工具提示“缺少 channel/chat_id”

**问题**：在聊天里让 Agent 创建定时任务时，模型调用 `cron` 工具经常返回 `no session context (channel/chat_id)`，用户看到“理论支持但实际不可用”。  
**根因**：
- `CronTool` 依赖 `SetContext(channel, chatID)` 里的内部状态。
- Agent Loop 执行工具时没有注入当前消息上下文，导致 `CronTool` 拿不到会话信息。
- 该模式在并发请求下还存在上下文串线风险。  
**修复措施**：
- 新增工具运行时上下文：`pkg/tools/runtime_context.go`（`WithRuntimeContext` / `RuntimeContextFrom`）。
- Agent Loop 在每次工具调用前注入当前 `channel/chatID` 到 `context.Context`。
- `CronTool` 与 `MessageTool` 改为优先读取运行时上下文（保留 `SetContext` 兼容逻辑）。  
**验证**：
- 新增 `internal/agent/loop_test.go`，验证 Agent 工具调用创建 cron 任务时 payload 正确写入 `channel=telegram`、`to=chat-42`。
- `go test ./pkg/tools ./internal/agent` 全部通过。

## 2026-02-17 - DeepSeek 400：`messages[n]: missing field content`

**问题**：在多轮工具调用后，LLM 流式请求失败：`Failed to deserialize ... messages[n]: missing field content`。  
**根因**：
- OpenAI 兼容请求结构里 `chatMessage.Content` 使用了 `json:",omitempty"`。
- 当 assistant/tool 消息 `content=""`（常见于纯 tool_call 回合）时，序列化会直接省略 `content` 字段。
- DeepSeek 对 `messages[*].content` 字段是强校验，缺失会直接 400。  
**修复措施**：
- 将 `internal/providers/openai.go` 的 `chatMessage.Content` 改为 `json:"content"`，确保空字符串也会被发送。
- 新增测试 `internal/providers/openai_test.go`，覆盖“空 content 但必须保留字段”的序列化场景。  
**验证**：
- `go test ./internal/providers ./internal/agent` 通过。
- `go test ./...` 全量通过。

## 2026-02-17 - Cron 已触发但 Telegram 未收到（`chat_id` 丢失 + 出站错误静默）

**问题**：用户在 Telegram 里设置 `18:00` 提醒后，没有收到消息，看起来像“定时任务没执行”。  
**关键证据**（`/Users/lua/.maxclaw/logs/session.log`）：
- `2026/02/17 18:00:00.007320 inbound channel=telegram chat= sender=cron content="[telegram] [Cron Job: hello] hello"`
- `2026/02/17 18:00:02.019950 outbound channel=telegram chat= content="..."`

两条记录都显示 `chat=` 为空，说明任务确实执行了，但回发目标会话缺失，导致 Telegram 不可达。

**根因**：
1. `executeCronJob` 构造 cron 入站消息时把 `chatID` 写成空字符串（未使用 `job.Payload.To`）。
2. Gateway 出站发送链路对 `SendMessage` 返回错误静默处理，缺少失败日志，导致送达失败难以定位。
3. `message` 工具本身并非根因：`pkg/tools/message.go` 已要求 `channel/chat_id` 必填，不会在空目标下“假成功”。

**修复措施**：
- `internal/cli/cron.go`
  - cron 入站消息改为使用 `job.Payload.To` 作为 `chatID`。
  - 抽取 `buildCronUserMessage` / `enqueueCronJob`，保证投递参数一致。
- `internal/cli/gateway.go`
  - 可投递 cron 任务优先进入主消息总线（保持正常 channel/chat 路由）。
  - 出站处理新增空 `channel/chat_id` 校验与日志。
  - `SendMessage` 失败时记录错误，不再静默吞掉。
- `internal/cli/cron_test.go`, `internal/cli/gateway_test.go`
  - 增加投递与出站链路单测，覆盖成功发送、空 chat 丢弃、失败后继续处理。

**验证**：
- `go test ./internal/cli ./pkg/tools` 通过。
- `go test ./internal/cli` 通过。
- `make build` 通过。

## 2026-02-22 - SkillsView 无限循环请求 skills 接口

**问题**：打开技能市场页面后，浏览器开发者工具显示无限重复请求 `/api/skills` 接口，CPU 占用高。

**根因**：
- `SkillsView` 组件中 `useEffect` 依赖的 `fetchSkills` 使用了 `useCallback`
- `fetchSkills` 的依赖项 `[t]` 中的 `t` 函数（来自 `useTranslation()`）在每次渲染时引用都会变化
- 这导致 `fetchSkills` 不断重新创建，触发 `useEffect` 重复执行，形成无限循环

**修复措施**：
- 移除 `fetchSkills` 对 `t` 的依赖，错误信息使用硬编码字符串
- 给 `useEffect` 空依赖数组 `[]`，确保只在组件挂载时获取一次

```typescript
// 修复前（有问题的代码）
const fetchSkills = useCallback(async () => {
  // ...
  setError(err instanceof Error ? err.message : t('common.error'));
}, [t]);  // ❌ t 每次渲染都变化

useEffect(() => {
  void fetchSkills();
}, [fetchSkills]);  // ❌ fetchSkills 不断变化，导致无限循环

// 修复后
const fetchSkills = useCallback(async () => {
  // ...
  setError(err instanceof Error ? err.message : 'Failed to load skills');
}, []);  // ✅ 无依赖

useEffect(() => {
  void fetchSkills();
  // eslint-disable-next-line react-hooks/exhaustive-deps
}, []);  // ✅ 只在挂载时执行
```

**验证**：
- `cd electron && npm run build`
- 打开技能市场页面，确认 `/api/skills` 只请求一次
- 浏览器开发者工具 Network 面板无重复请求

**修复文件**：
- `electron/src/renderer/views/SkillsView.tsx`

---

## 2026-02-20 - Electron 安装后无法启动（`Electron failed to install correctly`）

**问题**：`cd electron && npm run dev` / `npm run start` 可完成前置构建，但 Electron 主进程启动时直接报错：
`Electron failed to install correctly, please delete node_modules/electron and try installing again`。  
同时在进入主进程后，Gateway 子进程也可能因为二进制路径错误报 `ENOENT`。

**根因**：
1. `electron` npm 包已安装，但 `node_modules/electron/path.txt` 与 `dist/` 不存在，说明 Electron 二进制下载未完成或中断；`npm install` 在 lock 不变时不会自动修复这个损坏状态。  
2. 主进程使用 `process.env.NODE_ENV === 'development'` 判断开发态，在当前 Vite build + `electron .` 链路下并不稳定，导致路径分支选错。  
3. `GatewayManager.getBinaryPath()` 的开发态相对路径层级错误，实际指向了不存在的位置，触发 `spawn ... ENOENT`。  

**修复措施**：
- 新增 `electron/scripts/ensure-electron.cjs`，在启动前检查 Electron 二进制是否完整，缺失时自动执行 `node node_modules/electron/install.js` 自愈。
- 将自愈流程接入 `electron/package.json`：`postinstall`、`electron:start`、`start` 均先执行 `npm run ensure:electron`。
- 新增 `electron/.npmrc` 的 Electron 镜像配置，降低二进制下载失败概率。
- `electron/src/main/index.ts` 改为 `app.isPackaged` 判断开发态，并支持 `ELECTRON_RENDERER_URL` / `VITE_DEV_SERVER_URL` 优先加载。
- `electron/src/main/gateway.ts` 重写 Gateway 二进制定位逻辑（开发态/打包态分离，支持候选路径与 `NANOBOT_BINARY_PATH` 覆盖），并在缺失时给出明确错误信息。  

**验证**：
- `cd electron && npm install --foreground-scripts`（确认可自动补齐 Electron 二进制）
- `cd electron && npm run dev`（不再出现 `Electron failed to install correctly`）
- `cd electron && npm run start`（可启动主进程）
- `cd electron && npm run build`
- `make build`

## 2026-02-23 - Agent 简单问候（`hi`）回复慢定位分析（仅记录，不改代码）

**问题**：用户反馈即使发送简单消息（如 `hi`），回复也明显偏慢，怀疑可能卡在 MCP 初始化、模型思考或其他链路。

**排查方式**：
- 查看运行日志：`~/.maxclaw/logs/session.log`、`~/.maxclaw/logs/tools.log`、`~/.maxclaw/logs/webui.log`
- 本地压测非流式 `/api/message`（连续 3 次 `hi`）
- 本地压测流式 `/api/message`（记录首个 `content_delta` 时间）

**关键证据**：
1. 非流式 `hi` 三次耗时（`time_starttransfer`）：
   - 13.248s
   - 16.813s
   - 29.886s
2. 对应会话日志显示单轮消息确有明显延迟：
   - `session.log` 中 `desktop:latency-check-*` 的入站/出站间隔分别约 13s / 17s / 30s。
3. 慢请求期间未出现新的 MCP 初始化告警；最近一次 MCP 连接告警为：
   - `tools.log`：`2026/02/23 07:47:55 ... context deadline exceeded`
4. 流式请求中，首个内容 token 也较慢：
   - `first_delta_sec = 8.654s`
   - `final_sec = 9.404s`
5. 部分 `hi` 流程存在额外工具回合（会进一步拉长总时延）：
   - `tools.log` 出现 `message -> read_file(memory) -> message` 序列。

**结论**：
1. 当前“`hi` 也慢”的主要瓶颈不是网络连接（本地 connect 几乎 0ms）。
2. 不是每次都卡在 MCP；MCP 问题主要体现在重启后连接阶段，且已有超时保护。
3. 当前主要耗时来自两部分叠加：
   - LLM 首 token 较慢（约 8~9s）
   - 部分简单问候触发了不必要工具调用，产生多轮往返，放大到 15~30s

**状态**：
- 本条为分析记录，按用户要求“先不修改代码”。
- 后续若要优化，优先方向是：减少简单问候场景下的工具回合、收窄默认工具策略。
