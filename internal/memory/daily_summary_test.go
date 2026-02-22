package memory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Lichas/maxclaw/internal/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSummarizePreviousDayAppendsMemory(t *testing.T) {
	workspace := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(workspace, ".sessions"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(workspace, "memory"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(workspace, "memory", "MEMORY.md"), []byte("# Long-term Memory\n"), 0644))

	day := time.Date(2026, 2, 15, 10, 0, 0, 0, time.UTC)
	writeSessionFile(t, workspace, "telegram_chat_1", session.Session{
		Key: "telegram:chat-1",
		Messages: []session.Message{
			{Role: "user", Content: "Please summarize market news", Timestamp: day.Add(2 * time.Hour)},
			{Role: "assistant", Content: "Sure, here is the market summary.", Timestamp: day.Add(2*time.Hour + time.Minute)},
		},
	})

	updated, err := SummarizePreviousDay(workspace, day.AddDate(0, 0, 1))
	require.NoError(t, err)
	assert.True(t, updated)

	body, err := os.ReadFile(filepath.Join(workspace, "memory", "MEMORY.md"))
	require.NoError(t, err)
	text := string(body)
	assert.Contains(t, text, "## Daily Summaries")
	assert.Contains(t, text, "### 2026-02-15")
	assert.Contains(t, text, "Please summarize market news")
	assert.Contains(t, text, "here is the market summary")
}

func TestSummarizePreviousDayIsIdempotent(t *testing.T) {
	workspace := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(workspace, ".sessions"), 0755))
	day := time.Date(2026, 2, 15, 10, 0, 0, 0, time.UTC)

	writeSessionFile(t, workspace, "webui_default", session.Session{
		Key: "webui:default",
		Messages: []session.Message{
			{Role: "user", Content: "draft a release note", Timestamp: day.Add(time.Hour)},
		},
	})

	updated, err := SummarizePreviousDay(workspace, day.AddDate(0, 0, 1))
	require.NoError(t, err)
	assert.True(t, updated)

	updated, err = SummarizePreviousDay(workspace, day.AddDate(0, 0, 1))
	require.NoError(t, err)
	assert.False(t, updated)

	body, err := os.ReadFile(filepath.Join(workspace, "memory", "MEMORY.md"))
	require.NoError(t, err)
	assert.Equal(t, 1, strings.Count(string(body), "### 2026-02-15"))
}

func TestSummarizePreviousDayNoMessages(t *testing.T) {
	workspace := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(workspace, ".sessions"), 0755))

	now := time.Date(2026, 2, 16, 8, 0, 0, 0, time.UTC)
	updated, err := SummarizePreviousDay(workspace, now)
	require.NoError(t, err)
	assert.False(t, updated)
}

func writeSessionFile(t *testing.T, workspace, name string, sess session.Session) {
	t.Helper()
	data, err := json.MarshalIndent(sess, "", "  ")
	require.NoError(t, err)
	path := filepath.Join(workspace, ".sessions", name+".json")
	require.NoError(t, os.WriteFile(path, data, 0644))
}
