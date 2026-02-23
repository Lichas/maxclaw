# maxclaw Agent 架构详解

本文档详细说明 maxclaw Agent 系统的核心架构组件及其数据流向。

## 架构概览

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              Agent 系统架构                                   │
└─────────────────────────────────────────────────────────────────────────────┘

  ┌──────────┐     ┌──────────────┐     ┌─────────────────┐     ┌──────────┐
  │ Channels │────▶│  MessageBus  │────▶│   AgentLoop     │────▶│  Output  │
  │ (多渠道)  │     │   (消息总线)   │     │   (核心循环)     │     │ (多渠道) │
  └──────────┘     └──────────────┘     └─────────────────┘     └──────────┘
                                                │
                          ┌─────────────────────┼─────────────────────┐
                          ▼                     ▼                     ▼
                   ┌────────────┐      ┌─────────────┐      ┌──────────────┐
                   │   Skills   │      │    Tools    │      │     MCP      │
                   │  (技能系统) │      │  (工具注册表) │      │ (外部工具接入) │
                   └────────────┘      └─────────────┘      └──────────────┘
                          │                     │                     │
                          ▼                     ▼                     ▼
                   ┌──────────────────────────────────────────────────────┐
                   │                    LLM Provider                       │
                   │              (OpenAI/Anthropic/DeepSeek等)            │
                   └──────────────────────────────────────────────────────┘
```

---

## 1. Agent Loop (核心循环)

**位置**: `internal/agent/loop.go`

Agent Loop 是整个系统的核心处理引擎，负责协调消息处理、LLM 调用和工具执行。

### 核心职责

| 职责 | 说明 |
|------|------|
| 消息循环 | 持续监听 MessageBus，处理入站消息 |
| 上下文构建 | 整合系统提示、技能、历史消息、心跳 |
| LLM 调用 | 调用 Provider 获取响应 |
| 工具执行 | 解析工具调用，执行并返回结果 |
| 流式输出 | 支持 SSE 结构化事件流 |
| 中断处理 | 支持生成过程中的用户打断 |

### 处理流程

```
┌────────────────────────────────────────────────────────────────┐
│                        Agent Loop 处理流程                       │
└────────────────────────────────────────────────────────────────┘

  1. 监听入站消息
     └─▶ MessageBus.ConsumeInbound()
         │
         ▼
  2. 构建完整上下文
     ├─▶ 系统提示 (system prompt)
     ├─▶ 长期记忆 (memory/MEMORY.md)
     ├─▶ 短期心跳 (memory/heartbeat.md)
     ├─▶ 技能注入 (selected skills)
     ├─▶ 会话历史 (session messages)
     └─▶ 用户输入 (user message)
         │
         ▼
  3. 调用 LLM Provider
     ├─▶ 发送 messages + tools
     ├─▶ 接收响应
     │   ├─ 纯文本响应 ──▶ 直接返回
     │   └─ 工具调用 ────▶ 继续步骤4
         │
         ▼
  4. 执行工具调用 (Tool Loop)
     ├─▶ 解析 tool_calls
     ├─▶ 并发/串行执行工具
     ├─▶ 收集 tool results
     ├─▶ 构建新消息列表
     └─▶ 回到步骤3 (最多20轮)
         │
         ▼
  5. 保存会话 & 发送响应
     ├─▶ 更新 session
     └─▶ MessageBus.PublishOutbound()
