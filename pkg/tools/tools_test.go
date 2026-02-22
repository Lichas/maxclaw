package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateParams(t *testing.T) {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":      "string",
				"minLength": float64(2),
				"maxLength": float64(50),
			},
			"age": map[string]interface{}{
				"type":    "integer",
				"minimum": float64(0),
				"maximum": float64(150),
			},
			"email": map[string]interface{}{
				"type": "string",
			},
			"tags": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
				},
			},
			"address": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"city": map[string]interface{}{
						"type": "string",
					},
				},
				"required": []interface{}{"city"},
			},
		},
		"required": []interface{}{"name"},
	}

	tests := []struct {
		name    string
		params  map[string]interface{}
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid params",
			params:  map[string]interface{}{"name": "John", "age": 30},
			wantErr: false,
		},
		{
			name:    "missing required",
			params:  map[string]interface{}{"age": 30},
			wantErr: true,
			errMsg:  "missing required parameter: name",
		},
		{
			name:    "wrong type",
			params:  map[string]interface{}{"name": "John", "age": "thirty"},
			wantErr: true,
			errMsg:  "age should be integer",
		},
		{
			name:    "min length violation",
			params:  map[string]interface{}{"name": "J"},
			wantErr: true,
			errMsg:  "name must be at least 2 chars",
		},
		{
			name:    "minimum violation",
			params:  map[string]interface{}{"name": "John", "age": -5},
			wantErr: true,
			errMsg:  "age must be >= 0",
		},
		{
			name:    "nested object missing required",
			params:  map[string]interface{}{"name": "John", "address": map[string]interface{}{}},
			wantErr: true,
			errMsg:  "missing required parameter: city",
		},
		{
			name:    "array with wrong item type",
			params:  map[string]interface{}{"name": "John", "tags": []interface{}{1, 2, 3}},
			wantErr: true,
			errMsg:  "should be string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateParams(schema, tt.params)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRegistry(t *testing.T) {
	reg := NewRegistry()

	// 注册测试工具
	tool := NewReadFileTool()
	err := reg.Register(tool)
	require.NoError(t, err)

	// 测试获取
	got, exists := reg.Get("read_file")
	assert.True(t, exists)
	assert.Equal(t, "read_file", got.Name())

	// 测试获取不存在的工具
	_, exists = reg.Get("nonexistent")
	assert.False(t, exists)

	// 测试列出
	names := reg.List()
	assert.Contains(t, names, "read_file")

	// 测试数量
	assert.Equal(t, 1, reg.Count())
}

func TestReadFileTool(t *testing.T) {
	// 创建临时目录
	tmpDir := t.TempDir()
	SetAllowedDir(tmpDir)
	SetWorkspaceDir(tmpDir)
	t.Cleanup(func() {
		SetAllowedDir("")
		SetWorkspaceDir("")
	})

	// 创建测试文件
	testFile := filepath.Join(tmpDir, "test.txt")
	content := "line1\nline2\nline3\nline4\nline5"
	err := os.WriteFile(testFile, []byte(content), 0644)
	require.NoError(t, err)

	tool := NewReadFileTool()
	ctx := context.Background()

	t.Run("read full file", func(t *testing.T) {
		result, err := tool.Execute(ctx, map[string]interface{}{
			"path": testFile,
		})
		require.NoError(t, err)
		assert.Equal(t, content, result)
	})

	t.Run("read with limit", func(t *testing.T) {
		result, err := tool.Execute(ctx, map[string]interface{}{
			"path":  testFile,
			"limit": 2,
		})
		require.NoError(t, err)
		assert.Equal(t, "line1\nline2", result)
	})

	t.Run("read with offset", func(t *testing.T) {
		result, err := tool.Execute(ctx, map[string]interface{}{
			"path":   testFile,
			"offset": 2,
		})
		require.NoError(t, err)
		assert.Equal(t, "line3\nline4\nline5", result)
	})

	t.Run("read nonexistent file", func(t *testing.T) {
		_, err := tool.Execute(ctx, map[string]interface{}{
			"path": filepath.Join(tmpDir, "nonexistent.txt"),
		})
		assert.Error(t, err)
	})

	t.Run("path outside allowed dir", func(t *testing.T) {
		_, err := tool.Execute(ctx, map[string]interface{}{
			"path": "/etc/passwd",
		})
		assert.Error(t, err)
	})

	t.Run("read relative path from session directory", func(t *testing.T) {
		sessionCtx := WithRuntimeContextWithSession(ctx, "desktop", "ignored-chat", "desktop:task-42")
		sessionDir := filepath.Join(tmpDir, ".sessions", "desktop_task-42")
		require.NoError(t, os.MkdirAll(sessionDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(sessionDir, "notes.md"), []byte("session scoped"), 0644))

		result, err := tool.Execute(sessionCtx, map[string]interface{}{
			"path": "notes.md",
		})
		require.NoError(t, err)
		assert.Equal(t, "session scoped", result)
	})
}

