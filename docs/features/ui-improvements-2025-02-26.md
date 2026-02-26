# UI 改进与功能增强

> 创建日期: 2026-02-26
> 状态: 规划中
> 优先级: 高

---

## 概述

本文档记录 maxclaw 桌面应用的四个 UI/UX 改进需求，旨在提升用户体验和操作便利性。

---

## 需求列表

### 1. 应用语言自动检测

**问题**: 当前应用默认语言为中文，非中文用户首次使用需要手动切换语言。

**解决方案**: 应用启动时自动检测系统语言环境：
- 系统语言为中文（zh*）→ 默认使用中文
- 其他语言环境 → 默认使用英文

**技术实现**:
- 修改 `electron/src/renderer/store/index.ts` 中的 `uiSlice` 初始化逻辑
- 使用 `navigator.language` 检测浏览器/系统语言
- 仅在首次启动时检测（localStorage 中无保存的语言设置时）
- 优先使用用户已保存的语言偏好

**关键代码路径**:
```typescript
// electron/src/renderer/store/index.ts
const detectSystemLanguage = (): 'zh' | 'en' => {
  const systemLang = navigator.language || 'en';
  return systemLang.toLowerCase().startsWith('zh') ? 'zh' : 'en';
};
```

---

### 2. 定时任务失败红点提示

**问题**: 定时任务执行失败后，用户无法从主界面直观发现问题，需要进入任务详情才能查看。

**解决方案**: 在定时任务面板侧边栏项添加失败状态指示器（红点）：
- 当任意定时任务的最后执行记录状态为 `failed` 时显示红点
- 红点显示在侧边栏 "定时任务" 导航项上
- 实时更新（每 30 秒轮询检查）

**技术实现**:
- 修改 `Sidebar.tsx` 组件，添加失败状态检测
- 调用 `/api/cron/history` 接口获取执行记录
- 检查每个任务的最新执行状态
- 使用红色圆点徽章显示在导航图标上

**UI 设计**:
```
[定时任务图标] 定时任务  ●  <- 红点（当存在失败任务时）
```

---

### 3. 定时任务全自动执行属性

**问题**: 当前所有定时任务都使用全局的执行模式，无法为单个任务设置是否需要人工介入。

**解决方案**: 为每个定时任务添加独立的 `executionMode` 属性：
- `safe`: 只读探索模式，不执行任何修改操作
- `ask`: 需要用户确认后继续（默认）
- `auto`: 全自动执行，无需人工介入

**技术实现**:

**后端变更**:
- 修改 `internal/cron/types.go` 中的 `Job` 结构体
- 添加 `ExecutionMode string` 字段
- 修改 `internal/cron/service.go` 中的任务执行逻辑
- 执行任务时根据 Job 的 ExecutionMode 覆盖全局设置

**前端变更**:
- 修改 `ScheduledTasksView.tsx` 中的任务创建/编辑表单
- 添加执行模式下拉选择框
- 更新 CronJob 类型定义，添加 executionMode 字段

**数据结构变更**:
```go
type Job struct {
    ID            string   `json:"id"`
    Name          string   `json:"name"`
    Schedule      Schedule `json:"schedule"`
    Payload       Payload  `json:"payload"`
    Enabled       bool     `json:"enabled"`
    Created       int64    `json:"created"`
    ExecutionMode string   `json:"executionMode,omitempty"` // 新增
}
```

---

### 4. 可视化配置编辑器

**问题**: 当前设置面板仅支持通过表单修改部分配置，无法直接编辑 `config.json` 和 `USER_SOUL.md`。

**解决方案**: 在设置面板添加两个新的编辑器：

#### 4.1 Config.json 可视化编辑器
- 表单形式编辑所有配置项
- 分类显示：Providers、Channels、Tools、Gateway
- 实时验证配置有效性
- 保存时自动备份原配置

#### 4.2 USER_SOUL.md 编辑器
- Markdown 编辑器界面
- 实时预览功能
- 主题自适应（浅色/深色模式）
- 自动保存草稿

