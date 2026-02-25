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
  onOpenDirectory?: (path: string) => void;
}

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

export function FileTreeSidebar({
  sessionKey,
  workspacePath,
  onSelectFile,
  selectedPath,
  onOpenDirectory
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

  useEffect(() => {
    const init = async () => {
      if (!sessionDir) {
        setError('未配置工作空间');
        return;
      }

      setLoading(true);
      setError(null);

      try {
        const entries = await loadDirectory('.');
        setTreeData(entries);
        setSessionDirExists(true);
      } catch (err) {
        const errMsg = err instanceof Error ? err.message : String(err);
        // 如果目录不存在，显示空状态而不是错误
        if (errMsg.includes('ENOENT') || errMsg.includes('no such file') || errMsg.includes('not found')) {
          setSessionDirExists(false);
          setError(null);
        } else {
          setError(errMsg);
        }
      } finally {
        setLoading(false);
      }
    };

    void init();
  }, [sessionDir, loadDirectory]);

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
      setSessionDirExists(true);
      setError(null);
    } catch (err) {
      const errMsg = err instanceof Error ? err.message : String(err);
      if (errMsg.includes('ENOENT') || errMsg.includes('no such file') || errMsg.includes('not found')) {
        setSessionDirExists(false);
        setError(null);
      } else {
        setError(errMsg);
      }
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
        <div className="flex items-center gap-1">
          {onOpenDirectory && (
            <button
              onClick={() => onOpenDirectory(sessionDir)}
              className="rounded-md p-1.5 text-foreground/50 transition-colors hover:bg-secondary hover:text-foreground"
              title="打开目录"
            >
              <OpenFolderIcon className="h-3.5 w-3.5" />
            </button>
          )}
          <button
            onClick={refresh}
            disabled={loading}
            className="rounded-md p-1.5 text-foreground/50 transition-colors hover:bg-secondary hover:text-foreground disabled:opacity-50"
            title="刷新"
          >
            <RefreshIcon className={`h-3.5 w-3.5 ${loading ? 'animate-spin' : ''}`} />
          </button>
        </div>
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
              className={`group flex cursor-pointer items-center gap-1.5 rounded-md py-1.5 pr-2 transition-colors ${
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

              {/* Preview button for files */}
              {node.type === 'file' && (
                <button
                  onClick={(e) => {
                    e.stopPropagation();
                    onSelect(node);
                  }}
                  className="rounded border border-border/60 bg-background px-1.5 py-0.5 text-[10px] text-foreground/60 opacity-0 transition-opacity hover:bg-secondary hover:text-foreground group-hover:opacity-100"
                  title="预览"
                >
                  预览
                </button>
              )}
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

function OpenFolderIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M5 19a2 2 0 01-2-2V7a2 2 0 012-2h4l2 2h4a2 2 0 012 2v1M5 19h14a2 2 0 002-2v-5a2 2 0 00-2-2H9a2 2 0 00-2 2v5a2 2 0 01-2 2z"
      />
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M12 11v6m3-3l-3 3-3-3"
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
