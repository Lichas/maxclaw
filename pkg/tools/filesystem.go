package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// allowedDir 全局允许的目录（用于沙箱）
var allowedDir string
var workspaceDir string

// SetAllowedDir 设置允许访问的目录
func SetAllowedDir(dir string) {
	allowedDir = dir
}

// SetWorkspaceDir 设置工作区目录（用于会话文件默认路径）
func SetWorkspaceDir(dir string) {
	workspaceDir = strings.TrimSpace(dir)
}

// isPathAllowed 检查路径是否允许访问
func isPathAllowed(path string) error {
	if allowedDir == "" {
		return nil
	}

	// 解析绝对路径
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	// 解析允许的目录
	absAllowed, err := filepath.Abs(allowedDir)
	if err != nil {
		return fmt.Errorf("invalid allowed directory: %w", err)
	}

	// 检查路径是否在允许目录内
	if !strings.HasPrefix(absPath, absAllowed) {
		return fmt.Errorf("path %s is outside of allowed directory %s", absPath, absAllowed)
	}

	return nil
}

// resolvePath 解析并验证路径
func resolvePath(ctx context.Context, path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("path is required")
	}

	// 展开 ~
	if strings.HasPrefix(path, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(homeDir, path[2:])
	}

	if !filepath.IsAbs(path) {
		if sessionBase, ok := sessionBaseDirFromContext(ctx); ok {
			absBase, err := filepath.Abs(sessionBase)
			if err != nil {
				return "", fmt.Errorf("invalid session base path: %w", err)
			}

			absPath, err := filepath.Abs(filepath.Join(absBase, path))
			if err != nil {
				return "", err
			}

			rel, err := filepath.Rel(absBase, absPath)
			if err != nil {
				return "", fmt.Errorf("failed to resolve path: %w", err)
			}
			if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
				return "", fmt.Errorf("path %q escapes current session directory", path)
			}

			if err := isPathAllowed(absPath); err != nil {
				return "", err
			}
			return absPath, nil
		}
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	if err := isPathAllowed(absPath); err != nil {
		return "", err
	}

	return absPath, nil
}

func sessionBaseDirFromContext(ctx context.Context) (string, bool) {
	root := strings.TrimSpace(workspaceDir)
	if root == "" {
		return "", false
	}

	sessionKey := strings.TrimSpace(RuntimeSessionKeyFrom(ctx))
	if sessionKey == "" {
		channel, chatID := RuntimeContextFrom(ctx)
		chatID = strings.TrimSpace(chatID)
		if chatID == "" {
			return "", false
		}
		channel = strings.TrimSpace(channel)
		if channel == "" {
			sessionKey = chatID
		} else {
			sessionKey = channel + ":" + chatID
		}
	}

	return filepath.Join(root, ".sessions", sanitizePathSegment(sessionKey)), true
}

func sanitizePathSegment(input string) string {
	if input == "" {
		return "default"
	}

	var b strings.Builder
	b.Grow(len(input))
	for _, c := range input {
		if (c >= 'a' && c <= 'z') ||
			(c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') ||
			c == '-' || c == '_' {
			b.WriteRune(c)
		} else {
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 {
		return "default"
	}
	return b.String()
}

// ReadFileTool 读取文件工具
type ReadFileTool struct {
	BaseTool
}

// NewReadFileTool 创建读取文件工具
func NewReadFileTool() *ReadFileTool {
	return &ReadFileTool{
		BaseTool: BaseTool{
			name:        "read_file",
			description: "Read the contents of a file. Use for viewing code, logs, or any text file.",
			parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Relative path to the file to read (e.g., 'document.md' or 'data/report.json'). Automatically resolves to the current session directory.",
					},
					"limit": map[string]interface{}{
						"type":        "integer",
						"description": "Maximum number of lines to read (optional)",
						"minimum":     1,
						"maximum":     1000,
					},
					"offset": map[string]interface{}{
						"type":        "integer",
						"description": "Line offset to start reading from (0-indexed, optional)",
						"minimum":     0,
					},
				},
				"required": []string{"path"},
			},
		},
	}
}

// Execute 执行读取文件
func (t *ReadFileTool) Execute(ctx context.Context, params map[string]interface{}) (string, error) {
	path, _ := params["path"].(string)
	if path == "" {
		return "", fmt.Errorf("path is required")
	}

	resolvedPath, err := resolvePath(ctx, path)
	if err != nil {
		return "", err
	}

	content, err := os.ReadFile(resolvedPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	// 处理 offset 和 limit (支持 float64 和 int)
	offset := 0
	if v, ok := params["offset"].(float64); ok {
		offset = int(v)
	} else if v, ok := params["offset"].(int); ok {
		offset = v
	}

	limit := 0
	if v, ok := params["limit"].(float64); ok {
		limit = int(v)
	} else if v, ok := params["limit"].(int); ok {
		limit = v
	}

	lines := strings.Split(string(content), "\n")

	// 应用 offset
	if offset > 0 {
		if offset >= len(lines) {
			return "", fmt.Errorf("offset exceeds file length")
		}
		lines = lines[offset:]
	}

	// 应用 limit
	if limit > 0 && limit < len(lines) {
		lines = lines[:limit]
	}

	return strings.Join(lines, "\n"), nil
}

// WriteFileTool 写入文件工具
type WriteFileTool struct {
	BaseTool
}

// NewWriteFileTool 创建写入文件工具
func NewWriteFileTool() *WriteFileTool {
	return &WriteFileTool{
		BaseTool: BaseTool{
			name:        "write_file",
			description: "Write content to a file. Creates the file if it doesn't exist, overwrites if it does. Use for creating new files or completely replacing file content.",
			parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Relative path to the file to write (e.g., 'report.md' or 'docs/readme.md'). Files are automatically saved to the current session directory.",
					},
					"content": map[string]interface{}{
						"type":        "string",
						"description": "Content to write to the file",
					},
				},
				"required": []string{"path", "content"},
			},
		},
	}
}

