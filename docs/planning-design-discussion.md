# 规划系统设计讨论记录

## 日期：2026-02-25

## 背景

用户在分析一个复杂任务（下载腾讯5年财报、提取数据、制作图表）的执行过程时，发现了以下问题：

1. 没有触发任务规划和进度检查
2. 浏览器自动化没有处理 cookie 弹窗
3. 迭代太长超出后任务自动退出
4. 用户说"继续"时，agent 没有翻查历史记录，多次从零开始
5. 文件操作没有写到 session 目录下

## 设计目标

实现一个基于文件的规划系统，让 agent 能够：
- 自动分解复杂任务为可执行步骤
- 跟踪执行进度
- 支持中断后恢复（"继续"指令）
- 优雅处理迭代限制

## 关键设计决策

### 1. 存储位置：每个 session 一个 plan 文件

**选择**：`~/.maxclaw/workspace/.sessions/{session_key}/plan.json`

**理由**：
- 与 session 生命周期绑定，切换 session 自动切换 plan
- 实现简单，不需要额外的管理机制

**替代方案**：
- B. 专门的 plans 目录：支持多个并发 plan，但需要额外管理
- C. 两者结合：灵活但复杂度高

### 2. 执行顺序：顺序执行，无依赖

**选择**：线性步骤列表，按顺序执行

**理由**：
- 满足大多数场景，简化设计
- 复杂依赖关系可以用子步骤或嵌套 plan 实现

### 3. 生命周期管理：Agent 自主管理

**选择**：Agent 自动创建、更新、暂停/恢复 Plan

**理由**：
- LLM 经常忘记调用显式工具
- 进度更新是机械工作，不需要 LLM 参与
- 只需要在 system prompt 中告知 LLM 当前步骤

**替代方案**：显式工具调用（`create_plan`, `update_step` 等）

### 4. 触发条件：基于工具调用预测

**选择**：第一轮 LLM 响应包含工具调用时创建 Plan

**理由**：
- 简单任务（无工具调用）不会创建 Plan
- 延迟决策，直到确认需要多步骤执行

**替代方案**：
- 关键词启发式判断：可能误判或漏判
- 让 LLM 判断：增加延迟和成本
- 总是创建 Plan：简单任务也有 overhead

### 5. 步骤完成检测：混合策略

**最终方案**（三层优先级）：

```
1. 显式标记（立即生效）
   [Done], [完成], [Step Done], [步骤完成], etc.

2. 转换词检测（1 轮后生效）
   "现在", "接下来", "next", "then"...

3. 超时兜底（5 轮后生效）
```

**设计演进**：

| 版本 | 策略 | 问题 |
|------|------|------|
| v1 | 转换词检测（需 2 轮）+ 超时 10 轮 | 步骤推进太慢，用户体验差 |
| v2 | 降低阈值（1 轮 + 5 轮超时） | 仍依赖 LLM 输出特定词汇 |
| v3 | 添加显式标记 `[完成]` | LLM 可精确控制步骤边界 |

**参考 OpenClaw/OpenProse**：
- OpenClaw 使用声明式的 `.prose` 文件，用户显式定义步骤
- 没有自动步骤检测机制，依赖用户或 LLM 调用 `Task` 工具
- 我们的方案更自主，但增加了检测复杂度

## 实现细节

### Plan 数据结构

```json
{
  "id": "plan_1771945970128",
  "goal": "用户原始目标",
  "status": "running|paused|completed|failed",
  "steps": [
    {
      "id": "step_1",
      "description": "步骤描述",
      "status": "pending|running|completed|failed",
      "result": "执行结果摘要",
      "progress": {"current": 2, "total": 5}
    }
  ],
  "current_step_index": 0,
  "iteration_count": 15
}
```

### 关键实现点

1. **第 0 轮创建 Plan 时**：
   - 提取 LLM 输出中的 `[Step]` 声明
   - 第一个步骤自动设为 `running` 状态（因为已经在执行）

2. **每轮更新**：
   - 提取新的 `[Step]` 声明
   - 检测步骤完成（混合策略）
   - 保存 Plan 到文件
   - 更新 system message 中的 plan context

3. **"继续"指令处理**：
   - 检测 `继续/continue/resume` 等关键词
   - 加载 paused plan，注入进度摘要到用户消息
   - 从 `current_step_index` 恢复执行

4. **迭代限制处理**：
   - Plan 状态设为 `paused`
   - 返回包含进度摘要的消息
   - 提示用户输入"继续"恢复

## System Prompt 设计

```
## 当前任务规划

当前任务: {goal}
当前步骤 ({current}/{total}): {step_description}

步骤控制指令:
- 步骤完成后，说 "[完成]" 或 "[Done]" 推进到下一步
- 或使用 "现在..."、"接下来..." 等转换词
- 系统会自动跟踪进度并保存到 plan.json
```

## 遇到的问题与解决

### 问题 1：Plan 创建时 steps 为空

**现象**：LLM 没有在输出中声明 `[Step]`

**解决**：
- System prompt 添加提示："请先规划任务步骤，使用 [Step] 描述 格式..."
- 允许 LLM 在后续轮次中声明步骤

### 问题 2：步骤状态与实际执行不匹配

**现象**：第 0 轮已经在执行工具，但步骤状态是 `pending`

**解决**：创建 Plan 时，第一个步骤自动设为 `running` 状态

### 问题 3：步骤推进太慢

**现象**：需要 3-10 轮迭代才能推进到下一步

**解决**：
- 添加显式完成标记 `[完成]`/`[Done]`
- 降低转换词阈值（2→1 轮）
- 降低超时阈值（10→5 轮）

### 问题 4：JSON 文件没有更新

**原因**：`DetectCompletion` 条件未满足，没有调用 `CompleteCurrentStep`

**解决**：混合策略让 LLM 可以更灵活地标记步骤完成

## 未来改进方向

1. **工具边界启发式**：分析工具调用模式（读→写）判断步骤完成
2. **智能超时**：根据步骤复杂度动态调整超时阈值
3. **子 Plan 支持**：复杂步骤可以嵌套子任务
4. **Plan 模板**：常见任务预定义步骤序列
5. **Cookie 自动处理**：浏览器工具自动处理 cookie 弹窗

## 相关文件

- `internal/agent/plan.go` - Plan 数据结构和检测逻辑
- `internal/agent/loop.go` - Plan 生命周期集成
- `internal/agent/context.go` - System prompt 生成
- `docs/planning.md` - 用户文档
- `docs/plans/2026-02-24-file-based-planning-system-design.md` - 原始设计文档
- `docs/plans/2026-02-24-planning-system-implementation.md` - 实施计划

## 提交记录

```
bcfccde feat(planning): add Plan data structures and basic operations
1d6622f feat(planning): add plan persistence with Load/Save/Exists/Delete
0a2ab4f feat(planning): add step completion detection and continue intent recognition
5f5bf10 feat(planning): add progress summary generation
c43ad1d feat(planning): integrate PlanManager into AgentLoop
99b1eb5 feat(planning): implement plan lifecycle in ProcessMessage
c4a6967 docs: add planning system documentation, tests, and manual testing script
a624957 fix(planning): ensure plan context is updated each iteration
67f8341 fix(planning): mark first step as running on creation
7492adc feat(planning): implement hybrid step completion detection
```
