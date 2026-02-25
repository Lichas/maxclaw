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
	t.Setenv("MAXCLAW_SOURCE_DIR", workspace)
	builder := NewContextBuilder(workspace)

	messages := builder.BuildMessages(nil, "hello", nil, "telegram", "123")
	require.NotEmpty(t, messages)

	systemPrompt := messages[0].Content
	assert.Contains(t, systemPrompt, "Workspace")
	assert.Contains(t, systemPrompt, workspace)
	assert.Contains(t, systemPrompt, filepath.Join(workspace, "skills"))
	assert.Contains(t, systemPrompt, maxclawSourceMarkerFile)
	assert.Contains(t, systemPrompt, filepath.Join(workspace, maxclawSourceMarkerFile))
	assert.Contains(t, systemPrompt, "**Maxclaw Source Marker Found**: no")
}

func TestContextBuilderFindsSourceMarkerInParentDir(t *testing.T) {
	sourceRoot := t.TempDir()
	workspace := filepath.Join(sourceRoot, "sub", "project")
	require.NoError(t, os.MkdirAll(workspace, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(sourceRoot, maxclawSourceMarkerFile), []byte("marker"), 0644))

	builder := NewContextBuilder(workspace)
	messages := builder.BuildMessages(nil, "hello", nil, "telegram", "123")
	require.NotEmpty(t, messages)

	systemPrompt := messages[0].Content
	assert.Contains(t, systemPrompt, "**Maxclaw Source Directory**: "+sourceRoot)
	assert.Contains(t, systemPrompt, filepath.Join(sourceRoot, maxclawSourceMarkerFile))
	assert.Contains(t, systemPrompt, "**Maxclaw Source Marker Found**: yes")
}

func TestContextBuilderUsesEnvMaxclawSourceDir(t *testing.T) {
	workspace := t.TempDir()
	sourceRoot := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(sourceRoot, maxclawSourceMarkerFile), []byte("marker"), 0644))
	t.Setenv("MAXCLAW_SOURCE_DIR", sourceRoot)

	builder := NewContextBuilder(workspace)
	messages := builder.BuildMessages(nil, "hello", nil, "telegram", "123")
	require.NotEmpty(t, messages)

	systemPrompt := messages[0].Content
	assert.Contains(t, systemPrompt, "**Maxclaw Source Directory**: "+sourceRoot)
	assert.Contains(t, systemPrompt, filepath.Join(sourceRoot, maxclawSourceMarkerFile))
	assert.Contains(t, systemPrompt, "**Maxclaw Source Marker Found**: yes")
}

func TestContextBuilderSupportsLegacyEnvSourceDir(t *testing.T) {
	workspace := t.TempDir()
	sourceRoot := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(sourceRoot, legacySourceMarkerFile), []byte("marker"), 0644))
	t.Setenv("NANOBOT_SOURCE_DIR", sourceRoot)

	builder := NewContextBuilder(workspace)
	messages := builder.BuildMessages(nil, "hello", nil, "telegram", "123")
	require.NotEmpty(t, messages)

	systemPrompt := messages[0].Content
	assert.Contains(t, systemPrompt, "**Maxclaw Source Directory**: "+sourceRoot)
	assert.Contains(t, systemPrompt, filepath.Join(sourceRoot, legacySourceMarkerFile))
	assert.Contains(t, systemPrompt, "**Maxclaw Source Marker Found**: yes")
}

func TestContextBuilderFindsSourceMarkerFromSearchRootsEnv(t *testing.T) {
	workspace := t.TempDir()
	searchRoot := t.TempDir()
	sourceRoot := filepath.Join(searchRoot, "repos", "maxclaw")
	require.NoError(t, os.MkdirAll(sourceRoot, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(sourceRoot, maxclawSourceMarkerFile), []byte("marker"), 0644))
	t.Setenv(maxclawSourceSearchRootsEnv, searchRoot)

	builder := NewContextBuilder(workspace)
	messages := builder.BuildMessages(nil, "hello", nil, "telegram", "123")
	require.NotEmpty(t, messages)

	systemPrompt := messages[0].Content
	assert.Contains(t, systemPrompt, "**Maxclaw Source Directory**: "+sourceRoot)
	assert.Contains(t, systemPrompt, filepath.Join(sourceRoot, maxclawSourceMarkerFile))
	assert.Contains(t, systemPrompt, "**Maxclaw Source Marker Found**: yes")
}

func TestContextBuilderSystemPromptMentionsSelfImproveCommands(t *testing.T) {
	workspace := t.TempDir()
	builder := NewContextBuilder(workspace)

	messages := builder.BuildMessages(nil, "hello", nil, "telegram", "123")
	require.NotEmpty(t, messages)

	systemPrompt := messages[0].Content
	assert.Contains(t, systemPrompt, "`claude`")
	assert.Contains(t, systemPrompt, "`codex`")
	assert.Contains(t, systemPrompt, maxclawSourceMarkerFile)
}

func TestContextBuilderDiscoversProjectContextFilesRecursively(t *testing.T) {
	sourceRoot := t.TempDir()
	workspace := filepath.Join(sourceRoot, "apps", "desktop")
	require.NoError(t, os.MkdirAll(workspace, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(sourceRoot, maxclawSourceMarkerFile), []byte("marker"), 0644))

	require.NoError(t, os.WriteFile(filepath.Join(sourceRoot, "AGENTS.md"), []byte("# Root Agents"), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(sourceRoot, "packages", "api"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(sourceRoot, "packages", "api", "CLAUDE.md"), []byte("# API Context"), 0644))

	builder := NewContextBuilder(workspace)
	messages := builder.BuildMessages(nil, "hello", nil, "telegram", "123")
	require.NotEmpty(t, messages)

	systemPrompt := messages[0].Content
	assert.Contains(t, systemPrompt, "## Project Context Files")
	assert.Contains(t, systemPrompt, "AGENTS.md (root)")
	assert.Contains(t, systemPrompt, filepath.Join("packages", "api", "CLAUDE.md"))
	assert.Contains(t, systemPrompt, "### AGENTS.md")
	assert.Contains(t, systemPrompt, "# Root Agents")
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

func TestCommonSourceSearchPathsCoverStandardLocations(t *testing.T) {
	roots := commonSourceSearchRoots()
	patterns := commonSourceSearchRootPatterns()

	assert.Contains(t, roots, "/usr/local/src")
	assert.Contains(t, roots, "/usr/src")
	assert.Contains(t, roots, "/root/git")
	assert.Contains(t, roots, "/data/git")
	assert.Contains(t, patterns, "/Users/*/git")
	assert.Contains(t, patterns, "/home/*/git")
	assert.Contains(t, patterns, "/data/*/git")
}