func TestWriteFileTool(t *testing.T) {
	tmpDir := t.TempDir()
	SetAllowedDir(tmpDir)
	SetWorkspaceDir(tmpDir)
	t.Cleanup(func() {
		SetAllowedDir("")
		SetWorkspaceDir("")
	})

	tool := NewWriteFileTool()
	ctx := context.Background()

	t.Run("write new file", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "newfile.txt")
		result, err := tool.Execute(ctx, map[string]interface{}{
			"path":    testFile,
			"content": "hello world",
		})
		require.NoError(t, err)
		assert.Contains(t, result, "written successfully")

		content, err := os.ReadFile(testFile)
		require.NoError(t, err)
		assert.Equal(t, "hello world", string(content))
	})

	t.Run("write creates directories", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "subdir", "deep", "file.txt")
		_, err := tool.Execute(ctx, map[string]interface{}{
			"path":    testFile,
			"content": "nested content",
		})
		require.NoError(t, err)

		content, err := os.ReadFile(testFile)
		require.NoError(t, err)
		assert.Equal(t, "nested content", string(content))
	})

	t.Run("write relative path into current session directory", func(t *testing.T) {
		sessionCtx := WithRuntimeContextWithSession(ctx, "desktop", "ignored-chat", "desktop:1771671668841")
		_, err := tool.Execute(sessionCtx, map[string]interface{}{
			"path":    "draft/report.md",
			"content": "# report",
		})
		require.NoError(t, err)

		expectedPath := filepath.Join(tmpDir, ".sessions", "desktop_1771671668841", "draft", "report.md")
		body, err := os.ReadFile(expectedPath)
		require.NoError(t, err)
		assert.Equal(t, "# report", string(body))
	})

	t.Run("block parent escape for session-relative path", func(t *testing.T) {
		sessionCtx := WithRuntimeContextWithSession(ctx, "desktop", "ignored-chat", "desktop:1771671668841")
		_, err := tool.Execute(sessionCtx, map[string]interface{}{
			"path":    "../escape.md",
			"content": "nope",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "escapes current session directory")
	})
}

