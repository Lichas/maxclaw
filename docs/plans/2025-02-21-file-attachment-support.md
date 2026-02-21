# File Attachment Support Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Enable file upload and attachment functionality in the Electron App, allowing users to drag-and-drop files or select files to attach to messages. Requires both frontend UI changes and backend API support.

**Architecture:** Frontend uses Electron's file dialog API and HTML5 drag-and-drop to select files, then uploads them to a new Gateway API endpoint. Backend adds `/api/upload` endpoint that saves files to a workspace uploads directory and returns file metadata. Files are referenced in messages via markdown links.

**Tech Stack:** Electron file APIs, multer (Go equivalent), React DnD, TypeScript

---

## Prerequisites

- Gateway API is running on port 18890
- Electron app has IPC file dialog integration
- Workspace directory is configured and writable

---

### Task 1: Create Backend Upload API

**Files:**
- Create: `internal/webui/upload.go`
- Modify: `internal/webui/server.go` (add route)

**Step 1: Create upload handler**

```go
package webui

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

type UploadResponse struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
	URL      string `json:"url"`
}

func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Parse multipart form (32MB max memory)
	err := r.ParseMultipartForm(32 << 20)
	if err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Failed to get file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Create uploads directory
	uploadsDir := filepath.Join(s.cfg.Agents.Defaults.Workspace, ".uploads")
	if err := os.MkdirAll(uploadsDir, 0755); err != nil {
		http.Error(w, "Failed to create uploads directory", http.StatusInternalServerError)
		return
	}

	// Generate unique filename
	id := uuid.New().String()[:8]
	ext := filepath.Ext(header.Filename)
	safeName := fmt.Sprintf("%s_%s%s", time.Now().Format("20060102"), id, ext)
	targetPath := filepath.Join(uploadsDir, safeName)

	// Save file
	dst, err := os.Create(targetPath)
	if err != nil {
		http.Error(w, "Failed to create file", http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	size, err := io.Copy(dst, file)
	if err != nil {
		os.Remove(targetPath)
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}

	response := UploadResponse{
		ID:       id,
		Filename: header.Filename,
		Size:     size,
		URL:      fmt.Sprintf("/api/uploads/%s", safeName),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *Server) handleGetUpload(w http.ResponseWriter, r *http.Request) {
	filename := filepath.Base(r.URL.Path)
	uploadsDir := filepath.Join(s.cfg.Agents.Defaults.Workspace, ".uploads")
	filePath := filepath.Join(uploadsDir, filename)

	// Security check: ensure file is within uploads directory
	if !strings.HasPrefix(filePath, uploadsDir) {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	http.ServeFile(w, r, filePath)
}
```

**Step 2: Add route to server.go**

In `registerRoutes()` method, add:

```go
// File upload routes
mux.HandleFunc("/api/upload", s.handleUpload)
mux.HandleFunc("/api/uploads/", s.handleGetUpload)
```

**Step 3: Test backend build**

Run:
```bash
cd /Users/lua/git/nanobot-go
make build
```

Expected: Build succeeds.

**Step 4: Commit**

```bash
git add internal/webui/upload.go internal/webui/server.go
git commit -m "feat(api): add file upload endpoint"
```

---

### Task 2: Add IPC File Dialog Support

**Files:**
- Modify: `electron/src/preload/index.ts`
- Modify: `electron/src/main/ipc.ts`

**Step 1: Add file dialog to IPC**

In `ipc.ts`, ensure `selectFile` handler exists:

```typescript
ipcMain.handle('system:selectFile', async (_, filters) => {
  const result = await dialog.showOpenDialog(mainWindow, {
    properties: ['openFile'],
    filters: filters || [
      { name: 'All Files', extensions: ['*'] },
      { name: 'Images', extensions: ['png', 'jpg', 'jpeg', 'gif'] },
      { name: 'Documents', extensions: ['pdf', 'doc', 'docx', 'txt'] },
    ],
    title: 'Select File',
  });

  if (result.canceled || result.filePaths.length === 0) {
    return null;
  }

  // Read file and return path + content info
  const filePath = result.filePaths[0];
  const stats = await fs.promises.stat(filePath);

  return {
    path: filePath,
    name: path.basename(filePath),
    size: stats.size,
  };
});
```

**Step 2: Expose to preload**

Ensure preload script exposes:

