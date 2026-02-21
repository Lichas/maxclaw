package session

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManager(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir)
	assert.NotNil(t, manager)
}

func TestGetOrCreate(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir)

	// 获取新会话
	session1 := manager.GetOrCreate("cli:test123")
	assert.NotNil(t, session1)
	assert.Equal(t, "cli:test123", session1.Key)
	assert.Empty(t, session1.Messages)

	// 再次获取应该返回同一个会话
	session2 := manager.GetOrCreate("cli:test123")
	assert.Equal(t, session1, session2)
}

func TestAddMessage(t *testing.T) {
	session := &Session{
		Key:      "test",
		Messages: []Message{},
	}

	session.AddMessage("user", "Hello")
	require.Len(t, session.Messages, 1)
	assert.Equal(t, "user", session.Messages[0].Role)
	assert.Equal(t, "Hello", session.Messages[0].Content)
	assert.NotZero(t, session.Messages[0].Timestamp)

	session.AddMessage("assistant", "Hi there!")
	require.Len(t, session.Messages, 2)
	assert.Equal(t, "assistant", session.Messages[1].Role)
}

func TestAddMessageWithTimeline(t *testing.T) {
	session := &Session{
		Key:      "test",
		Messages: []Message{},
	}

	timeline := []TimelineEntry{
		{
			Kind: "activity",
			Activity: &TimelineActivity{
				Type:    "status",
				Summary: "Iteration 1",
			},
		},
		{
			Kind: "text",
			Text: "hello",
		},
	}

	session.AddMessageWithTimeline("assistant", "hello", timeline)
	require.Len(t, session.Messages, 1)
	require.Len(t, session.Messages[0].Timeline, 2)
	assert.Equal(t, "activity", session.Messages[0].Timeline[0].Kind)
	assert.Equal(t, "status", session.Messages[0].Timeline[0].Activity.Type)
	assert.Equal(t, "text", session.Messages[0].Timeline[1].Kind)
	assert.Equal(t, "hello", session.Messages[0].Timeline[1].Text)
}

func TestGetHistory(t *testing.T) {
	session := &Session{
		Key: "test",
		Messages: []Message{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi!"},
		},
	}

	history := session.GetHistory()
	assert.Len(t, history, 2)
	assert.Equal(t, "Hello", history[0].Content)
}

func TestClear(t *testing.T) {
	session := &Session{
		Key:              "test",
		LastConsolidated: 3,
		Messages: []Message{
			{Role: "user", Content: "Hello"},
		},
	}

	session.Clear()
	assert.Empty(t, session.Messages)
	assert.Equal(t, 0, session.LastConsolidated)
}

func TestGetHistoryWithLimit(t *testing.T) {
	session := &Session{
		Key:      "test",
		Messages: []Message{},
	}

	for i := 0; i < 505; i++ {
		session.AddMessage("user", "message")
	}

	assert.Len(t, session.Messages, 505)
	assert.Len(t, session.GetHistory(500), 500)
	assert.Len(t, session.GetHistory(999), 505)
}

func TestSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir)

	// 创建并保存会话
	session := manager.GetOrCreate("cli:session1")
	session.AddMessage("user", "Hello")
	session.AddMessage("assistant", "Hi!")
	session.LastConsolidated = 1

	err := manager.Save(session)
	require.NoError(t, err)

	// 创建新的管理器来测试加载
	manager2 := NewManager(tmpDir)
	loaded := manager2.GetOrCreate("cli:session1")

	assert.Len(t, loaded.Messages, 2)
	assert.Equal(t, "user", loaded.Messages[0].Role)
	assert.Equal(t, "Hello", loaded.Messages[0].Content)
	assert.Equal(t, "assistant", loaded.Messages[1].Role)
	assert.Equal(t, 1, loaded.LastConsolidated)
}

func TestSaveAndLoadTimeline(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir)

	session := manager.GetOrCreate("cli:timeline")
	session.AddMessage("user", "Hello")
	session.AddMessageWithTimeline("assistant", "Hi!", []TimelineEntry{
		{
			Kind: "activity",
			Activity: &TimelineActivity{
				Type:    "status",
				Summary: "Iteration 1",
			},
		},
		{
			Kind: "text",
			Text: "Hi!",
		},
	})
	require.NoError(t, manager.Save(session))

	manager2 := NewManager(tmpDir)
	loaded := manager2.GetOrCreate("cli:timeline")
	require.Len(t, loaded.Messages, 2)
	require.Len(t, loaded.Messages[1].Timeline, 2)
	assert.Equal(t, "activity", loaded.Messages[1].Timeline[0].Kind)
	assert.Equal(t, "status", loaded.Messages[1].Timeline[0].Activity.Type)
	assert.Equal(t, "text", loaded.Messages[1].Timeline[1].Kind)
	assert.Equal(t, "Hi!", loaded.Messages[1].Timeline[1].Text)
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"cli:test123", "cli_test123"},
		{"telegram:user@domain", "telegram_user_domain"},
		{"discord:user#1234", "discord_user_1234"},
		{"normal", "normal"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sanitizeFilename(tt.input)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestSessionFileCreation(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir)

	session := manager.GetOrCreate("test:session")
	session.AddMessage("user", "test")
	err := manager.Save(session)
	require.NoError(t, err)

	// 验证文件被创建
	sessionDir := filepath.Join(tmpDir, ".sessions")
	assert.DirExists(t, sessionDir)

	// 应该有文件
	files, err := os.ReadDir(sessionDir)
	require.NoError(t, err)
	assert.Len(t, files, 1)
}
