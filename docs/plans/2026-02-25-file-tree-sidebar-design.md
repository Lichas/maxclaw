# 文件目录树侧边栏设计文档

## 背景

当前 Electron 桌面应用右侧边栏只有"文件预览"和"Browser Co-Pilot"两种模式。用户需要：
1. 一个文件目录树窗口，默认展示当前 session 目录下的文件
2. 文件预览实时刷新，避免缓存旧内容
3. 确保 agent 文件操作都在 sessionKey 目录下

## 设计目标

- 在右侧边栏添加文件目录树面板，与现有功能并列
- 提供直观的文件浏览和快速预览体验
- 确保文件预览始终显示最新内容

## 设计方案

### 架构变更

```
右侧边栏布局（垂直排列）:
┌─────────────────────────┐
│  标签切换: [文件树] [预览] [Browser] │
├─────────────────────────┤
│                         │
│    内容区域              │
│  （根据标签显示不同内容）  │
│                         │
└─────────────────────────┘
```

### 方案选择: Tab 集成式

选择将文件树作为新的 Tab 选项添加到现有 `FilePreviewSidebar` 中，与"文件预览"、"Browser Co-Pilot"并列。

**理由:**
- 保持界面一致性
- 右侧边栏宽度可调整，适合文件树展示
- 三个功能互斥，Tab 切换符合使用逻辑

## 详细设计

### 1. 文件树组件

**新组件:** `FileTreeSidebar`

```typescript
interface FileTreeNode {
  name: string;
  path: string;
  type: 'file' | 'directory';
  children?: FileTreeNode[];
  expanded?: boolean;
  selected?: boolean;
}

interface FileTreeSidebarProps {
  sessionKey: string;
  workspacePath: string;
  onSelectFile: (path: string) => void;
  selectedPath?: string;
}
```

**功能:**
- 递归展示目录结构
- 点击文件夹展开/折叠
- 点击文件触发预览
- 自动刷新（监听文件变化或定时刷新）

### 2. FilePreviewSidebar 改造

**新增模式:**
```typescript
type SidebarMode = 'tree' | 'file' | 'browser';
```

**Tab 顺序:**
1. 文件树 - 浏览 session 目录
2. 文件预览 - 预览选中的文件
3. Browser Co-Pilot - 浏览器协作

### 3. 后端 IPC API 扩展

**新增 API:**
```typescript
// preload/index.ts
system: {
  // 现有 API...
  listDirectory: (dirPath: string, options?: FileResolveOptions) => Promise<FileListResult>;
}

// IPC 返回类型
interface FileListResult {
  success: boolean;
  entries?: Array<{
    name: string;
    path: string;
    type: 'file' | 'directory';
    size?: number;
    modifiedTime?: string;
  }>;
  error?: string;
}
```

### 4. 文件预览缓存问题解决

**问题:** 使用 `file://` URL 时，Electron/浏览器可能缓存文件内容

**解决方案:**
- 在 fileUrl 后添加时间戳参数: `file://path/to/file?ts=${Date.now()}`
- 在 `buildFilePreview` 函数中处理
- 或者在读取文本文件时，总是重新读取磁盘内容（已实现）

对于图片/视频等使用 URL 的资源，修改 `buildFilePreview`:
```typescript
fileUrl: `${pathToFileURL(resolvedPath).toString()}?t=${Date.now()}`
```

### 5. 数据流

```
用户点击文件树中的文件
        ↓
FileTreeSidebar.onSelectFile(path)
        ↓
ChatView.previewReference(path)
        ↓
自动切换到 "file" 模式
        ↓
调用 electronAPI.system.previewFile()
        ↓
返回带时间戳的 fileUrl
        ↓
渲染预览内容
```

### 6. UI 设计

**文件树样式:**
- 缩进表示层级（每级 16px）
- 文件夹图标: 📁/📂 表示展开状态
- 文件图标: 根据扩展名显示不同图标
- 选中项高亮显示
- 右键菜单: 打开目录、复制路径

**空状态:**
- session 目录不存在: 显示提示，提供"创建目录"按钮
- 目录为空: 显示"暂无文件"提示

## 实施计划

### 阶段 1: 后端 API
1. 在 `ipc.ts` 中添加 `listDirectory` 处理函数
2. 在 `preload/index.ts` 中暴露 API
3. 修改 `buildFilePreview` 添加时间戳防缓存

### 阶段 2: 前端组件
1. 创建 `FileTreeSidebar` 组件
2. 实现目录递归加载
3. 添加文件夹展开/折叠功能

### 阶段 3: 集成
1. 修改 `FilePreviewSidebar`，添加 "文件树" Tab
2. 更新 `ChatView`，传递文件树所需 props
3. 实现点击文件自动切换预览

### 阶段 4: 优化
1. 添加目录监听自动刷新
2. 优化大量文件时的性能
3. 添加文件搜索/过滤

## 文件变更清单

**修改文件:**
- `electron/src/main/ipc.ts` - 添加 listDirectory API 和防缓存
- `electron/src/preload/index.ts` - 暴露 listDirectory API
- `electron/src/renderer/components/FilePreviewSidebar.tsx` - 添加文件树 Tab
- `electron/src/renderer/views/ChatView.tsx` - 集成文件树

**新增文件:**
- `electron/src/renderer/components/FileTreeSidebar.tsx` - 文件树组件

## 验证标准

1. 打开 ChatView，右侧边栏显示三个 Tab: 文件树、文件预览、Browser
2. 文件树默认显示当前 session 目录结构
3. 点击文件，自动切换到文件预览并显示内容
4. 在外部修改文件后，重新点击文件树中的文件，显示最新内容
5. 文件预览中的图片/视频刷新后显示最新内容