```typescript
system: {
  selectFile: (filters?: Array<{ name: string; extensions: string[] }>) =>
    ipcRenderer.invoke('system:selectFile', filters),
}
```

**Step 3: Commit**

```bash
git add electron/src/main/ipc.ts electron/src/preload/index.ts
git commit -m "feat(electron): add file dialog IPC support"
```

---

### Task 3: Create FileAttachment Component

**Files:**
- Create: `electron/src/renderer/components/FileAttachment.tsx`
- Create: `electron/src/renderer/components/FileAttachment.css`

**Step 1: Create component**

```typescript
import React, { useCallback } from 'react';
import { useDropzone } from 'react-dropzone';

interface FileAttachmentProps {
  onFileSelect: (file: File) => void;
  attachedFiles: AttachedFile[];
  onRemove: (index: number) => void;
}

export interface AttachedFile {
  id?: string;
  name: string;
  size: number;
  type: string;
  file?: File;
  url?: string;
}

export function FileAttachment({
  onFileSelect,
  attachedFiles,
  onRemove
}: FileAttachmentProps) {
  const onDrop = useCallback((acceptedFiles: File[]) => {
    if (acceptedFiles.length > 0) {
      onFileSelect(acceptedFiles[0]);
    }
  }, [onFileSelect]);

  const { getRootProps, getInputProps, isDragActive } = useDropzone({
    onDrop,
    noClick: true,
    noKeyboard: true,
  });

  const handleSelectFile = async () => {
    const result = await window.electronAPI.system.selectFile();
    if (result) {
      // Convert to File object
      const response = await fetch(`file://${result.path}`);
      const blob = await response.blob();
      const file = new File([blob], result.name, { size: result.size });
      onFileSelect(file);
    }
  };

  const formatSize = (bytes: number) => {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  };

  return (
    <div {...getRootProps()} className="file-attachment">
      <input {...getInputProps()} />

      {attachedFiles.length > 0 && (
        <div className="attached-files">
          {attachedFiles.map((file, index) => (
            <div key={index} className="file-chip">
              <span className="file-icon">{getFileIcon(file.type)}</span>
              <span className="file-name" title={file.name}>
                {file.name}
              </span>
              <span className="file-size">({formatSize(file.size)})</span>
              <button
                onClick={() => onRemove(index)}
                className="remove-btn"
                aria-label="Remove file"
              >
                ×
              </button>
            </div>
          ))}
        </div>
      )}

      <div className="attachment-actions">
        <button
          onClick={handleSelectFile}
          className="attach-btn"
          type="button"
        >
          <PaperclipIcon className="w-4 h-4" />
          Attach File
        </button>
        {isDragActive && (
          <span className="drag-hint">Drop file here...</span>
        )}
      </div>
    </div>
  );
}

function getFileIcon(type: string): string {
  if (type.startsWith('image/')) return '🖼️';
  if (type.includes('pdf')) return '📄';
  if (type.includes('word') || type.includes('doc')) return '📝';
  if (type.includes('excel') || type.includes('sheet')) return '📊';
  return '📎';
}

function PaperclipIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2}
            d="M15.172 7l-6.586 6.586a2 2 0 102.828 2.828l6.414-6.586a4 4 0 00-5.656-5.656l-6.415 6.585a6 6 0 108.486 8.486L20.5 13" />
    </svg>
  );
}
```

**Step 2: Add styles**

```css
.file-attachment {
  padding: 8px 0;
}

.attached-files {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 8px;
}

.file-chip {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 8px;
  background: var(--secondary);
  border-radius: 6px;
  font-size: 13px;
}

