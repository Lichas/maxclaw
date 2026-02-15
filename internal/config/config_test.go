package config

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.NotEmpty(t, cfg.Agents.Defaults.Workspace)
	assert.Equal(t, "anthropic/claude-opus-4-5", cfg.Agents.Defaults.Model)
	assert.Equal(t, 8192, cfg.Agents.Defaults.MaxTokens)
	assert.Equal(t, 0.7, cfg.Agents.Defaults.Temperature)
	assert.Equal(t, 20, cfg.Agents.Defaults.MaxToolIterations)

	assert.Equal(t, "0.0.0.0", cfg.Gateway.Host)
	assert.Equal(t, 18890, cfg.Gateway.Port)

	assert.False(t, cfg.Tools.RestrictToWorkspace)
	assert.Equal(t, 5, cfg.Tools.Web.Search.MaxResults)
	assert.Equal(t, 60, cfg.Tools.Exec.Timeout)
}

func TestGetAPIKey(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Providers.OpenRouter.APIKey = "openrouter-key"
	cfg.Providers.Anthropic.APIKey = "anthropic-key"
	cfg.Providers.OpenAI.APIKey = "openai-key"

	tests := []struct {
		model    string
		expected string
	}{
		{"openrouter/gpt-4", "openrouter-key"},
		{"anthropic/claude-3", "anthropic-key"},
		{"claude-3-opus", "anthropic-key"},
		{"gpt-4", "openai-key"},
		{"openai/gpt-3.5", "openai-key"},
		{"unknown-model", "openrouter-key"}, // fallback to first available
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			got := cfg.GetAPIKey(tt.model)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestGetAPIKeyNoKeys(t *testing.T) {
	cfg := DefaultConfig()
	assert.Empty(t, cfg.GetAPIKey("any-model"))
}

func TestGetAPIBase(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Providers.VLLM.APIBase = "http://localhost:8000/v1"

	tests := []struct {
		model    string
		expected string
	}{
		{"openrouter/gpt-4", "https://openrouter.ai/api/v1"},
		{"vllm/llama-3", "http://localhost:8000/v1"},
		{"anthropic/claude", ""},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			got := cfg.GetAPIBase(tt.model)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestWorkspacePath(t *testing.T) {
	cfg := DefaultConfig()
	path := cfg.Agents.Defaults.Workspace
	assert.NotEmpty(t, path)
}

func TestLoadSaveConfig(t *testing.T) {
	// 使用临时目录
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// 测试加载不存在的配置
	cfg, err := LoadConfig()
	require.NoError(t, err)
	assert.NotNil(t, cfg)

	// 修改配置并保存
	cfg.Providers.OpenRouter.APIKey = "test-key"
	cfg.Agents.Defaults.Model = "test-model"
	err = SaveConfig(cfg)
	require.NoError(t, err)

	// 重新加载
	loaded, err := LoadConfig()
	require.NoError(t, err)
	assert.Equal(t, "test-key", loaded.Providers.OpenRouter.APIKey)
	assert.Equal(t, "test-model", loaded.Agents.Defaults.Model)
}

func TestLoadConfigExpandsWorkspace(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	configDir := GetConfigDir()
	require.NoError(t, os.MkdirAll(configDir, 0755))

	tests := []struct {
		name  string
		value string
		want  string
	}{
		{"tilde", "~/ws", filepath.Join(tmpDir, "ws")},
		{"env", "$HOME/ws2", filepath.Join(tmpDir, "ws2")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := fmt.Sprintf(`{"agents":{"defaults":{"workspace":%q}}}`, tt.value)
			require.NoError(t, os.WriteFile(GetConfigPath(), []byte(raw), 0600))

			loaded, err := LoadConfig()
			require.NoError(t, err)
			assert.Equal(t, tt.want, loaded.Agents.Defaults.Workspace)
		})
	}
}

func TestEnsureWorkspace(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	err := EnsureWorkspace()
	require.NoError(t, err)
	assert.DirExists(t, GetWorkspacePath())
}

func TestCreateWorkspaceTemplates(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	err := EnsureWorkspace()
	require.NoError(t, err)

	err = CreateWorkspaceTemplates()
	require.NoError(t, err)

	workspace := GetWorkspacePath()
	assert.FileExists(t, filepath.Join(workspace, "AGENTS.md"))
	assert.FileExists(t, filepath.Join(workspace, "SOUL.md"))
	assert.FileExists(t, filepath.Join(workspace, "USER.md"))
	assert.FileExists(t, filepath.Join(workspace, "skills", "README.md"))
	assert.FileExists(t, filepath.Join(workspace, "skills", "example", "SKILL.md"))
	assert.FileExists(t, filepath.Join(workspace, "memory", "MEMORY.md"))
	assert.FileExists(t, filepath.Join(workspace, "memory", "heartbeat.md"))
}
