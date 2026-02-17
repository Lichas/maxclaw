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

func TestContextBuilderIncludesWorkspaceAndSkillsDir(t *testing.T) {
	workspace := t.TempDir()
	builder := NewContextBuilder(workspace)

	messages := builder.BuildMessages(nil, "hello", nil, "telegram", "123")
	require.NotEmpty(t, messages)

	systemPrompt := messages[0].Content
	assert.Contains(t, systemPrompt, "Workspace")
	assert.Contains(t, systemPrompt, workspace)
	assert.Contains(t, systemPrompt, filepath.Join(workspace, "skills"))
	assert.Contains(t, systemPrompt, nanobotSourceMarkerFile)
	assert.Contains(t, systemPrompt, filepath.Join(workspace, nanobotSourceMarkerFile))
	assert.Contains(t, systemPrompt, "**Nanobot Source Marker Found**: no")
}

func TestContextBuilderFindsSourceMarkerInParentDir(t *testing.T) {
	sourceRoot := t.TempDir()
	workspace := filepath.Join(sourceRoot, "sub", "project")
	require.NoError(t, os.MkdirAll(workspace, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(sourceRoot, nanobotSourceMarkerFile), []byte("marker"), 0644))

	builder := NewContextBuilder(workspace)
	messages := builder.BuildMessages(nil, "hello", nil, "telegram", "123")
	require.NotEmpty(t, messages)

	systemPrompt := messages[0].Content
	assert.Contains(t, systemPrompt, "**Nanobot Source Directory**: "+sourceRoot)
	assert.Contains(t, systemPrompt, filepath.Join(sourceRoot, nanobotSourceMarkerFile))
	assert.Contains(t, systemPrompt, "**Nanobot Source Marker Found**: yes")
}

func TestContextBuilderUsesEnvNanobotSourceDir(t *testing.T) {
	workspace := t.TempDir()
	sourceRoot := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(sourceRoot, nanobotSourceMarkerFile), []byte("marker"), 0644))
	t.Setenv("NANOBOT_SOURCE_DIR", sourceRoot)

	builder := NewContextBuilder(workspace)
	messages := builder.BuildMessages(nil, "hello", nil, "telegram", "123")
	require.NotEmpty(t, messages)

	systemPrompt := messages[0].Content
	assert.Contains(t, systemPrompt, "**Nanobot Source Directory**: "+sourceRoot)
	assert.Contains(t, systemPrompt, filepath.Join(sourceRoot, nanobotSourceMarkerFile))
	assert.Contains(t, systemPrompt, "**Nanobot Source Marker Found**: yes")
}

func TestContextBuilderFindsSourceMarkerFromSearchRootsEnv(t *testing.T) {
	workspace := t.TempDir()
	searchRoot := t.TempDir()
	sourceRoot := filepath.Join(searchRoot, "repos", "nanobot-go")
	require.NoError(t, os.MkdirAll(sourceRoot, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(sourceRoot, nanobotSourceMarkerFile), []byte("marker"), 0644))
	t.Setenv(nanobotSourceSearchRootsEnv, searchRoot)

	builder := NewContextBuilder(workspace)
	messages := builder.BuildMessages(nil, "hello", nil, "telegram", "123")
	require.NotEmpty(t, messages)

	systemPrompt := messages[0].Content
	assert.Contains(t, systemPrompt, "**Nanobot Source Directory**: "+sourceRoot)
	assert.Contains(t, systemPrompt, filepath.Join(sourceRoot, nanobotSourceMarkerFile))
	assert.Contains(t, systemPrompt, "**Nanobot Source Marker Found**: yes")
}

func TestContextBuilderSystemPromptMentionsSelfImproveCommands(t *testing.T) {
	workspace := t.TempDir()
	builder := NewContextBuilder(workspace)

	messages := builder.BuildMessages(nil, "hello", nil, "telegram", "123")
	require.NotEmpty(t, messages)

	systemPrompt := messages[0].Content
	assert.Contains(t, systemPrompt, "`claude`")
	assert.Contains(t, systemPrompt, "`codex`")
	assert.Contains(t, systemPrompt, nanobotSourceMarkerFile)
}

func TestContextBuilderIncludesTwoLayerMemoryHints(t *testing.T) {
	workspace := t.TempDir()
	builder := NewContextBuilder(workspace)

	messages := builder.BuildMessages(nil, "hello", nil, "telegram", "123")
	require.NotEmpty(t, messages)

	systemPrompt := messages[0].Content
	assert.Contains(t, systemPrompt, "## Memory System")
	assert.Contains(t, systemPrompt, filepath.Join(workspace, "memory", "MEMORY.md"))
	assert.Contains(t, systemPrompt, filepath.Join(workspace, "memory", "HISTORY.md"))
	assert.Contains(t, systemPrompt, "grep -i")
}