// Execute 执行写入文件
func (t *WriteFileTool) Execute(ctx context.Context, params map[string]interface{}) (string, error) {
	path, _ := params["path"].(string)
	content, _ := params["content"].(string)

	if path == "" {
		return "", fmt.Errorf("path is required")
	}

	resolvedPath, err := resolvePath(ctx, path)
	if err != nil {
		return "", err
	}

	// 确保目录存在
	dir := filepath.Dir(resolvedPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(resolvedPath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return fmt.Sprintf("File written successfully: %s", resolvedPath), nil
}

// EditFileTool 编辑文件工具（替换文本）
type EditFileTool struct {
	BaseTool
}

// NewEditFileTool 创建编辑文件工具
func NewEditFileTool() *EditFileTool {
	return &EditFileTool{
		BaseTool: BaseTool{
			name:        "edit_file",
			description: "Edit a file by replacing specific text. Use for making targeted changes to existing files.",
			parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Relative path to the file to edit (e.g., 'config.yaml'). Automatically resolves to the current session directory.",
					},
					"old_string": map[string]interface{}{
						"type":        "string",
						"description": "Text to replace",
					},
					"new_string": map[string]interface{}{
						"type":        "string",
						"description": "New text to insert",
					},
				},
				"required": []string{"path", "old_string", "new_string"},
			},
		},
	}
}

// Execute 执行编辑文件
func (t *EditFileTool) Execute(ctx context.Context, params map[string]interface{}) (string, error) {
	path, _ := params["path"].(string)
	oldString, _ := params["old_string"].(string)
	newString, _ := params["new_string"].(string)

	if path == "" {
		return "", fmt.Errorf("path is required")
	}

	resolvedPath, err := resolvePath(ctx, path)
	if err != nil {
		return "", err
	}

	content, err := os.ReadFile(resolvedPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	oldContent := string(content)
	if !strings.Contains(oldContent, oldString) {
		return "", fmt.Errorf("old_string not found in file")
	}

	newContent := strings.Replace(oldContent, oldString, newString, 1)
	if err := os.WriteFile(resolvedPath, []byte(newContent), 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return fmt.Sprintf("File edited successfully: %s", resolvedPath), nil
}

// ListDirTool 列出目录工具
type ListDirTool struct {
	BaseTool
}

// NewListDirTool 创建列出目录工具
func NewListDirTool() *ListDirTool {
	return &ListDirTool{
		BaseTool: BaseTool{
			name:        "list_dir",
			description: "List files and directories in a given path. Use to explore directory structure.",
			parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Relative path to the directory to list (e.g., '.' for current directory or 'src' for subdirectory). Automatically resolves to the current session directory.",
					},
					"recursive": map[string]interface{}{
						"type":        "boolean",
						"description": "Whether to list recursively (default: false)",
					},
				},
				"required": []string{"path"},
			},
		},
	}
}

// Execute 执行列出目录
func (t *ListDirTool) Execute(ctx context.Context, params map[string]interface{}) (string, error) {
	path, _ := params["path"].(string)
	if path == "" {
		path = "."
	}

	resolvedPath, err := resolvePath(ctx, path)
	if err != nil {
		return "", err
	}

	recursive := false
	if v, ok := params["recursive"].(bool); ok {
		recursive = v
	}

	var result strings.Builder
	err = listDirRecursive(resolvedPath, "", recursive, &result)
	if err != nil {
		return "", err
	}

	return result.String(), nil
}

// listDirRecursive 递归列出目录
func listDirRecursive(basePath, prefix string, recursive bool, result *strings.Builder) error {
	entries, err := os.ReadDir(basePath)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			result.WriteString(fmt.Sprintf("%s[DIR]  %s/\n", prefix, name))
			if recursive {
				subPath := filepath.Join(basePath, name)
				listDirRecursive(subPath, prefix+"  ", recursive, result)
			}
		} else {
			info, _ := entry.Info()
			size := ""
			if info != nil {
				size = fmt.Sprintf(" (%d bytes)", info.Size())
			}
			result.WriteString(fmt.Sprintf("%s[FILE] %s%s\n", prefix, name, size))
		}
	}

	return nil
}
