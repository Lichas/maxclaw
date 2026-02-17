# Changelog

## [Unreleased]

### 新增功能

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
