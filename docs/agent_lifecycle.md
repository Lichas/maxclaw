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
