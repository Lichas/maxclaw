# File-Based Planning System Design

## Overview

为 maxclaw Agent 增加基于文件的规划系统，解决复杂长任务的自主性问题：
- 自动分解任务为可执行的步骤
- 跟踪执行进度，支持中断后恢复
- 迭代用完时优雅暂停，用户说"继续"后从断点恢复

## Design Decisions

| 决策点 | 选择 | 理由 |
|--------|------|------|
| 存储位置 | 每个 session 一个 plan 文件 | 与 session 生命周期绑定，切换 session 自动切换 plan |
| 执行顺序 | 顺序执行，无依赖 | 简化设计，满足大多数场景 |
| 生命周期 | Agent 自主管理 | LLM 经常忘记调用工具，机械工作由 Agent 处理 |
| 触发条件 | 基于工具调用预测 | 第一次调用工具说明任务不简单，需要规划 |
| 步骤完成检测 | 转换词检测 + 超时兜底 | 检测"现在"、"接下来"等词，超时自动推进 |

## Data Structure

### Plan Schema

```json
{
  "id": "plan_abc123",
  "goal": "下载分析腾讯最近5年年度财报PDF，提取关键财务指标，制作成图表",
  "status": "running",
  "created_at": "2026-02-24T10:00:00Z",
  "updated_at": "2026-02-24T10:30:00Z",
  "steps": [
    {
      "id": "step_1",
      "description": "搜索腾讯2020-2024年财报PDF链接",
      "status": "completed",
      "result": "找到5个PDF链接: tencent_2024.pdf, tencent_2023.pdf...",
      "progress": {"current": 5, "total": 5},
      "started_at": "2026-02-24T10:00:05Z",
      "completed_at": "2026-02-24T10:02:30Z"
    },
    {
      "id": "step_2",
      "description": "下载所有PDF文件到本地",
      "status": "running",
      "result": "",
      "progress": {"current": 2, "total": 5},
      "started_at": "2026-02-24T10:02:35Z",
      "completed_at": null
    }
  ],
  "current_step_index": 1,
  "iteration_count": 15
}
```

### Field Definitions

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Plan 唯一标识，格式 `plan_<random>` |
| `goal` | string | 用户原始目标描述 |
| `status` | enum | `pending`, `running`, `paused`, `completed`, `failed` |
| `steps` | array | 任务步骤列表 |
| `steps[].id` | string | 步骤 ID，格式 `step_<index>` |
| `steps[].description` | string | 步骤描述（由 LLM 生成） |
| `steps[].status` | enum | `pending`, `running`, `completed`, `failed` |
| `steps[].result` | string | 步骤执行结果摘要（纯文本） |
| `steps[].progress` | object | 可选，用于显示子进度 `{current, total}` |
| `current_step_index` | int | 当前执行到的步骤索引 |
| `iteration_count` | int | 已使用的迭代次数 |

## Workflow

### 1. Plan Creation

触发条件：**第一轮 LLM 响应包含工具调用**

```go
// 在 Agent Loop 中
if firstIteration && len(toolCalls) > 0 && !planExists {
    plan := createPlan(msg.Content) // 基于用户消息创建
    savePlan(sessionKey, plan)
    updateSystemPromptWithPlan(plan) // 告知 LLM 当前计划
}
```

Plan 初始结构：
- `goal` = 用户消息
- `steps` = 空数组（由 LLM 在后续迭代中填充）
- `current_step_index` = 0
- `status` = "running"

### 2. Step Progression

每个迭代后，Agent 自动检测步骤是否完成：

```go
func detectStepCompletion(llmOutput string, currentStep *Step, iterationInStep int) bool {
    // 策略 1: 转换词检测
    transitionWords := []string{"现在", "接下来", "然后", "继续", "开始", "现在让我"}
    if containsAny(llmOutput, transitionWords) && iterationInStep >= 2 {
        return true
    }

    // 策略 2: 超时兜底（一个步骤最多 10 轮）
    if iterationInStep >= 10 {
        return true
    }

    return false
}
```

