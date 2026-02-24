# 会话任务状态管理设计讨论

## 背景问题

当前 maxclaw 的会话管理存在以下问题：

1. **任务边界不明确**：一个 Session 里怎么标记消息完成了？
2. **复杂任务无法暂停**：深度调研任务需要二次澄清确认时，怎么标记任务成功完成？
3. **新任务识别困难**：任务成功后继续讨论，怎么标记新任务开始和成功完成？
4. **等待状态缺失**：是否标记 Session 等待用户提供澄清信息的状态？

## 当前机制分析

### 会话结构 (`internal/session/manager.go`)

```go
type Message struct {
    Role      string          `json:"role"`
    Content   string          `json:"content"`
    Timeline  []TimelineEntry `json:"timeline,omitempty"`
    Timestamp time.Time       `json:"timestamp"`
}

type Session struct {
    Key              string    `json:"key"`
    Messages         []Message `json:"messages"`
    LastConsolidated int       `json:"lastConsolidated,omitempty"`
}
```

- `Session` 只包含扁平的消息列表，**没有任务概念**
- `Message` 只有 role/content/timeline/timestamp，**没有任务关联**

### Agent 循环 (`internal/agent/loop.go`)

- 通过 LLM 是否返回 tool calls 来判断是否继续
- 当 LLM 不调用工具直接回复时，认为"完成"
- **没有显式的"任务完成"标记**

### 中断机制 (`internal/agent/interrupt.go`)

- 支持 Cancel/Append 两种中断模式
- 可以打断正在执行的任务
- **但没有"等待澄清"状态**

## 推荐设计方案

核心思路：**让 LLM 显式标记任务边界和状态**，而不是隐式推断。

### 1. 任务状态定义

```go
type TaskState string

const (
    TaskIdle                   TaskState = "idle"                    // 无活跃任务
    TaskRunning                TaskState = "running"                 // 执行中
    TaskWaitingClarification   TaskState = "waiting_clarification"   // 等待用户澄清
    TaskCompleted              TaskState = "completed"               // 已完成
    TaskFailed                 TaskState = "failed"                  // 失败
)

type Task struct {
    ID           string     `json:"id"`
    ParentID     string     `json:"parent_id,omitempty"`   // 支持子任务
    Goal         string     `json:"goal"`                  // 任务目标
    State        TaskState  `json:"state"`
    StartTime    time.Time  `json:"start_time"`
    EndTime      *time.Time `json:"end_time,omitempty"`
    MessageRange [2]int     `json:"message_range"`         // 关联的消息索引 [start, end]
}
```

### 2. 新增任务管理工具

让 LLM 可以显式控制任务状态：

#### task_complete - 标记当前任务完成

```go
func (t *TaskCompleteTool) Execute(ctx context.Context, params map[string]interface{}) (string, error) {
    summary := params["summary"].(string)
    // 1. 更新当前任务状态为 completed
    // 2. 记录完成时间
    // 3. 可选：生成任务摘要存入记忆
    return fmt.Sprintf("任务已标记完成: %s", summary), nil
}
```

#### task_request_clarification - 请求用户澄清

```go
func (t *TaskClarifyTool) Execute(ctx context.Context, params map[string]interface{}) (string, error) {
    question := params["question"].(string)
    // 1. 设置任务状态为 waiting_clarification
    // 2. 向用户发送问题
    // 3. 暂停 Agent 循环等待回复
    return question, nil
}
```

#### task_start - 显式开始新任务

```go
func (t *TaskStartTool) Execute(ctx context.Context, params map[string]interface{}) (string, error) {
    goal := params["goal"].(string)
    // 1. 如有活跃任务，先标记完成
    // 2. 创建新任务
    // 3. 返回任务ID
    return fmt.Sprintf("开始新任务: %s", goal), nil
}
```

### 3. Session 中跟踪任务状态

```go
type Session struct {
    Key           string    `json:"key"`
    Messages      []Message `json:"messages"`
    Tasks         []Task    `json:"tasks"`            // 任务历史
    CurrentTaskID string    `json:"current_task_id"`  // 当前活跃任务
}
```

### 4. Agent Loop 修改

在 `processMessageWithIC` 中：

```go
// 检查当前任务状态
sess := a.sessions.GetOrCreate(msg.SessionKey)

switch sess.GetCurrentTaskState() {
case TaskWaitingClarification:
    // 用户回复视为澄清内容
    // 恢复任务继续执行
    sess.UpdateCurrentTask(TaskRunning)

case TaskRunning:
    // 检查用户意图是否是新任务
    intent := a.intentAnalyzer.Analyze(msg.Content, "")
    if intent.Intent == IntentNewTopic {
        // 自动结束当前任务，开始新任务
        sess.CompleteCurrentTask("用户开始新话题")
    }
}

// 添加用户消息
sess.AddMessage("user", msg.Content)

// LLM 循环中检测任务工具调用
for i := 0; i < a.MaxIterations; i++ {
    // ... 原有逻辑 ...

    // 检查是否调用了任务管理工具
    for _, tc := range toolCalls {
        switch tc.Function.Name {
        case "task_complete", "task_request_clarification":
            // 更新会话中的任务状态
            sess.UpdateTaskFromToolCall(tc)
        }
    }
}
```

### 5. UI 展示

在 Electron 前端展示任务状态：

```typescript
interface TaskInfo {
  id: string;
  goal: string;
  state: 'running' | 'waiting' | 'completed';
}

// 会话列表显示当前任务状态
// - 执行中:  正在调研 xxx
// - 等待澄清:  等待确认: xxx
// - 已完成:  任务完成
```

## 实现优先级建议

| 优先级 | 功能 | 说明 |
|--------|------|------|
| P0 | 添加 `task_complete` 工具 | 让 LLM 显式标记任务完成 |
| P1 | Session 中跟踪当前任务 | 存储任务状态和边界 |
| P1 | `task_request_clarification` | 支持需要确认的场景 |
| P2 | `task_start` 工具 | 显式开始新任务（也可自动检测） |
| P2 | UI 状态展示 | 前端显示任务状态 |

## 设计优势

1. **显式优于隐式**：LLM 明确知道何时任务完成，而不是靠"没有 tool call"推断
2. **支持复杂工作流**：深度调研任务可以中途暂停等待用户确认
3. **向后兼容**：不破坏现有会话机制，任务信息作为附加元数据
4. **状态可追溯**：任务历史可查询，支持任务级别的记忆和复盘

## 相关文件

- `internal/session/manager.go` - 会话管理
- `internal/agent/loop.go` - Agent 循环
- `internal/agent/interrupt.go` - 中断机制
- `internal/agent/intent.go` - 意图分析
