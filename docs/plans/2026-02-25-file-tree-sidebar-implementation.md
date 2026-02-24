# 文件目录树侧边栏实施计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans or superpowers:subagent-driven-development to implement this plan task-by-task.

**Goal:** 在右侧边栏添加文件目录树窗口，与文件预览、Browser Co-Pilot 并列，默认展示 sessionKey 目录下的文件，点击可预览，并确保文件预览实时显示最新内容。

**Architecture:** 采用 Tab 集成式设计，在现有 FilePreviewSidebar 中添加"文件树" Tab。后端新增 listDirectory IPC API 用于读取目录结构，前端创建 FileTreeSidebar 组件递归展示文件树。

**Tech Stack:** React + TypeScript + Electron IPC + Tailwind CSS

---

## 任务清单

- [ ] 任务 1: 后端添加 listDirectory IPC API
- [ ] 任务 2: 修改 buildFilePreview 添加时间戳防缓存
- [ ] 任务 3: 创建 FileTreeSidebar 组件
- [ ] 任务 4: 修改 FilePreviewSidebar 添加文件树 Tab
- [ ] 任务 5: 更新 ChatView 集成文件树
- [ ] 任务 6: 验证和测试

---

### 任务 1: 后端添加 listDirectory IPC API

**Files:**
- Modify: `electron/src/preload/index.ts` - 添加 listDirectory API 定义
- Modify: `electron/src/main/ipc.ts` - 添加 listDirectory 处理函数

**Step 1: 在 preload/index.ts 添加 API 定义**

在 `system` 对象中添加：

```typescript
listDirectory: (dirPath: string, options?: { workspace?: string; sessionKey?: string }) =>
  ipcRenderer.invoke('system:listDirectory', dirPath, options)
```

在 `FilePreviewResult` 类型附近添加类型定义：

```typescript
interface FileListEntry {
  name: string;
  path: string;
  type: 'file' | 'directory';
  size?: number;
  modifiedTime?: string;
}

interface FileListResult {
  success: boolean;
  entries?: FileListEntry[];
  error?: string;
}
```

**Step 2: 在 main/ipc.ts 添加处理函数**

在文件顶部添加类型定义（在 `FileExistsResult` 之后）：

```typescript
interface FileListEntry {
  name: string;
  path: string;
  type: 'file' | 'directory';
  size?: number;
  modifiedTime?: string;
}

interface FileListResult {
  success: boolean;
  entries?: FileListEntry[];
  error?: string;
}
```

在 `checkFileExists` 函数之后添加 listDirectory 函数：

```typescript
async function listDirectory(dirPath: string, options?: FileResolveOptions): Promise<FileListResult> {
  try {
    const resolvedPath = resolveLocalFilePath(dirPath, options);
    const stat = await fs.promises.stat(resolvedPath);

    if (!stat.isDirectory()) {
      return {
        success: false,
        error: 'Path is not a directory'
      };
    }

    const entries = await fs.promises.readdir(resolvedPath, { withFileTypes: true });
    const result: FileListEntry[] = [];

    for (const entry of entries) {
      // 隐藏文件和目录不显示
      if (entry.name.startsWith('.')) {
        continue;
      }

      const entryPath = path.join(resolvedPath, entry.name);
      const entryStat = await fs.promises.stat(entryPath);

      result.push({
        name: entry.name,
        path: entryPath,
        type: entry.isDirectory() ? 'directory' : 'file',
        size: entryStat.size,
        modifiedTime: entryStat.mtime.toISOString()
      });
    }

    // 按类型排序：目录在前，然后按名称排序
    result.sort((a, b) => {
      if (a.type !== b.type) {
        return a.type === 'directory' ? -1 : 1;
      }
      return a.name.localeCompare(b.name);
    });

    return {
      success: true,
      entries: result
    };
  } catch (error) {
    return {
      success: false,
      error: error instanceof Error ? error.message : String(error)
    };
  }
}
```

在 `createIPCHandlers` 函数中添加 IPC handler（在 `system:fileExists` 之后）：

```typescript
ipcMain.handle('system:listDirectory', async (_, dirPath: string, options?: FileResolveOptions) => {
  return listDirectory(dirPath, options);
});
```

**Step 3: 验证编译**

```bash
cd /Users/lua/git/nanobot-go/electron && npm run typecheck
```

Expected: 无类型错误

**Step 4: Commit**

```bash
git add electron/src/preload/index.ts electron/src/main/ipc.ts
git commit -m "feat: add listDirectory IPC API for file tree"
```