```

### 流式事件结构

```go
type StreamEvent struct {
    Type       string // status | tool_start | tool_result | content_delta | final | error
    Iteration  int    // 当前迭代轮次
    Message    string // 状态消息
    Delta      string // 文本增量
    ToolID     string // 工具调用ID
    ToolName   string // 工具名称
    ToolArgs   string // 工具参数
    Summary    string // 工具结果摘要
    ToolResult string // 完整工具结果
    Response   string // 最终响应
    Done       bool   // 是否完成
}
```

### 中断处理机制

```
┌─────────────────────────────────────────────────────────────┐
│                      智能打断流程                              │
└─────────────────────────────────────────────────────────────┘

  用户输入 "打断"
        │
        ▼
  ┌──────────────┐
  │ 意图分析器    │──▶ 识别为 "interrupt" 意图
  │ (intent.go)  │
  └──────────────┘
        │
        ▼
  ┌──────────────────┐
  │ 取消当前上下文    │──▶ context.Cancel()
  │ (interrupt.go)   │
  └──────────────────┘
        │
        ▼
  ┌──────────────────┐
  │ 两种处理模式      │
  │                  │
  │ 1. 打断重试       │──▶ 停止生成，重新回复
  │    (Enter)       │
  │                  │
  │ 2. 补充上下文     │──▶ 不打断，追加到下一轮
  │    (Shift+Enter) │
  └──────────────────┘
```

---

## 2. MessageBus (消息总线)

**位置**: `internal/bus/queue.go`

消息总线是系统的异步通信中枢，采用 Channel 实现的生产者-消费者模式。

### 设计特点

| 特点 | 实现 |
|------|------|
| 双通道设计 | inbound（入站）+ outbound（出站） |
| 缓冲队列 | 默认 100 条消息缓冲 |
| 线程安全 | 读写锁保护 |
| 优雅关闭 | 支持标记关闭状态 |

### 数据流向

```
┌──────────────────────────────────────────────────────────────┐
│                      消息总线数据流                             │
└──────────────────────────────────────────────────────────────┘

  入站消息 (Inbound)
  ══════════════════

  Telegram Bot ──┐
  Discord Bot  ──┼──▶ Channel.PublishInbound() ──▶ inbound chan
  WebSocket    ──┤         (生产者)                    (100缓冲)
  Email        ──┤                                       │
  ...          ──┘                                       ▼
                                                   AgentLoop
                                                   ConsumeInbound()
                                                      (消费者)


  出站消息 (Outbound)
  ═══════════════════

  AgentLoop ──▶ PublishOutbound() ──▶ outbound chan
  (生产者)                              (100缓冲)
                                            │
                    ┌───────────────────────┼───────────────────────┐
                    ▼                       ▼                       ▼
               Telegram Bot          Discord Bot              WebSocket
               发送回复               发送回复                  推送消息
```

### 消息类型

```go
// InboundMessage 入站消息
type InboundMessage struct {
    Channel   string // 来源渠道: telegram/discord/whatsapp/...
    ChatID    string // 会话标识
    SenderID  string // 发送者ID
    Content   string // 消息内容
    SessionID string // 会话ID
}

// OutboundMessage 出站消息
type OutboundMessage struct {
    Channel string // 目标渠道
    ChatID  string // 目标会话
    Content string // 响应内容
    HTML    bool   // 是否HTML格式
}
```

---

## 3. Tools (工具系统)

**位置**: `pkg/tools/`

工具系统是 Agent 与外部环境交互的接口层。

### 架构设计

```
┌─────────────────────────────────────────────────────────────┐
│                       工具系统架构                             │
└─────────────────────────────────────────────────────────────┘

                    ┌─────────────────────┐
                    │   tools.Registry    │
                    │     (工具注册表)     │
                    │                     │
                    │  map[name]Tool      │
                    │  - Register()       │
                    │  - Get()            │
                    │  - Execute()        │
                    └──────────┬──────────┘
                               │
           ┌───────────────────┼───────────────────┐
           ▼                   ▼                   ▼
    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐
    │ Built-in    │    │  Browser    │    │    MCP      │
    │ 内置工具     │    │ 浏览器工具   │    │  外部工具    │
    └─────────────┘    └─────────────┘    └─────────────┘
           │                   │                   │
    ┌──────┴──────┐    ┌──────┴──────┐    ┌──────┴──────┐
    ▼      ▼      ▼    ▼             │    ▼             │
 read  write  exec   web_fetch       │   filesystem     │
_file  _file  _shell   │              │    server        │
  │      │      │      │              │                  │
  ▼      ▼      ▼      ▼              │                  │
文件系统  编辑   Shell  Playwright ────┘                  │
操作                Chrome/CDP                          │
                                                       │
                                            ┌──────────┘
                                            ▼
                                    MCPConnector
                                    - Connect()
                                    - ListTools()
                                    - CallTool()