**技术实现**:

**新增组件**:
- `ConfigEditor.tsx`: config.json 可视化编辑器
- `SoulEditor.tsx`: USER_SOUL.md Markdown 编辑器
- 使用 `@monaco-editor/react` 或轻量级 `react-simplemde-editor`

**UI 布局**:
在设置面板新增 "高级配置" 分类：
- 通用 (General)
- 模型配置 (Providers)
- 渠道配置 (Channels)
- Gateway
- 高级配置 (Advanced) ← 新增
  - 编辑 config.json
  - 编辑 USER_SOUL.md

**文件路径**:
- Config 文件: `~/.maxclaw/config.json`
- Soul 文件: `~/.maxclaw/workspace/USER_SOUL.md`

---

## 实施顺序

1. **需求 1（语言自动检测）** - 简单快速，提升首次体验
2. **需求 2（失败红点提示）** - 需要前后端配合，提升问题发现能力
3. **需求 3（全自动执行属性）** - 需要修改数据结构，影响任务执行逻辑
4. **需求 4（可视化编辑器）** - 最复杂，需要新增组件和依赖

---

## 依赖项

### 新增依赖
```json
{
  "@monaco-editor/react": "^4.x",  // 或 react-simplemde-editor
  "react-markdown": "^9.x"         // Markdown 预览
}
```

### API 变更
- `GET /api/cron` - 返回数据添加 `executionMode` 字段
- `POST /api/cron` - 接受 `executionMode` 参数
- `PUT /api/cron/:id` - 接受 `executionMode` 参数

---

## 测试计划

### 语言自动检测
1. 清除 localStorage，设置系统语言为中文 → 应用应为中文
2. 清除 localStorage，设置系统语言为英文 → 应用应为英文
3. 切换语言后重启 → 应保持上次选择的语言

### 定时任务失败红点
1. 创建定时任务并使其执行失败 → 侧边栏显示红点
2. 查看执行历史后 → 红点可标记为已读（可选）
3. 失败后重启应用 → 红点仍然存在

### 全自动执行属性
1. 创建任务选择 auto 模式 → 任务自动执行无需确认
2. 创建任务选择 ask 模式 → 需要用户确认
3. 修改现有任务的执行模式 → 下次执行生效

### 可视化编辑器
1. 修改 config.json 并保存 → 配置生效，Gateway 重启（如必要）
2. 编辑 USER_SOUL.md → 文件正确保存
3. 主题切换 → 编辑器主题自动适配

---

## 相关文件

### 需求 1
- `electron/src/renderer/store/index.ts`
- `electron/src/renderer/i18n/index.ts`

### 需求 2
- `electron/src/renderer/components/Sidebar.tsx`
- `electron/src/renderer/views/ScheduledTasksView.tsx`

### 需求 3
- `internal/cron/types.go`
- `internal/cron/service.go`
- `electron/src/renderer/views/ScheduledTasksView.tsx`

### 需求 4
- `electron/src/renderer/views/SettingsView.tsx`
- `electron/src/renderer/components/ConfigEditor.tsx` (新增)
- `electron/src/renderer/components/SoulEditor.tsx` (新增)

---

## 附录：API 数据结构

### CronJob (前端)
```typescript
interface CronJob {
  id: string;
  title: string;
  prompt: string;
  schedule: string;
  scheduleType: 'once' | 'every' | 'cron';
  workDir?: string;
  enabled: boolean;
  createdAt: string;
  lastRun?: string;
  nextRun?: string;
  executionMode?: 'safe' | 'ask' | 'auto';  // 新增
}
```

### Job (后端)
```go
type Job struct {
  ID            string   `json:"id"`
  Name          string   `json:"name"`
  Schedule      Schedule `json:"schedule"`
  Payload       Payload  `json:"payload"`
  Enabled       bool     `json:"enabled"`
  Created       int64    `json:"created"`
  ExecutionMode string   `json:"executionMode,omitempty"`  // 新增
}
```
