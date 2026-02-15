# Bug 修复记录

## 概述

本文档记录 nanobot-go 项目开发过程中发现的关键 bug 及其修复方案。

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
$ ./nanobot agent -m "搜索今日AI新闻"
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
`You are nanobot, a lightweight AI assistant with access to tools.

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
./nanobot agent -m "搜索今日AI新闻"

# 测试 list_dir
./nanobot agent -m "列出当前目录"

# 测试 read_file
./nanobot agent -m "查看 README.md 内容"

# 测试 exec
./nanobot agent -m "运行 pwd 命令"
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
