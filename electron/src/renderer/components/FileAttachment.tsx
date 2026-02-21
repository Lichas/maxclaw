import React, { useCallback, useState } from 'react';
import { useDropzone } from 'react-dropzone';

export interface UploadedFile {
  id: string;
  filename: string;
  size: number;
  url: string;
}

interface FileAttachmentProps {
  onFilesUploaded: (files: UploadedFile[]) => void;
  attachedFiles: UploadedFile[];
  onRemoveFile: (id: string) => void;
  disabled?: boolean;
}

export function FileAttachment({
  onFilesUploaded,
  attachedFiles,
  onRemoveFile,
  disabled = false
}: FileAttachmentProps) {
  const [uploading, setUploading] = useState(false);
  const [uploadError, setUploadError] = useState<string | null>(null);

  const uploadFiles = async (files: File[]): Promise<UploadedFile[]> => {
    const uploaded: UploadedFile[] = [];

    for (const file of files) {
      const formData = new FormData();
      formData.append('file', file);

      const response = await fetch('http://localhost:18890/api/upload', {
        method: 'POST',
        body: formData
      });

      if (!response.ok) {
        const error = await response.text();
        throw new Error(`Upload failed: ${error}`);
      }

      const result: UploadedFile = await response.json();
      uploaded.push(result);
    }

    return uploaded;
  };

  const onDrop = useCallback(
    async (acceptedFiles: File[]) => {
      if (acceptedFiles.length === 0 || disabled) return;

      setUploading(true);
      setUploadError(null);

      try {
        const uploaded = await uploadFiles(acceptedFiles);
        onFilesUploaded(uploaded);
      } catch (err) {
        setUploadError(err instanceof Error ? err.message : 'Upload failed');
      } finally {
        setUploading(false);
      }
    },
    [disabled, onFilesUploaded]
  );

  const { getRootProps, getInputProps, isDragActive, open } = useDropzone({
    onDrop,
    noClick: true,
    noKeyboard: true,
    disabled: disabled || uploading
  });

  const handleAttachClick = async () => {
    // Use Electron's file dialog for native file selection
    try {
      const filePath = await window.electronAPI.system.selectFile();
      if (!filePath) return;

      // Read file and create File object
      const response = await fetch(`file://${filePath}`);
      const blob = await response.blob();
      const filename = filePath.split('/').pop() || 'file';
      const file = new File([blob], filename, { type: blob.type || 'application/octet-stream' });

      setUploading(true);
      setUploadError(null);

      const uploaded = await uploadFiles([file]);
      onFilesUploaded(uploaded);
    } catch (err) {
      setUploadError(err instanceof Error ? err.message : 'Upload failed');
    } finally {
      setUploading(false);
    }
  };

  const formatFileSize = (bytes: number): string => {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  };

  return (
    <div {...getRootProps()} className="relative">
      <input {...getInputProps()} />

      {/* Drag overlay */}
      {isDragActive && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-primary/20 backdrop-blur-sm">
          <div className="rounded-2xl border-2 border-dashed border-primary bg-background px-8 py-6 text-center shadow-xl">
            <UploadIcon className="mx-auto mb-3 h-10 w-10 text-primary" />
            <p className="text-lg font-medium text-foreground">释放以上传文件</p>
            <p className="mt-1 text-sm text-foreground/60">支持拖放多个文件</p>
          </div>
        </div>
      )}

      {/* File list */}
      {attachedFiles.length > 0 && (
        <div className="mb-3 flex flex-wrap gap-2">
          {attachedFiles.map((file) => (
            <div
              key={file.id}
              className="flex items-center gap-2 rounded-lg border border-border bg-secondary px-3 py-1.5 text-xs"
            >
              <DocumentIcon className="h-4 w-4 text-foreground/60" />
              <span className="max-w-[120px] truncate text-foreground">{file.filename}</span>
              <span className="text-foreground/50">({formatFileSize(file.size)})</span>
              <button
                type="button"
                onClick={() => onRemoveFile(file.id)}
                disabled={disabled}
                className="ml-1 rounded p-0.5 text-foreground/40 hover:bg-foreground/10 hover:text-foreground disabled:opacity-50"
              >
                <XIcon className="h-3 w-3" />
              </button>
            </div>
          ))}
        </div>
      )}

      {/* Error message */}
      {uploadError && (
        <div className="mb-2 rounded-md bg-red-500/10 px-3 py-1.5 text-xs text-red-500">
          {uploadError}
        </div>
      )}

      {/* Attach button */}
      <button
        type="button"
        onClick={handleAttachClick}
        disabled={disabled || uploading}
        className="inline-flex items-center gap-1 rounded-md bg-secondary px-2 py-1 text-xs text-foreground/70 transition-colors hover:bg-secondary/80 disabled:opacity-50"
      >
        {uploading ? (
          <>
            <SpinnerIcon className="h-3.5 w-3.5 animate-spin" />
            上传中...
          </>
        ) : (
          <>
            <PaperClipIcon className="h-3.5 w-3.5" />
            附件
          </>
        )}
      </button>
    </div>
  );
}

function PaperClipIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M15.172 7l-6.586 6.586a2 2 0 102.828 2.828L18 9.828a4 4 0 00-5.657-5.657L5.757 10.757a6 6 0 108.486 8.486L20 13.486"
      />
    </svg>
  );
}

function DocumentIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"
      />
    </svg>
  );
}

function XIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
    </svg>
  );
}

function UploadIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12"
      />
    </svg>
  );
}

function SpinnerIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" viewBox="0 0 24 24">
      <circle
        className="opacity-25"
        cx="12"
        cy="12"
        r="10"
        stroke="currentColor"
        strokeWidth="4"
      />
      <path
        className="opacity-75"
        fill="currentColor"
        d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
      />
    </svg>
  );
}