```

### 内置工具清单

| 工具 | 文件 | 功能 |
|------|------|------|
| read_file | `filesystem.go` | 读取文件内容 |
| write_file | `filesystem.go` | 写入文件 |
| edit_file | `filesystem.go` | 编辑文件（搜索替换） |
| list_dir | `filesystem.go` | 列出目录 |
| exec | `shell.go` | 执行 Shell 命令 |
| web_search | `web.go` | Web 搜索 |
| web_fetch | `web.go` | 网页抓取（HTTP/Chrome） |
| browser | `browser.go` | 浏览器自动化 |
| message | `message.go` | 发送消息 |
| spawn | `spawn.go` | 子代理任务 |
| cron | `cron.go` | 定时任务管理 |

### 工具执行流程

```
┌─────────────────────────────────────────────────────────────┐
│                      工具执行流程                              │
└─────────────────────────────────────────────────────────────┘

  AgentLoop
      │
      ▼
  registry.Execute(ctx, name, params)
      │
      ├─▶ Get(toolName) ──▶ 查找工具实例
      │
      ├─▶ ValidateParams() ──▶ 参数校验（JSON Schema）
      │
      └─▶ tool.Execute(ctx, params)
              │
              ├─▶ 提取运行时上下文 (WithRuntimeContext)
              │    - channel/chat_id
              │
              ├─▶ 执行具体逻辑
              │    - 文件操作（工作区限制）
              │    - Shell 命令（超时控制）
              │    - Web 请求（浏览器/CDP）
              │
              └─▶ 返回结果字符串
```

---

## 4. Skills (技能系统)

**位置**: `internal/skills/`

技能系统允许通过 Markdown 文件扩展 Agent 的专业知识。

### 工作原理

```
┌─────────────────────────────────────────────────────────────┐
│                      技能系统数据流                            │
└─────────────────────────────────────────────────────────────┘

  技能发现 (Discover)
  ═══════════════════

  workspace/skills/
  ├── react-best-practices.md
  ├── golang-style/
  │   └── SKILL.md
  └── kubernetes-troubleshooting.md
        │
        ▼
  loader.Discover(skillsDir)
        │
        ▼
  []Entry{
    {Name: "react-best-practices", Body: "..."},
    {Name: "golang-style", Body: "..."},
    {Name: "kubernetes-troubleshooting", Body: "..."},
  }


  技能选择 (Selection)
  ════════════════════

  用户消息: "帮我写 React 组件 @skill:react-best-practices"
                                              │
                                              ▼
                                       正则匹配 @skill:<name>
                                              │
                                              ▼
                                       加载指定技能内容
                                              │
                                              ▼
  AgentLoop ──▶ ContextBuilder ──▶ 注入到 System Prompt


  快捷语法
  ════════

  @skill:<name>  或  $<name>    ──▶ 加载指定技能
  @skill:all     或  $all        ──▶ 加载全部技能
  @skill:none    或  $none       ──▶ 不加载技能
```

### 技能文件格式

```markdown
---
name: React 最佳实践
description: React 组件开发规范和模式
---

# React 最佳实践

## 组件设计

- 优先使用函数组件
- Props 使用 TypeScript 接口定义
- 使用 React.FC 或显式返回类型

## 状态管理