---

### 任务 2: 修改 buildFilePreview 添加时间戳防缓存

**Files:**
- Modify: `electron/src/main/ipc.ts` - 修改 buildFilePreview 函数

**Step 1: 修改 fileUrl 生成逻辑**

找到 `buildFilePreview` 函数中生成 `fileUrl` 的代码（约第 499 行）：

```typescript
fileUrl: pathToFileURL(resolvedPath).toString()
```

修改为：

```typescript
fileUrl: `${pathToFileURL(resolvedPath).toString()}?t=${Date.now()}`
```

**Step 2: 验证修改**

确保修改后的代码在正确的位置，且语法正确。

**Step 3: Commit**

```bash
git add electron/src/main/ipc.ts
git commit -m "fix: add timestamp to fileUrl to prevent caching"
```

---

### 任务 3: 创建 FileTreeSidebar 组件

**Files:**
- Create: `electron/src/renderer/components/FileTreeSidebar.tsx`

**Step 1: 创建组件文件**

```typescript
import React, { useState, useEffect, useCallback } from 'react';

interface FileListEntry {
  name: string;
  path: string;
  type: 'file' | 'directory';
  size?: number;
  modifiedTime?: string;
}

interface FileTreeNode extends FileListEntry {
  children?: FileTreeNode[];
  expanded?: boolean;
  loading?: boolean;
}

interface FileTreeSidebarProps {
  sessionKey: string;
  workspacePath: string;
  onSelectFile: (path: string) => void;
  selectedPath?: string;
}

export function FileTreeSidebar({
  sessionKey,
  workspacePath,
  onSelectFile,
  selectedPath
}: FileTreeSidebarProps) {
  const [treeData, setTreeData] = useState<FileTreeNode[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [sessionDirExists, setSessionDirExists] = useState(true);

  const sessionDir = workspacePath
    ? `${workspacePath}/.sessions/${sanitizeSessionKey(sessionKey)}`
    : '';

  const loadDirectory = useCallback(async (dirPath: string): Promise<FileTreeNode[]> => {
    const result = await window.electronAPI.system.listDirectory(dirPath, {
      workspace: workspacePath,
      sessionKey
    });

    if (!result.success || !result.entries) {
      throw new Error(result.error || 'Failed to load directory');
    }

    return result.entries.map((entry) => ({
      ...entry,
      expanded: false,
      loading: false
    }));
  }, [workspacePath, sessionKey]);

  const checkSessionDir = useCallback(async () => {
    if (!sessionDir) return;

    const result = await window.electronAPI.system.fileExists('.', {
      workspace: workspacePath,
      sessionKey
    });

    setSessionDirExists(result.exists && result.isFile === false);
  }, [sessionDir, workspacePath, sessionKey]);

  useEffect(() => {
    const init = async () => {
      if (!sessionDir) {
        setError('未配置工作空间');
        return;
      }

      setLoading(true);
      setError(null);

      try {
        await checkSessionDir();
        const entries = await loadDirectory('.');
        setTreeData(entries);
      } catch (err) {
        setError(err instanceof Error ? err.message : String(err));
      } finally {
        setLoading(false);
      }
    };

    void init();
  }, [sessionDir, loadDirectory, checkSessionDir]);

  const toggleDirectory = async (node: FileTreeNode, indexPath: number[]) => {
    if (node.type !== 'directory') return;

    const updateNodeAtPath = (
      nodes: FileTreeNode[],
      path: number[],
      depth: number
    ): FileTreeNode[] => {
      if (depth === path.length) {
        return nodes.map((n, i) =>
          i === path[depth - 1]
            ? { ...n, expanded: !n.expanded, loading: !n.expanded && !n.children }
            : n
        );
      }

      return nodes.map((n, i) =>
        i === path[depth]
          ? { ...n, children: updateNodeAtPath(n.children || [], path, depth + 1) }
          : n
      );
    };

    // 如果是展开且有 children，直接切换
    if (node.expanded) {
      setTreeData((prev) => updateNodeAtPath(prev, indexPath, 0));
      return;
    }

    // 如果是展开但没有 children，需要加载
    setTreeData((prev) => updateNodeAtPath(prev, indexPath, 0));

    try {
      const relativePath = getRelativePath(node.path, sessionDir);
      const children = await loadDirectory(relativePath || '.');

      const setChildrenAtPath = (
        nodes: FileTreeNode[],
        path: number[],
        depth: number
      ): FileTreeNode[] => {
        if (depth === path.length - 1) {
          return nodes.map((n, i) =>
            i === path[depth] ? { ...n, children, loading: false } : n
          );
        }

        return nodes.map((n, i) =>
          i === path[depth]
            ? { ...n, children: setChildrenAtPath(n.children || [], path, depth + 1) }
            : n
        );
      };

      setTreeData((prev) => setChildrenAtPath(prev, indexPath, 0));
    } catch (err) {
      // 加载失败，恢复状态
      setTreeData((prev) =>
        updateNodeAtPath(
          prev.map((n, i) =>
            i === indexPath[0] ? { ...n, loading: false } : n
          ),
          indexPath,
          0
        )
      );
    }
  };

  const handleFileClick = (node: FileTreeNode) => {
    if (node.type === 'file') {
      onSelectFile(node.path);
    }
  };

  const refresh = async () => {
    setLoading(true);
    try {
      const entries = await loadDirectory('.');
      setTreeData(entries);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  };

  if (!workspacePath) {
    return (
      <div className="flex h-full flex-col items-center justify-center px-4 text-center">
        <FolderIcon className="mb-3 h-10 w-10 text-foreground/30" />
        <p className="text-sm text-foreground/50">未配置工作空间</p>
      </div>
    );
  }

  if (!sessionDirExists) {
    return (
      <div className="flex h-full flex-col items-center justify-center px-4 text-center">
        <FolderIcon className="mb-3 h-10 w-10 text-foreground/30" />
        <p className="mb-2 text-sm text-foreground/50">Session 目录不存在</p>
        <p className="mb-4 text-xs text-foreground/40">{sessionDir}</p>
        <button
          onClick={refresh}
          className="rounded-md border border-border/80 bg-background px-3 py-1.5 text-xs text-foreground/70 transition-colors hover:bg-secondary"
        >
          刷新
        </button>
      </div>
    );
  }

  if (loading && treeData.length === 0) {
    return (
      <div className="flex h-full flex-col items-center justify-center px-4">
        <div className="mb-3 h-6 w-6 animate-spin rounded-full border-2 border-primary/30 border-t-primary" />
        <p className="text-sm text-foreground/50">加载中...</p>
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex h-full flex-col items-center justify-center px-4 text-center">
        <p className="mb-3 text-sm text-red-500">{error}</p>
        <button
          onClick={refresh}
          className="rounded-md border border-border/80 bg-background px-3 py-1.5 text-xs text-foreground/70 transition-colors hover:bg-secondary"
        >
          重试
        </button>
      </div>
    );
  }

  return (
    <div className="flex h-full flex-col">
      {/* Header */}
      <div className="flex items-center justify-between border-b border-border/60 px-3 py-2">
        <span className="text-[11px] font-semibold uppercase tracking-wide text-foreground/50">
          {sessionKey}
        </span>
        <button
          onClick={refresh}
          disabled={loading}
          className="rounded-md p-1.5 text-foreground/50 transition-colors hover:bg-secondary hover:text-foreground disabled:opacity-50"
          title="刷新"
        >
          <RefreshIcon className={`h-3.5 w-3.5 ${loading ? 'animate-spin' : ''}`} />
        </button>
      </div>

      {/* Tree */}
      <div className="flex-1 overflow-y-auto py-2">
        {treeData.length === 0 ? (
          <div className="flex h-full flex-col items-center justify-center px-4 text-center">
            <p className="text-sm text-foreground/50">暂无文件</p>
          </div>
        ) : (
          <TreeNodeList
            nodes={treeData}
            level={0}
            indexPath={[]}
            selectedPath={selectedPath}
            onToggle={toggleDirectory}
            onSelect={handleFileClick}
          />
        )}
      </div>

      {/* Footer */}
      <div className="border-t border-border/60 px-3 py-2">
        <p className="truncate text-[10px] text-foreground/40" title={sessionDir}>
          {sessionDir}
        </p>
      </div>
    </div>
  );
}

interface TreeNodeListProps {
  nodes: FileTreeNode[];
  level: number;
  indexPath: number[];
  selectedPath?: string;
  onToggle: (node: FileTreeNode, indexPath: number[]) => void;
  onSelect: (node: FileTreeNode) => void;
}

function TreeNodeList({
  nodes,
  level,
  indexPath,
  selectedPath,
  onToggle,
  onSelect
}: TreeNodeListProps) {
  return (
    <div className="space-y-0.5">
      {nodes.map((node, index) => {
        const currentIndexPath = [...indexPath, index];
        const isSelected = node.path === selectedPath;
        const paddingLeft = level * 16 + 8;

        return (
          <div key={`${node.path}-${index}`}>
            <div
              className={`flex cursor-pointer items-center gap-1.5 rounded-md py-1.5 pr-2 transition-colors ${
                isSelected
                  ? 'bg-primary/10 text-primary'
                  : 'hover:bg-secondary/60'
              }`}
              style={{ paddingLeft: `${paddingLeft}px` }}
              onClick={() => {
                if (node.type === 'directory') {
                  onToggle(node, currentIndexPath);
                } else {
                  onSelect(node);
                }
              }}
            >
              {/* Toggle icon for directory */}
              {node.type === 'directory' ? (
                <span className="flex h-4 w-4 items-center justify-center text-foreground/50">
                  {node.loading ? (
                    <div className="h-3 w-3 animate-spin rounded-full border-2 border-primary/30 border-t-primary" />
                  ) : node.expanded ? (
                    <ChevronDownIcon className="h-3.5 w-3.5" />
                  ) : (
                    <ChevronRightIcon className="h-3.5 w-3.5" />
                  )}
                </span>
              ) : (
                <span className="w-4" />
              )}

              {/* Icon */}
              {node.type === 'directory' ? (
                node.expanded ? (
                  <FolderOpenIcon className="h-4 w-4 flex-shrink-0 text-primary/70" />
                ) : (
                  <FolderIcon className="h-4 w-4 flex-shrink-0 text-primary/70" />
                )
              ) : (
                <FileIcon className="h-4 w-4 flex-shrink-0 text-foreground/50" />
              )}

              {/* Name */}
              <span
                className={`min-w-0 flex-1 truncate text-xs ${
                  isSelected ? 'font-medium' : ''
                }`}
                title={node.name}
              >
                {node.name}
              </span>
            </div>

            {/* Children */}
            {node.type === 'directory' && node.expanded && node.children && (
              <TreeNodeList
                nodes={node.children}
                level={level + 1}
                indexPath={currentIndexPath}
                selectedPath={selectedPath}
                onToggle={onToggle}
                onSelect={onSelect}
              />
            )}
          </div>
        );
      })}
    </div>
  );
}

function sanitizeSessionKey(input: string): string {
  if (!input) return 'default';

  let out = '';
  for (const char of input) {
    if (
      (char >= 'a' && char <= 'z') ||
      (char >= 'A' && char <= 'Z') ||
      (char >= '0' && char <= '9') ||
      char === '-' ||
      char === '_'
    ) {
      out += char;
    } else {
      out += '_';
    }
  }

  return out || 'default';
}

function getRelativePath(absolutePath: string, basePath: string): string {
  if (absolutePath.startsWith(basePath)) {
    const relative = absolutePath.slice(basePath.length).replace(/^[/\\]/, '');
    return relative || '.';
  }
  return absolutePath;
}

// Icons
function FolderIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={1.8}
        d="M3 7a2 2 0 012-2h4l2 2h8a2 2 0 012 2v8a2 2 0 01-2 2H5a2 2 0 01-2-2V7z"
      />
    </svg>
  );
}

function FolderOpenIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={1.8}
        d="M5 19a2 2 0 01-2-2V7a2 2 0 012-2h4l2 2h4a2 2 0 012 2v1M5 19h14a2 2 0 002-2v-5a2 2 0 00-2-2H9a2 2 0 00-2 2v5a2 2 0 01-2 2z"
      />
    </svg>
  );
}

function FileIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={1.8}
        d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"
      />
    </svg>
  );
}

function RefreshIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
      />
    </svg>
  );
}

function ChevronRightIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
    </svg>
  );
}

function ChevronDownIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
    </svg>
  );
}
```

