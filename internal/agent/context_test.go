package agent

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContextBuilderLoadsHeartbeatFromMemoryDir(t *testing.T) {
	workspace := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(workspace, "memory"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(workspace, "memory", "heartbeat.md"), []byte("focus: ship cron fix"), 0644))

	builder := NewContextBuilder(workspace)
	messages := builder.BuildMessages(nil, "hello", nil, "telegram", "123")
	require.NotEmpty(t, messages)

	systemPrompt := messages[0].Content
	assert.Contains(t, systemPrompt, "## Heartbeat")
	assert.Contains(t, systemPrompt, "focus: ship cron fix")
}

func TestContextBuilderHeartbeatPrefersMemoryFile(t *testing.T) {
	workspace := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(workspace, "memory"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(workspace, "memory", "heartbeat.md"), []byte("memory heartbeat"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(workspace, "heartbeat.md"), []byte("root heartbeat"), 0644))

	builder := NewContextBuilder(workspace)
	messages := builder.BuildMessages(nil, "hello", nil, "telegram", "123")
	require.NotEmpty(t, messages)

	systemPrompt := messages[0].Content
	assert.Contains(t, systemPrompt, "memory heartbeat")
	assert.NotContains(t, systemPrompt, "root heartbeat")
}