...
```

---

## 5. MCP (Model Context Protocol)

**位置**: `pkg/tools/mcp.go`

MCP 支持接入外部工具服务器，扩展 Agent 能力边界。

### 架构设计

```
┌─────────────────────────────────────────────────────────────┐
│                      MCP 架构设计                              │
└─────────────────────────────────────────────────────────────┘

  config.json
  ═══════════

  {
    "tools": {
      "mcpServers": {
        "filesystem": {
          "command": "npx",
          "args": ["-y", "@modelcontextprotocol/server-filesystem", "/path"]
        },
        "remote": {
          "url": "https://mcp.example.com/sse"
        }
      }
    }
  }


  MCPConnector
  ════════════

  ┌──────────────────────────────────────────────┐
  │          MCPConnector (pkg/tools/mcp.go)      │
  │                                              │
  │  Connect()      ──▶ 连接所有配置的服务器      │
  │  ListTools()    ──▶ 获取可用工具列表          │
  │  CallTool()     ──▶ 调用远程工具              │
  │  Close()        ──▶ 关闭所有连接              │
  │                                              │
  │  超时保护:                                     │
  │  - Initialize: 10s                            │
  │  - ListTools:   10s                           │
  │  - CallTool:    60s                           │
  └──────────────────────────────────────────────┘
               │
    ┌──────────┴──────────┐
    ▼                     ▼
 STDIO Transport      HTTP/SSE Transport
 (本地进程)            (远程服务器)
    │                     │
    ▼                     ▼
 npx @server/...    https://mcp.example.com


  工具透传流程
  ═════════════

  AgentLoop ──▶ registry.GetDefinitions()
                    │
                    ├─▶ 内置工具定义
                    └─▶ MCPConnector.ListTools() ──▶ 外部工具定义
                    │
                    ▼
              合并后发送给 LLM
                    │
                    ▼
              LLM 返回 tool_calls
                    │
                    ▼
              registry.Execute()
                    │
                    ├─▶ 内置工具 ──▶ 直接执行
                    └─▶ MCP 工具 ──▶ MCPConnector.CallTool()
```

---

## 6. Subagent (子代理)

**位置**: `pkg/tools/spawn.go`

子代理允许 Agent 创建后台任务，实现并行处理和长时间运行任务。

### 工作原理

```
┌─────────────────────────────────────────────────────────────┐
│                       子代理机制                               │
└─────────────────────────────────────────────────────────────┘

  主 Agent
  ════════

  用户: "分析这个项目的代码质量"
      │
      ▼
  AgentLoop ──▶ 决定使用 spawn 工具
      │
      ▼
  spawn_tool.Execute()
      │
      ├─▶ 创建新的 AgentLoop 实例
      ├─▶ 在新 goroutine 中运行
      ├─▶ 分配独立 session_key
      │
      └─▶ 返回: task_id (立即返回，不等待)


  子 Agent 运行
  ═════════════

  ┌─────────────────────┐
  │   Subagent Task     │
  │   (独立 goroutine)  │
  │                     │
  │  1. 加载上下文       │
  │  2. 执行分析         │
  │  3. 生成报告         │
  │  4. 保存结果         │
  │                     │
  │  状态: pending      │
  │      → running      │
  │      → completed    │
  │      → failed       │
  └─────────────────────┘


  状态查询
  ════════

  用户: "查询任务 spawn-xxx-xxx 的状态"
      │
      ▼
  spawn_tool.List()
      │
      ▼
  返回任务状态和结果摘要