**Step 2: 验证类型定义**

检查文件顶部是否缺少类型定义。需要确保 TypeScript 能够识别 `window.electronAPI.system.listDirectory`。在文件顶部添加类型声明：

```typescript
declare global {
  interface Window {
    electronAPI: {
      system: {
        listDirectory: (dirPath: string, options?: { workspace?: string; sessionKey?: string }) => Promise<{
          success: boolean;
          entries?: FileListEntry[];
          error?: string;
        }>;
        fileExists: (path: string, options?: { workspace?: string; sessionKey?: string }) => Promise<{
          exists: boolean;
          isFile?: boolean;
          resolvedPath?: string;
        }>;
      };
    };
  }
}
```

**Step 3: Commit**

```bash
git add electron/src/renderer/components/FileTreeSidebar.tsx
git commit -m "feat: add FileTreeSidebar component"
```

---

### 任务 4: 修改 FilePreviewSidebar 添加文件树 Tab

**Files:**
- Modify: `electron/src/renderer/components/FilePreviewSidebar.tsx`

**Step 1: 更新 Props 类型**

修改 `FilePreviewSidebarProps` 接口：

```typescript
interface FilePreviewSidebarProps {
  collapsed: boolean;
  width: number;
  // ... 现有 props ...
  mode?: 'tree' | 'file' | 'browser';  // 修改: 添加 'tree'
  // ... 其他 props ...
  treePanel?: React.ReactNode;  // 新增: 文件树面板内容
  onModeChange?: (mode: 'tree' | 'file' | 'browser') => void;  // 修改类型
}
```

