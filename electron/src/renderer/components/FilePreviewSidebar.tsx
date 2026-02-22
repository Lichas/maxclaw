import React from 'react';
import { FileReference } from '../utils/fileReferences';
import { MarkdownRenderer } from './MarkdownRenderer';

interface PreviewPayload {
  success: boolean;
  resolvedPath?: string;
  kind?: 'markdown' | 'text' | 'image' | 'pdf' | 'audio' | 'video' | 'office' | 'binary';
  extension?: string;
  fileUrl?: string;
  content?: string;
  error?: string;
}

interface FilePreviewSidebarProps {
  collapsed: boolean;
  width: number;
  selected: FileReference | null;
  preview: PreviewPayload | null;
  loading: boolean;
  onToggle: () => void;
  onResize: (nextWidth: number) => void;
  onOpenFile: () => void;
}

export function FilePreviewSidebar({
  collapsed,
  width,
  selected,
  preview,
  loading,
  onToggle,
  onResize,
  onOpenFile
}: FilePreviewSidebarProps) {
  const startResize = (event: React.MouseEvent<HTMLDivElement>) => {
    event.preventDefault();
    const startX = event.clientX;
    const startWidth = width;

    const onMouseMove = (moveEvent: MouseEvent) => {
      const delta = startX - moveEvent.clientX;
      const next = Math.max(360, Math.min(900, startWidth + delta));
      onResize(next);
    };

    const onMouseUp = () => {
      window.removeEventListener('mousemove', onMouseMove);
      window.removeEventListener('mouseup', onMouseUp);
    };

    window.addEventListener('mousemove', onMouseMove);
    window.addEventListener('mouseup', onMouseUp);
  };

  if (collapsed) {
    return (
      <aside className="hidden h-full w-11 border-l border-border/70 bg-background/60 md:flex md:flex-col md:items-center md:pt-3">
        <button
          onClick={onToggle}
          className="rounded-md border border-border/80 bg-background p-1.5 text-foreground/70 transition-colors hover:bg-secondary hover:text-foreground"
          title="展开预览栏"
          aria-label="Expand preview sidebar"
        >
          <PanelOpenIcon className="h-4 w-4" />
        </button>
      </aside>
    );
  }

  return (
    <aside
      className="relative hidden h-full shrink-0 border-l border-border/70 bg-background/45 md:flex md:flex-col"
      style={{ width: `${width}px` }}
    >
      <div
        onMouseDown={startResize}
        className="absolute left-0 top-0 z-20 h-full w-2 -translate-x-1/2 cursor-col-resize"
        title="拖拽调整预览栏宽度"
        aria-hidden="true"
      />
      <header className="flex items-center gap-2 border-b border-border/60 px-3 py-2">
        <button
          onClick={onToggle}
          className="rounded-md border border-border/80 bg-background p-1.5 text-foreground/70 transition-colors hover:bg-secondary hover:text-foreground"
          title="收起预览栏"
          aria-label="Collapse preview sidebar"
        >
          <PanelCloseIcon className="h-4 w-4" />
        </button>
        <div className="min-w-0 flex-1">
          <p className="truncate text-xs font-semibold uppercase tracking-wide text-foreground/55">文件预览</p>
          <p className="truncate text-sm text-foreground">{selected ? selected.displayName : '未选择文件'}</p>
        </div>
        {selected && (
          <button
            onClick={onOpenFile}
            className="rounded-md border border-border/80 bg-background px-2 py-1 text-xs text-foreground/75 transition-colors hover:bg-secondary hover:text-foreground"
          >
            打开目录
          </button>
        )}
      </header>

      <div className="min-h-0 flex-1 overflow-y-auto p-3">
        {loading && (
          <div className="rounded-xl border border-border/70 bg-card/70 px-3 py-4 text-sm text-foreground/65">
            正在渲染预览...
          </div>
        )}

        {!loading && !selected && (
          <div className="rounded-xl border border-dashed border-border/80 bg-card/60 px-3 py-4 text-sm text-foreground/60">
            点击聊天消息中的文件按钮，在这里查看预览。
          </div>
        )}

        {!loading && selected && preview && !preview.success && (
          <div className="rounded-xl border border-red-400/45 bg-red-500/10 px-3 py-4 text-sm text-red-400">
            预览失败: {preview.error || '未知错误'}
          </div>
        )}

        {!loading && selected && preview && preview.success && (
          <div className="space-y-3">
            <div className="inline-flex rounded-full bg-secondary px-2 py-1 text-[11px] uppercase tracking-wide text-foreground/60">
              {preview.kind || selected.kind}
            </div>
            <FilePreviewBody preview={preview} />
          </div>
        )}
      </div>
    </aside>
  );
}

function FilePreviewBody({ preview }: { preview: PreviewPayload }) {
  if (preview.kind === 'markdown') {
    return (
      <div className="rounded-xl border border-border/70 bg-card/75 p-3">
        <MarkdownRenderer content={preview.content || ''} className="text-sm" />
      </div>
    );
  }

  if (preview.kind === 'text' || preview.kind === 'office') {
    return (
      <pre className="rounded-xl border border-border/70 bg-card/75 p-3 text-xs leading-5 text-foreground/85 whitespace-pre-wrap break-all">
        {preview.content || '文件没有可展示内容。'}
      </pre>
    );
  }

  if (preview.kind === 'image' && preview.fileUrl) {
    return (
      <div className="overflow-hidden rounded-xl border border-border/70 bg-card/80 p-2">
        <img src={preview.fileUrl} alt="preview" className="max-h-[75vh] w-full rounded-md object-contain" />
      </div>
    );
  }

  if (preview.kind === 'pdf' && preview.fileUrl) {
    return (
      <div className="overflow-hidden rounded-xl border border-border/70 bg-card/80">
        <iframe src={preview.fileUrl} className="h-[72vh] w-full" title="PDF Preview" />
      </div>
    );
  }

  if (preview.kind === 'video' && preview.fileUrl) {
    return (
      <div className="overflow-hidden rounded-xl border border-border/70 bg-card/80 p-2">
        <video src={preview.fileUrl} controls className="max-h-[72vh] w-full rounded-md" />
      </div>
    );
  }

  if (preview.kind === 'audio' && preview.fileUrl) {
    return (
      <div className="rounded-xl border border-border/70 bg-card/80 p-3">
        <audio src={preview.fileUrl} controls className="w-full" />
      </div>
    );
  }

  return (
    <div className="rounded-xl border border-border/70 bg-card/70 px-3 py-4 text-sm text-foreground/65">
      当前文件类型暂不支持内嵌预览。可点击“打开目录”在文件夹中查看。
    </div>
  );
}

function PanelOpenIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <rect x={3} y={4} width={18} height={16} rx={2.5} strokeWidth={1.7} />
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.7} d="M11 4v16m3-8h4" />
    </svg>
  );
}

function PanelCloseIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <rect x={3} y={4} width={18} height={16} rx={2.5} strokeWidth={1.7} />
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.7} d="M13 4v16m3-8h-4" />
    </svg>
  );
}