步骤推进逻辑：
```go
if detectStepCompletion(content, currentStep, iterationsInCurrentStep) {
    currentStep.status = "completed"
    currentStep.completed_at = now()
    currentStep.result = summarizeStepResult(timeline)

    plan.current_step_index++
    if plan.current_step_index < len(plan.steps) {
        nextStep := plan.steps[plan.current_step_index]
        nextStep.status = "running"
        nextStep.started_at = now()
    }
}
```

### 3. Step Creation

LLM 可以在任何时候创建新步骤：

```go
// 检测 LLM 输出中的步骤声明
if matches := extractStepDeclaration(content); matches != nil {
    for _, desc := range matches {
        appendStep(plan, desc)
    }
}
```

步骤声明格式（由 system prompt 约定）：
```
[Step] 下载2024年财报PDF
[Step] 提取关键财务指标
```

### 4. Iteration Limit Handling

当达到 `MaxIterations` 时：

```go
if iteration >= maxIterations {
    plan.status = "paused"
    plan.iteration_count = iteration
    savePlan(sessionKey, plan)

    return fmt.Sprintf(
        "任务执行中（%d/%d 步已完成）。输入'继续'以恢复执行。",
        plan.current_step_index, len(plan.steps)
    )
}
```

### 5. Resume on "Continue"

检测"继续"指令：

```go
func isContinueIntent(content string) bool {
    content = strings.ToLower(strings.TrimSpace(content))
    return content == "继续" || content == "continue" ||
           content == "go on" || strings.Contains(content, "继续执行")
}
```

恢复逻辑：
```go
if isContinueIntent(msg.Content) && planExists && plan.status == "paused" {
    plan.status = "running"

    // 生成进度摘要，加入 context
    summary := generateProgressSummary(plan)
    msg.Content = fmt.Sprintf("[恢复任务] %s\n\n用户指令: %s", summary, msg.Content)

    // 继续从 current_step_index 执行
}
```

进度摘要示例：
```
当前任务: 下载分析腾讯最近5年年度财报PDF
进度: 2/5 步已完成
当前步骤: 下载所有PDF文件到本地 (2/5 已下载)
已用迭代: 15/20

历史步骤:
1. ✓ 搜索腾讯2020-2024年财报PDF链接
2. ⏳ 下载所有PDF文件到本地 (进行中)
3. ⏸ 提取关键财务指标 (待执行)
4. ⏸ 制作数据图表 (待执行)
```

## Integration with Agent Loop

### File Locations

```
~/.maxclaw/workspace/
└── .sessions/
    └── {session_key}/
        ├── plan.json          # Plan 文件
        └── ...                # 其他 session 文件
```

### Code Changes

1. **新增 `internal/agent/plan.go`**
   - `Plan` 结构体定义
   - `PlanManager` 管理 Plan 生命周期
   - `detectStepCompletion()` 步骤完成检测
   - `isContinueIntent()` 继续指令检测

2. **修改 `internal/agent/loop.go`**
   - 初始化时检查是否存在 paused plan
   - 第一轮工具调用时创建 plan
   - 每个迭代后更新 plan
   - 迭代用完时暂停 plan
   - 检测 continue intent 并恢复

3. **修改 System Prompt**
   - 告知 LLM 当前步骤信息
   - 指导 LLM 使用 `[Step]` 语法声明步骤

### System Prompt Addition

```
当前任务规划信息：
- 目标: {plan.goal}
- 当前步骤: {current_step.description} ({current_step_index + 1}/{len(steps)})
- 步骤进度: {current_step.progress.current}/{current_step.progress.total}

你可以使用以下格式声明新步骤：
[Step] 步骤描述

完成当前步骤后，系统会自动推进到下一步。
```

## Error Handling

| 场景 | 处理策略 |
|------|----------|
| Plan 文件读写失败 | 记录错误，降级为无规划模式继续执行 |
| Plan JSON 损坏 | 备份旧文件，创建新 plan |
| 步骤检测误判 | 用户可以通过插话说"还没完成"来纠正 |
| 任务完全失败 | plan.status = "failed"，保留现场供分析 |

## Future Improvements (Out of Scope)

- Step dependencies（依赖关系）
- Sub-plan（子任务嵌套）
- Plan 模板（常见任务预定义步骤）
- Plan 历史分析（优化步骤预测）
