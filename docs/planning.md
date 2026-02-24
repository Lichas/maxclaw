# Task Planning System

maxclaw now supports automatic task planning for complex multi-step tasks.

## How It Works

When you give maxclaw a complex task that requires multiple steps (like downloading files, analyzing data, and creating charts), it will:

1. **Automatically create a plan** - The task is broken into manageable steps
2. **Track progress** - Each step's status is tracked and saved
3. **Handle interruptions** - If the task exceeds iteration limits, it pauses gracefully
4. **Resume on "continue"** - Type "继续" or "continue" to resume from where it left off

## Usage

### Normal Flow

Simply ask maxclaw to perform a complex task:

```
下载分析腾讯最近5年年度财报PDF，提取关键财务指标，制作成图表
```

maxclaw will:
- Create a plan with steps like "搜索财报链接", "下载PDF文件", "提取数据", "制作图表"
- Execute each step automatically
- Show progress as it works

### Pause and Resume

If a task is very complex and exceeds the iteration limit:

```
任务执行中（2/5 步已完成）。输入'继续'以恢复执行。
```

Simply type:

```
继续
```

maxclaw will resume from the exact step where it left off.

### Declaring Steps

The agent can declare new steps during execution using the `[Step]` syntax:

```
[Step] 下载2024年财报
[Step] 提取收入数据
```

These steps will be tracked in the plan.

## Plan File Location

Plans are stored per-session at:

```
~/.maxclaw/workspace/.sessions/{session_key}/plan.json
```

You can inspect this file to see the current plan state.

## Plan Status

- `running` - Task is actively executing
- `paused` - Task exceeded iteration limit, waiting for "continue"
- `completed` - All steps finished successfully
- `failed` - Task encountered an error
