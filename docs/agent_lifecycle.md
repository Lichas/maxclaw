# MaxClaw Agent Lifecycle (验证→反思→适应→持久化→进化)

MaxClaw 实现了完整的 Agent 生命周期循环系统，从 Hermes-Agent 迁移并适配到 Go 语言环境。

## 架构概览

```
┌─────────────────────────────────────────────────────────────────┐
│                     Agent Lifecycle Cycle                        │
├─────────────────────────────────────────────────────────────────┤
│  ┌──────────┐    ┌──────────┐    ┌──────────┐    ┌──────────┐  │
│  │Verification│──→│ Reflection│──→│ Adaptation│──→│Persistence│  │
│  │  (验证)    │    │  (反思)   │    │  (适应)   │    │ (持久化)  │  │
│  └──────────┘    └──────────┘    └──────────┘    └──────────┘  │
│        ↑                                               │        │
│        └───────────────────────────────────────────────┘        │
│                          Evolution (进化)                        │
└─────────────────────────────────────────────────────────────────┘
```

## 五层循环系统

### 1. 验证层 (Verification Layer)

**文件**: `internal/agent/error_classifier.go`

功能：
- 结构化错误分类 (`ErrorClassifier`)
- 支持的错误类型：Auth, Billing, RateLimit, ContextOverflow, ModelNotFound, Timeout 等
- 自动识别可恢复的错误
- 提供恢复策略建议

```go
classifiedErr := errorClassifier.ClassifyError(
    err,
    provider,
    model,
    approxTokens,
    contextLength,
    numMessages,
)
// classifiedErr.Retryable - 是否可重试
// classifiedErr.ShouldCompress - 是否需要压缩
// classifiedErr.ShouldFallback - 是否需要回退
```

### 2. 反思层 (Reflection Layer)

**文件**: `internal/agent/context_compressor.go`, `internal/agent/insights.go`

#### 上下文压缩 (ContextCompressor)
- 自动检测上下文大小并触发压缩
- 迭代式摘要更新
- 结构化摘要模板 (Goal, Progress, Decisions, Files, Next Steps)
- 工具调用/结果对完整性维护

#### 会话洞察 (InsightsEngine)
- Token 使用统计
- 成本估算
- 工具使用分析
- 活动模式识别
- 模型性能对比

```go
// 压缩上下文
result, err := compressor.Compress(ctx, messages, systemPrompt)

// 生成洞察报告
report := insightsEngine.Generate(days, source)
```

### 3. 适应层 (Adaptation Layer)

**文件**: `internal/agent/adaptation.go`

功能：
- 多层级模型回退 (`FallbackChain`)
- 上下文长度自适应调整
- 运行时参数动态修改
- 凭证轮换支持
- 智能重试退避

```go
// 添加回退提供商
adaptationManager.AddFallbackProvider(FallbackProvider{
    Provider: fallbackProvider,
    Model:    "gpt-3.5-turbo",
    Priority: 1,
})

// 处理错误
action := adaptationManager.HandleError(ctx, classifiedErr)
// 可能的动作: Retry, Fallback, Compress, AdjustContext, Abort
```

### 4. 持久化层 (Persistence Layer)

**文件**: `internal/agent/checkpoint.go`

功能：
- 文件系统检查点快照 (`CheckpointManager`)
- 会话状态保存/恢复
- 工具执行结果持久化
- 会话历史归档

```go
// 保存检查点
checkpoint, err := checkpointManager.Save(sessionKey, messages, systemPrompt, iteration)

// 加载最新检查点
checkpoint, err := checkpointManager.LoadLatest(sessionKey)
```

### 5. 进化层 (Evolution Layer)

**文件**: `internal/agent/evolution.go`

功能：
- 错误模式识别
- 恢复策略效果跟踪
- 模型性能历史记录
- 自适应阈值调整
- 学习参数持久化

```go
// 开始会话跟踪
evolutionTracker.StartSession(model, provider)

// 记录成功调用
evolutionTracker.RecordAPICall(model, provider, tokens, latency)

// 记录错误
evolutionTracker.RecordError(errorReason, errorMsg, retryCount, recovered, recoveryTime)

// 获取恢复建议
recommendation := evolutionTracker.GetRecoveryRecommendation(errorReason)
```

## 生命周期集成

**文件**: `internal/agent/lifecycle.go`

`AgentLifecycle` 是五层循环的统一管理器，提供简化的 API：

```go
// 初始化
lifecycle := NewAgentLifecycle(workspace, primaryRuntime)
lifecycle.InitializeCompression(model, baseURL, apiKey, provider, contextLength)

// 会话生命周期
lifecycle.StartSession(sessionKey, model, provider)
defer lifecycle.EndSession(sess, history)

// API 调用处理
err := lifecycle.BeforeAPICall(ctx, messages)
action, newRuntime, err := lifecycle.HandleAPIError(ctx, err, provider, model, tokens, numMessages)
lifecycle.RecordSuccess(model, provider, tokens, latency)

// 检查点
lifecycle.SaveCheckpoint(sessionKey, messages, systemPrompt, iteration)

// 上下文压缩
compressed, err := lifecycle.CompressContext(ctx, messages, systemPrompt)
```