**Step 2: 添加 treePanel 到函数参数**

```typescript
export function FilePreviewSidebar({
  collapsed,
  width,
  selected,
  preview,
  loading,
  mode = 'tree',  // 默认改为 'tree'
  browserAvailable = false,
  browserPanel,
  treePanel,  // 新增
  onModeChange,
  onToggle,
  onResize,
  onOpenFile,
  imageAssist,
}: FilePreviewSidebarProps) {
```

**Step 3: 修改 Tab 切换按钮区域**

找到 header 中的 Tab 切换按钮部分（约第 124-151 行），修改为三个 Tab：

```tsx
<div className="min-w-0 flex-1">
  <div className="inline-flex rounded-lg border border-border/80 bg-secondary/40 p-1">
    <button
      type="button"
      onClick={() => onModeChange?.('tree')}
      className={`rounded px-2 py-1 text-[11px] transition-colors ${
        mode === 'tree'
          ? 'bg-primary text-primary-foreground shadow-sm'
          : 'text-foreground/70 hover:bg-background/80'
      }`}
    >
      文件树
    </button>
    <button
      type="button"
      onClick={() => onModeChange?.('file')}
      className={`rounded px-2 py-1 text-[11px] transition-colors ${
        mode === 'file'
          ? 'bg-primary text-primary-foreground shadow-sm'
          : 'text-foreground/70 hover:bg-background/80'
      }`}
    >
      文件预览
    </button>
    {browserAvailable && (
      <button
        type="button"
        onClick={() => onModeChange?.('browser')}
        className={`rounded px-2 py-1 text-[11px] transition-colors ${
          mode === 'browser'
            ? 'bg-primary text-primary-foreground shadow-sm'
            : 'text-foreground/70 hover:bg-background/80'
        }`}
      >
        Browser
      </button>
    )}
  </div>
</div>
```