.file-name {
  max-width: 150px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.file-size {
  color: var(--muted);
  font-size: 11px;
}

.remove-btn {
  background: none;
  border: none;
  color: var(--muted);
  cursor: pointer;
  font-size: 16px;
  padding: 0 2px;
}

.remove-btn:hover {
  color: var(--foreground);
}

.attachment-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

.attach-btn {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  background: transparent;
  border: 1px solid var(--border);
  border-radius: 6px;
  color: var(--foreground);
  font-size: 13px;
  cursor: pointer;
}

.attach-btn:hover {
  background: var(--secondary);
}

.drag-hint {
  color: var(--primary);
  font-size: 13px;
}
```

**Step 3: Install react-dropzone**

```bash
cd electron && npm install react-dropzone
```

**Step 4: Commit**

```bash
git add electron/src/renderer/components/FileAttachment.*
git add electron/package.json electron/package-lock.json
git commit -m "feat(electron): add FileAttachment component with drag-and-drop"
```

---

### Task 4: Integrate File Upload into Chat

**Files:**
- Modify: `electron/src/renderer/views/ChatView.tsx`

**Step 1: Add file upload logic**

Add state and handlers:

```typescript
const [attachedFiles, setAttachedFiles] = useState<AttachedFile[]>([]);

const handleFileSelect = async (file: File) => {
  const newFile: AttachedFile = {
    name: file.name,
    size: file.size,
    type: file.type,
    file: file,
  };
  setAttachedFiles(prev => [...prev, newFile]);
};

const handleRemoveFile = (index: number) => {
  setAttachedFiles(prev => prev.filter((_, i) => i !== index));
};

const uploadFile = async (file: File): Promise<string> => {
  const formData = new FormData();
  formData.append('file', file);

  const response = await fetch('http://localhost:18890/api/upload', {
    method: 'POST',
    body: formData,
  });

  if (!response.ok) {
    throw new Error('Upload failed');
  }

  const result = await response.json();
  return result.url;
};
```

**Step 2: Modify message sending to include attachments**

```typescript
const handleSubmit = async (e: React.FormEvent) => {
  e.preventDefault();
  if (!input.trim() && attachedFiles.length === 0) return;

  // Upload files first
  const uploadedUrls = await Promise.all(
    attachedFiles.map(async (file) => {
      if (file.file) {
        const url = await uploadFile(file.file);
        return { ...file, url };
      }
      return file;
    })
  );

  // Build message with file references
  let messageContent = input;
  uploadedUrls.forEach(file => {
    messageContent += `\n\n[${file.name}](${file.url})`;
  });

  // Send message as usual...

  // Clear attachments
  setAttachedFiles([]);
};
```

**Step 3: Add FileAttachment to input area**

```tsx
<div className="border-t border-border p-4 bg-background">
  <FileAttachment
    attachedFiles={attachedFiles}
    onFileSelect={handleFileSelect}
    onRemove={handleRemoveFile}
  />
  <form onSubmit={handleSubmit} className="relative">
    {/* existing textarea */}
  </form>
</div>
```

**Step 4: Commit**

```bash
git add electron/src/renderer/views/ChatView.tsx
git commit -m "feat(electron): integrate file upload into chat"
```

---

### Task 5: Test and Verify

**Step 1: Build and start**

```bash
cd /Users/lua/git/nanobot-go
make build
./build/nanobot-go gateway &
cd electron
npm run dev
```

**Step 2: Test scenarios**

1. Click "Attach File" button - verify file dialog opens
2. Select a file - verify it appears as chip in UI
3. Drag and drop file - verify it attaches
4. Send message with attachment - verify file uploads
5. Click uploaded file link - verify it downloads/displays
6. Remove attachment before sending - verify it clears
7. Test with large files (>10MB) - verify error handling
8. Test with multiple files - verify all upload

**Step 3: Verify backend**

Check uploads directory:
```bash
ls -la ~/.nanobot/workspace/.uploads/
```

**Step 4: Stop and commit final changes**

```bash
pkill -f "nanobot-go gateway"
git add -A
git commit -m "test(electron): verify file attachment functionality"
```

---

### Task 6: Update Documentation

**Files:**
- Modify: `docs/Electron_STATUS.md`
- Modify: `docs/Electron_PRD.md`
- Modify: `CHANGELOG.md`

**Step 1: Update status files**

Mark file attachment as completed.

**Step 2: Update CHANGELOG**

```markdown
#### 新增文件附件功能
- **功能**：聊天支持文件上传和附件
- **实现**：
  - 后端：`/api/upload` 端点，支持 multipart/form-data
  - 前端：拖拽上传、文件选择对话框
  - 文件存储在工作区 `.uploads/` 目录
  - 消息中通过 markdown 链接引用文件
- **测试**
  - 上传功能验证
  - 文件下载验证
```

**Step 3: Final commit**

```bash
git add docs/ CHANGELOG.md
git commit -m "docs: update for file attachment feature"
```

---

## Summary

After completing this plan:
- Users can attach files via drag-and-drop or file dialog
- Files are uploaded to backend and stored in workspace
- Attachments are referenced in messages as markdown links
- Both frontend and backend properly handle file operations