## AgentLoop 集成

**文件**: `internal/agent/loop.go`

AgentLoop 已集成生命周期管理：

```go
type AgentLoop struct {
    // ... 现有字段 ...
    
    // 生命周期循环层
    Lifecycle *AgentLifecycle
}

// 使用生命周期管理
func (a *AgentLoop) processMessageWithIC(...) {
    // 开始会话生命周期
    a.StartSessionLifecycle(msg.SessionKey)
    defer a.EndSessionLifecycle(sess)
    
    // 处理 API 错误
    shouldRetry, err := a.HandleAPIErrorWithLifecycle(ctx, err, approxTokens, numMessages)
    
    // 记录成功
    a.RecordAPICallSuccess(tokens, latency)
    
    // 保存检查点
    a.SaveCheckpoint(msg.SessionKey, messages, systemPrompt, iteration)
}
```

## 配置选项

生命周期功能可以通过以下方式配置：

```go
lifecycle := &AgentLifecycle{
    Enabled:           true,   // 启用生命周期管理
    EnableCompression: true,   // 启用上下文压缩
    EnableFallbacks:   true,   // 启用模型回退
    EnableCheckpoints: true,   // 启用检查点
    EnableEvolution:   true,   // 启用进化跟踪
}
```

## 命令行接口

未来可以通过 CLI 命令管理生命周期：

```bash
# 查看生命周期统计
maxclaw lifecycle stats

# 查看压缩状态
maxclaw lifecycle compression status

# 手动触发压缩
maxclaw lifecycle compression trigger

# 查看检查点列表
maxclaw lifecycle checkpoints list

# 恢复到检查点
maxclaw lifecycle checkpoints restore <checkpoint-id>

# 查看洞察报告
maxclaw lifecycle insights --days 30

# 管理回退链
maxclaw lifecycle fallback add <provider> <model>
maxclaw lifecycle fallback list
```

## 性能考虑

- **上下文压缩**: 仅在超过阈值时触发，使用异步摘要生成
- **检查点**: 每回合最多一个快照，自动清理旧检查点
- **进化跟踪**: 批量更新，减少磁盘 I/O
- **错误分类**: 内存中的模式匹配，O(1) 复杂度

## 故障排除

### 上下文压缩不触发
- 检查 `EnableCompression` 是否启用
- 检查 `ContextCompressor.ThresholdPercent` 设置
- 检查当前 token 数是否超过阈值

### 回退不生效
- 检查 `EnableFallbacks` 是否启用
- 确认已添加回退提供商
- 检查错误是否被分类为需要回退

### 检查点未保存
- 检查 `EnableCheckpoints` 是否启用
- 检查工作目录权限
- 检查磁盘空间

## 与 Hermes-Agent 的对比

| 功能 | Hermes-Agent (Python) | MaxClaw (Go) |
|------|----------------------|--------------|
| 错误分类 | ✅ error_classifier.py | ✅ error_classifier.go |
| 上下文压缩 | ✅ context_compressor.py | ✅ context_compressor.go |
| 会话洞察 | ✅ insights.py | ✅ insights.go |
| 模型回退 | ✅ 运行时回退链 | ✅ AdaptationManager |
| 检查点 | ✅ CheckpointManager | ✅ CheckpointManager |
| 进化跟踪 | ✅ 隐式实现 | ✅ EvolutionTracker |
| 生命周期集成 | ✅ run_agent.py | ✅ AgentLifecycle |

## 未来扩展

- [ ] 支持更多 LLM 提供商的错误模式
- [ ] 智能压缩模型选择
- [ ] 分布式检查点存储
- [ ] 进化数据的机器学习分析
- [ ] 自适应阈值自动调优


## 第六层：用户反馈学习 (User Feedback Loop)

**文件**: 
- `internal/agent/feedback_detector.go` - 反馈检测（三层架构）
- `internal/agent/feedback_learner.go` - 反馈学习与持久化

功能：
- **三层检测架构**：规则引擎 → 上下文模式 → LLM 语义分析
- **多语言支持**：中文 + 英文正则匹配
- **自动提取教训**：从用户调教中提取可复用的知识
- **持久化到 MEMORY.md**：长期记忆，跨会话生效

### 三层反馈检测架构