**Step 4: 修改内容渲染区域**

找到内容渲染区域（`{mode === 'browser' ? ...}` 部分），修改为：

```tsx
<div className="min-h-0 flex-1 overflow-y-auto p-3">
  {mode === 'tree' && treePanel}

  {mode === 'browser' && (
    browserAvailable ? (
      <div className="space-y-3">{browserPanel}</div>
    ) : (
      <div className="rounded-xl border border-dashed border-border/80 bg-card/60 px-3 py-4 text-sm text-foreground/60">
        当前会话尚无 Browser 活动。执行 browser 工具后会在这里显示协作面板。
      </div>
    )
  )}

  {mode === 'file' && (
    <>
      {/* 原有的文件预览内容 */}
      {loading && (
        <div className="rounded-xl border border-border/70 bg-card/70 px-3 py-4 text-sm text-foreground/65">
          正在渲染预览...
        </div>
      )}

      {!loading && !selected && (
        <div className="rounded-xl border border-dashed border-border/80 bg-card/60 px-3 py-4 text-sm text-foreground/60">
          点击文件树中的文件，在这里查看预览。
        </div>
      )}

      {/* ... 其余文件预览内容保持不变 ... */}
    </>
  )}
</div>
```

**Step 5: Commit**

```bash
git add electron/src/renderer/components/FilePreviewSidebar.tsx
git commit -m "feat: add tree tab to FilePreviewSidebar"
```

---

### 任务 5: 更新 ChatView 集成文件树

**Files:**
- Modify: `electron/src/renderer/views/ChatView.tsx`

**Step 1: 导入 FileTreeSidebar**

在文件顶部添加导入：

```typescript
import { FileTreeSidebar } from '../components/FileTreeSidebar';
```

**Step 2: 修改 previewSidebarMode 默认值**

找到 `previewSidebarMode` 的定义，将默认值改为 `'tree'`：