func TestEditFileTool(t *testing.T) {
	tmpDir := t.TempDir()
	SetAllowedDir(tmpDir)

	tool := NewEditFileTool()
	ctx := context.Background()

	t.Run("edit existing text", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "editable.txt")
		err := os.WriteFile(testFile, []byte("hello world"), 0644)
		require.NoError(t, err)

		result, err := tool.Execute(ctx, map[string]interface{}{
			"path":       testFile,
			"old_string": "world",
			"new_string": "universe",
		})
		require.NoError(t, err)
		assert.Contains(t, result, "edited successfully")

		content, err := os.ReadFile(testFile)
		require.NoError(t, err)
		assert.Equal(t, "hello universe", string(content))
	})

	t.Run("edit nonexistent text", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "editable2.txt")
		err := os.WriteFile(testFile, []byte("hello world"), 0644)
		require.NoError(t, err)

		_, err = tool.Execute(ctx, map[string]interface{}{
			"path":       testFile,
			"old_string": "nonexistent",
			"new_string": "replacement",
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestListDirTool(t *testing.T) {
	tmpDir := t.TempDir()
	SetAllowedDir(tmpDir)

	// 创建测试目录结构
	os.MkdirAll(filepath.Join(tmpDir, "subdir"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("content"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "file2.txt"), []byte("content"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "subdir", "nested.txt"), []byte("content"), 0644)

	tool := NewListDirTool()
	ctx := context.Background()

	t.Run("list directory", func(t *testing.T) {
		result, err := tool.Execute(ctx, map[string]interface{}{
			"path": tmpDir,
		})
		require.NoError(t, err)
		assert.Contains(t, result, "[DIR]  subdir/")
		assert.Contains(t, result, "[FILE] file1.txt")
		assert.Contains(t, result, "[FILE] file2.txt")
		assert.NotContains(t, result, "nested.txt") // 非递归
	})

	t.Run("list recursive", func(t *testing.T) {
		result, err := tool.Execute(ctx, map[string]interface{}{
			"path":      tmpDir,
			"recursive": true,
		})
		require.NoError(t, err)
		assert.Contains(t, result, "[DIR]  subdir/")
		assert.Contains(t, result, "[FILE] nested.txt")
	})
}

func TestExecTool(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewExecTool(tmpDir, 5, false)
	ctx := context.Background()

	t.Run("execute simple command", func(t *testing.T) {
		result, err := tool.Execute(ctx, map[string]interface{}{
			"command": "echo hello",
		})
		require.NoError(t, err)
		assert.Contains(t, result, "hello")
	})

	t.Run("execute with working dir", func(t *testing.T) {
		// 在当前目录创建文件
		testFile := filepath.Join(tmpDir, "testfile.txt")
		os.WriteFile(testFile, []byte("test"), 0644)

		result, err := tool.Execute(ctx, map[string]interface{}{
			"command": "ls -1",
		})
		require.NoError(t, err)
		assert.Contains(t, result, "testfile.txt")
	})

	t.Run("dangerous command blocked", func(t *testing.T) {
		_, err := tool.Execute(ctx, map[string]interface{}{
			"command": "rm -rf /",
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "dangerous")
	})

	t.Run("command timeout", func(t *testing.T) {
		result, err := tool.Execute(ctx, map[string]interface{}{
			"command": "sleep 10",
			"timeout": 1,
		})
		require.NoError(t, err) // 不返回错误，而是返回超时消息
		assert.Contains(t, result, "timed out")
	})
}

func TestExecToolRestrictToWorkspace(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewExecTool(tmpDir, 5, true)
	ctx := context.Background()

	t.Run("allow relative command", func(t *testing.T) {
		result, err := tool.Execute(ctx, map[string]interface{}{
			"command": "echo ok > file.txt && cat file.txt",
		})
		require.NoError(t, err)
		assert.Contains(t, result, "ok")
	})

	t.Run("block absolute path outside workspace", func(t *testing.T) {
		_, err := tool.Execute(ctx, map[string]interface{}{
			"command": "cat /etc/passwd",
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "outside workspace")
	})

	t.Run("block parent directory escape", func(t *testing.T) {
		_, err := tool.Execute(ctx, map[string]interface{}{
			"command": "cd .. && ls",
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "outside workspace")
	})

	t.Run("block home expansion", func(t *testing.T) {
		_, err := tool.Execute(ctx, map[string]interface{}{
			"command": "ls ~",
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not allowed")
	})
}

func TestMessageTool(t *testing.T) {
	var receivedChannel, receivedChatID, receivedContent string
	callback := func(channel, chatID, content string) error {
		receivedChannel = channel
		receivedChatID = chatID
		receivedContent = content
		return nil
	}

	tool := NewMessageTool(callback)
	tool.SetContext("telegram", "123456")
	ctx := context.Background()

	t.Run("send message", func(t *testing.T) {
		result, err := tool.Execute(ctx, map[string]interface{}{
			"content": "Hello user!",
		})
		require.NoError(t, err)
		assert.Contains(t, result, "sent successfully")
		assert.Equal(t, "telegram", receivedChannel)
		assert.Equal(t, "123456", receivedChatID)
		assert.Equal(t, "Hello user!", receivedContent)
	})

	t.Run("send with override", func(t *testing.T) {
		_, err := tool.Execute(ctx, map[string]interface{}{
			"content": "Hello!",
			"channel": "discord",
			"chat_id": "999999",
		})
		require.NoError(t, err)
		assert.Equal(t, "discord", receivedChannel)
		assert.Equal(t, "999999", receivedChatID)
	})

	t.Run("use runtime context", func(t *testing.T) {
		toolNoContext := NewMessageTool(callback)
		runtimeCtx := WithRuntimeContext(ctx, "whatsapp", "556677")
		_, err := toolNoContext.Execute(runtimeCtx, map[string]interface{}{
			"content": "Runtime hello",
		})
		require.NoError(t, err)
		assert.Equal(t, "whatsapp", receivedChannel)
		assert.Equal(t, "556677", receivedChatID)
		assert.Equal(t, "Runtime hello", receivedContent)
	})
}

func TestExtractTextFromHTML(t *testing.T) {
	html := `<html>
		<head><script>alert('test');</script><style>body{color:red}</style></head>
		<body>
			<h1>Title</h1>
			<p>This is a paragraph.</p>
			<div>Another block</div>
		</body>
	</html>`

	result := extractTextFromHTML(html)
	assert.Contains(t, result, "Title")
	assert.Contains(t, result, "This is a paragraph.")
	assert.Contains(t, result, "Another block")
	assert.NotContains(t, result, "<script>")
	assert.NotContains(t, result, "alert")
	assert.NotContains(t, result, "<style>")
}