```

---

## 7. 数据流向总结

### 完整请求生命周期

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         完整请求生命周期示例                                   │
│                    (用户: "查看并总结我的周报.pdf")                            │
└─────────────────────────────────────────────────────────────────────────────┘

  1. 消息接收
  ════════════

  Telegram Bot ──▶ 收到消息 "查看并总结我的周报.pdf"
       │
       ▼
  channels.Telegram ──▶ MessageBus.PublishInbound()
       │
       ▼
  ┌────────────────────────────────────────────────────────────────┐
  │ InboundMessage{                                                │
  │   Channel: "telegram",                                         │
  │   ChatID:  "123456789",                                        │
  │   Content: "查看并总结我的周报.pdf"                               │
  │ }                                                              │
  └────────────────────────────────────────────────────────────────┘


  2. Agent 处理
  ══════════════

  AgentLoop.ConsumeInbound()
       │
       ├─▶ 2.1 加载技能 (@skill:none 默认加载全部)
       │
       ├─▶ 2.2 构建上下文
       │     - System Prompt (含工具描述)
       │     - Memory (MEMORY.md)
       │     - Heartbeat (heartbeat.md)
       │     - Session History
       │     - User Message
       │
       ├─▶ 2.3 LLM 调用 (Iteration 1)
       │     │
       │     └─▶ LLM 返回 tool_calls:
       │           - read_file(path="周报.pdf")
       │
       ├─▶ 2.4 执行工具
       │     │
       │     └─▶ tools.Registry.Execute("read_file", {...})
       │           │
       │           └─▶ read_file_tool.Execute()
       │               │
       │               ├─▶ 检查工作区限制
       │               ├─▶ 解析路径: <workspace>/.sessions/<session>/周报.pdf
       │               ├─▶ 读取文件内容
       │               └─▶ 返回内容
       │
       ├─▶ 2.5 LLM 调用 (Iteration 2)
       │     │
       │     ├─▶ 发送工具结果
       │     └─▶ LLM 返回文本响应: "本周工作完成度90%，主要完成了..."
       │
       ├─▶ 2.6 保存会话
       │     - 更新 session.json
       │     - 记录 Timeline
       │
       └─▶ 2.7 发送响应
             │
             ▼
       MessageBus.PublishOutbound()


  3. 消息发送
  ════════════

  ┌────────────────────────────────────────────────────────────────┐
  │ OutboundMessage{                                               │
  │   Channel: "telegram",                                         │
  │   ChatID:  "123456789",                                        │
  │   Content: "本周工作完成度90%，主要完成了..."                     │
  │ }                                                              │
  └────────────────────────────────────────────────────────────────┘
       │
       ▼
  channels.Telegram ──▶ SendMessage()
       │
       ▼
  Telegram API ──▶ 用户收到回复


  4. 数据持久化
  ══════════════

  Session 存储: ~/.maxclaw/workspace/.sessions/telegram-123456.json
  │
  ├─▶ Messages (用户输入 + AI 响应)
  ├─▶ Timeline (执行轨迹: tool_start → tool_result → content)
  └─▶ Metadata (模型、时间戳等)
```

---

## 8. 关键设计决策

### 8.1 为什么使用 MessageBus？

| 优势 | 说明 |
|------|------|
| 解耦 | Channels 和 Agent 不直接依赖 |
| 异步 | 消息缓冲，削峰填谷 |
| 可扩展 | 新增渠道无需修改 Agent |
| 容错 | 渠道故障不影响其他组件 |

### 8.2 工具注册表 vs 硬编码？

```go
// 硬编码（不推荐）
if toolName == "read_file" {
    result = readFile(params)
} else if toolName == "exec" {
    result = execCommand(params)
}

// 注册表模式（实际使用）
tool, exists := registry.Get(toolName)
result = tool.Execute(ctx, params)
```

优势：
- 动态注册（MCP 工具）
- 统一接口
- 便于测试

### 8.3 会话隔离机制

```
Session Key 格式: <channel>:<chat_id>

Examples:
- telegram:123456789
- discord:987654321
- desktop:1234567890123
- webui:default
```

每个 Session 独立：
- 消息历史
- 文件存储路径 (`<workspace>/.sessions/<session_key>/`)
- 终端会话
- Cron 任务上下文

---

## 9. 扩展指南

### 添加新渠道

1. 实现 `channels.Channel` 接口
2. 在 Gateway 中注册
3. 实现消息转换逻辑

### 添加新工具

1. 实现 `tools.Tool` 接口
2. 在 `AgentLoop.initTools()` 中注册
3. 更新系统提示描述

### 添加 MCP 服务器

1. 在 `config.json` 中配置
2. `MCPConnector` 自动连接
3. 工具自动透传给 LLM

---

## 10. 文件索引

| 组件 | 关键文件 |
|------|----------|
| Agent Loop | `internal/agent/loop.go`, `internal/agent/interrupt.go`, `internal/agent/intent.go` |
| MessageBus | `internal/bus/queue.go`, `internal/bus/events.go` |
| Tools | `pkg/tools/registry.go`, `pkg/tools/base.go`, `pkg/tools/*.go` |
| Skills | `internal/skills/loader.go`, `internal/skills/state.go` |
| MCP | `pkg/tools/mcp.go` |
| Context | `internal/agent/context.go` |
| Session | `internal/session/manager.go` |