```typescript
const previewSidebarMode = browserCopilotVisible
  ? previewModeBySession[currentSessionKey] || 'browser'
  : 'tree';  // 修改: 从 'file' 改为 'tree'
```

**Step 3: 修改 renderPreviewSidebar 函数**

更新 `renderPreviewSidebar` 函数，传入 `treePanel`：

```typescript
const renderPreviewSidebar = () => (
  <FilePreviewSidebar
    collapsed={previewSidebarCollapsed}
    width={previewSidebarWidth}
    selected={selectedFileRef}
    preview={previewData}
    loading={previewLoading}
    mode={previewSidebarMode}
    browserAvailable={browserCopilotVisible}
    browserPanel={renderBrowserCopilotPanel()}
    treePanel={  // 新增
      <FileTreeSidebar
        sessionKey={currentSessionKey}
        workspacePath={workspacePath}
        onSelectFile={(path) => {
          // 创建 FileReference 并预览
          const fileRef: FileReference = {
            id: path.toLowerCase(),
            pathHint: path,
            displayName: path.split(/[/\\]/).pop() || path,
            extension: path.split('.').pop() || '',
            kind: 'binary'
          };
          void previewReference(fileRef);
        }}
        selectedPath={selectedFileRef?.pathHint}
      />
    }
    onModeChange={(mode) => {
      setPreviewModeForSession(currentSessionKey, mode);
    }}
    onToggle={() => setPreviewSidebarCollapsed((prev) => !prev)}
    onResize={setPreviewSidebarWidth}
    onOpenFile={() => {
      void handleOpenSelectedFile();
    }}
    imageAssist={{
      enabled: browserScreenshotInteractive && !browserCopilotBusy,
      busy: browserCopilotBusy,
      hint: browserCopilotBusy ? '正在执行 browser 点击...' : '点击截图即可回传坐标到浏览器执行点击。',
      onImageClick: ({ x, y }) => {
        void handleBrowserImageClick({ x, y });
      }
    }}
  />
);
```

**Step 4: 修改文件预览空状态提示**

由于现在默认显示文件树，文件预览的空状态提示需要修改。这个已经在 Step 3 中处理了（通过修改 FilePreviewSidebar 中的空状态文本）。

**Step 5: 验证类型**

```bash
cd /Users/lua/git/nanobot-go/electron && npm run typecheck
```

**Step 6: Commit**

```bash
git add electron/src/renderer/views/ChatView.tsx
git commit -m "feat: integrate FileTreeSidebar in ChatView"
```

---

### 任务 6: 验证和测试

**Files:**
- 所有修改的文件

**Step 1: 类型检查**

```bash
cd /Users/lua/git/nanobot-go/electron && npm run typecheck
```

Expected: 无错误

**Step 2: 构建 Electron 应用**

```bash
cd /Users/lua/git/nanobot-go/electron && npm run build
```

Expected: 构建成功

**Step 3: 启动应用测试**

1. 启动后端 Gateway: `cd /Users/lua/git/nanobot-go && make run` 或 `./build/maxclaw gateway`
2. 启动 Electron: `cd /Users/lua/git/nanobot-go/electron && npm run dev`

**验证步骤:**

1. 打开应用，进入 Chat 页面
2. 观察右侧边栏，默认应显示"文件树" Tab
3. 文件树应显示当前 session 目录结构
4. 点击文件夹，应展开/折叠
5. 点击文件，应自动切换到"文件预览" Tab 并显示内容
6. 在外部修改文件内容，重新点击文件树中的文件，应显示最新内容
7. Browser Co-Pilot 功能应正常工作

**Step 4: 最终 Commit**

```bash
git add -A
git commit -m "feat: add file tree sidebar with real-time preview"
```

---

## 回滚计划

如果出现问题，可以按以下顺序回滚：

1. 回滚 ChatView 集成: `git checkout -- electron/src/renderer/views/ChatView.tsx`
2. 回滚 FilePreviewSidebar: `git checkout -- electron/src/renderer/components/FilePreviewSidebar.tsx`
3. 删除 FileTreeSidebar: `rm electron/src/renderer/components/FileTreeSidebar.tsx`
4. 回滚 IPC 修改: `git checkout -- electron/src/main/ipc.ts electron/src/preload/index.ts`

---

## 后续优化（可选）

- 添加文件搜索功能
- 添加文件右键菜单（删除、重命名）
- 添加文件拖拽上传
- 自动监听文件系统变化并刷新