```
┌─────────────────────────────────────────────────────────────────┐
│                    User Feedback Detection                       │
├─────────────────────────────────────────────────────────────────┤
│  Layer 1: Rule Engine (100% 消息, 零成本)                        │
│  ├── 直接否定: "不对"/"错了"/"wrong"/"incorrect"                 │
│  ├── 修正信号: "应该用 X 而不是 Y"/"should use X instead of Y"    │
│  ├── 肯定信号: "好的"/"完美"/"good"/"perfect"                    │
│  └── 否定+肯定翻转: "not good" → 负面                            │
├─────────────────────────────────────────────────────────────────┤
│  Layer 2: Contextual Patterns (零成本)                           │
│  ├── 重复抱怨检测: "还是不对" → 累积不满                         │
│  ├── 回避确认: Agent问"可以吗?" → 用户说"但是..."               │
│  ├── 教学模式: "你应该先...然后..." → 强烈不满                  │
│  └── 质疑方式: "为什么不..." → 隐性不满                         │
├─────────────────────────────────────────────────────────────────┤
│  Layer 3: LLM Semantic Analysis (成本控制)                       │
│  ├── 触发条件: 包含模糊词("感觉"/"seems")或对比词("但是"/"but")   │
│  ├── 采样率: 30% 的模糊消息 + 10% 随机采样                       │
│  ├── 成本: ~$0.1/天 (每小时最多 50 次调用)                      │
│  └── 语义理解: "感觉性能会不会有问题" → 隐性不满                  │
└─────────────────────────────────────────────────────────────────┘
```

### 反馈类型

```go
type FeedbackType int

const (
    FeedbackPositive      // 用户满意
    FeedbackNegative      // 用户不满
    FeedbackCorrection    // 用户给出具体修正
    FeedbackClarification // 用户澄清意图
    FeedbackQuestion      // 用户提问（可能是困惑）
    FeedbackNeutral       // 无明显情绪
)
```

### 使用示例

```go
// 初始化反馈检测器（可选 LLM 增强）
agentLoop.InitializeFeedbackDetector(llmProvider, "gpt-3.5-turbo")

// 用户发送消息后检测反馈
result := agentLoop.DetectUserFeedback(ctx, userMsg, lastAgentOutput)

switch result.Type {
case FeedbackNegative, FeedbackCorrection:
    // 记录学习
    lesson := agentLoop.LearnFromFeedback(result, taskContext, agentOutput, userMsg)
    
    //  lesson 会被写入 MEMORY.md:
    //  ## User Feedback Lesson [lesson_ab12]
    //  - **Task Type**: refactoring
    //  - **Issue Type**: implementation
    //  - **Lesson**: User prefers Promise.all over serial forEach
    //  - **Occurrences**: 1
    //  - **Learned**: 2024-01-15
}

// 下次类似任务前，自动注入学到的知识
enhancedPrompt := agentLoop.BuildFeedbackEnhancedPrompt("refactoring")
// 返回: "[Previous Feedback Lessons]\n1. implementation (3 times): User prefers Promise.all..."
```

### 实际工作流示例

**第 1 次重构任务**:
```
Agent: "我按顺序用 forEach 重构了这些文件"
User:  "不对，应该用 Promise.all 并行处理，这样太慢了"

[FeedbackDetector] 
- Layer 1: 匹配 "不对" + "应该...而不是" → FeedbackCorrection, 置信度 95%

[FeedbackLearner]
- 提取教训: "User prefers Promise.all over serial forEach for batch operations"
- 写入 MEMORY.md
```

**第 2 次类似任务（一周后）**:
```
[BuildSystemPrompt] 自动读取 MEMORY.md，注入:
"[Previous Feedback Lessons]
1. implementation (1 time): User prefers Promise.all over serial forEach..."

Agent: "我将使用 Promise.all 并行处理这些文件..."
User:  "👍 完美"

[FeedbackDetector] → FeedbackPositive
```

### 成本优化

| 层级 | 覆盖率 | 成本 | 延迟 |
|-----|-------|------|-----|
| 规则引擎 | ~70% | $0 | <1ms |
| 上下文模式 | ~15% | $0 | <1ms |
| LLM 分析 | ~15% | ~$0.1/天 | ~500ms |

### 配置

```go
lifecycle := NewAgentLifecycle(workspace, config)
lifecycle.EnableFeedback = true

// 可选：初始化 LLM 增强检测
lifecycle.InitializeFeedback(llmProvider, "gpt-3.5-turbo")
```

### 与 Evolution 层的区别

| 维度 | Evolution 层 | User Feedback Loop |
|-----|-------------|-------------------|
| **触发条件** | API 错误、工具失败 | 用户明确表达不满或修正 |
| **学习内容** | 系统恢复策略 | 用户偏好和期望 |
| **持久化** | evolution/state.json | MEMORY.md（用户可见） |
| **应用时机** | 错误恢复时 | 任务开始前（预防） |
| **数据隐私** | 系统内部 | 可编辑、可删除 |
