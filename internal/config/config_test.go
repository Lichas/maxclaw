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
	assert.Equal(t, "http", cfg.Tools.Web.Fetch.Mode)
	assert.Equal(t, "chrome", cfg.Tools.Web.Fetch.Chrome.ProfileName)
	assert.Equal(t, "chrome", cfg.Tools.Web.Fetch.Chrome.Channel)
	assert.True(t, cfg.Tools.Web.Fetch.Chrome.Headless)
	assert.True(t, cfg.Tools.Web.Fetch.Chrome.AutoStartCDP)
	assert.Equal(t, 15000, cfg.Tools.Web.Fetch.Chrome.LaunchTimeoutMs)
	assert.Equal(t, 60, cfg.Tools.Exec.Timeout)
	assert.Empty(t, cfg.Tools.MCPServers)

	assert.False(t, cfg.Channels.Slack.Enabled)
	assert.False(t, cfg.Channels.Email.Enabled)
	assert.False(t, cfg.Channels.QQ.Enabled)
	assert.False(t, cfg.Channels.Feishu.Enabled)
}

func TestGetAPIKey(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Providers.OpenRouter.APIKey = "openrouter-key"
	cfg.Providers.Anthropic.APIKey = "anthropic-key"
	cfg.Providers.OpenAI.APIKey = "openai-key"
	cfg.Providers.MiniMax.APIKey = "minimax-key"
	cfg.Providers.DashScope.APIKey = "dashscope-key"
	cfg.Providers.Zhipu.APIKey = "zhipu-key"

	tests := []struct {
		model    string
		expected string
	}{
		{"openrouter/gpt-4", "openrouter-key"},
		{"anthropic/claude-3", "anthropic-key"},
		{"claude-3-opus", "anthropic-key"},
		{"gpt-4", "openai-key"},
		{"openai/gpt-3.5", "openai-key"},
		{"minimax/MiniMax-M2", "minimax-key"},
		{"qwen-max", "dashscope-key"},
		{"glm-4.5", "zhipu-key"},
		{"zai/glm-5", "zhipu-key"},
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
	cfg.Providers.MiniMax.APIBase = "https://api.minimaxi.com/v1"
	cfg.Providers.Moonshot.APIBase = "https://api.moonshot.ai/v1"
	cfg.Providers.DashScope.APIBase = "https://dashscope.custom/v1"
	cfg.Providers.Zhipu.APIBase = "https://open.bigmodel.cn/api/coding/paas/v4"

	tests := []struct {
		model    string
		expected string
	}{
		{"openrouter/gpt-4", "https://openrouter.ai/api/v1"},
		{"vllm/llama-3", "http://localhost:8000/v1"},
		{"meta-llama/Llama-3.1-8B-Instruct", "http://localhost:8000/v1"},
		{"kimi-k2.5", "https://api.moonshot.ai/v1"},
		{"minimax/MiniMax-M2", "https://api.minimaxi.com/v1"},
		{"qwen-max", "https://dashscope.custom/v1"},
		{"minimax/another-model", "https://api.minimaxi.com/v1"},
		{"glm-4.5", "https://open.bigmodel.cn/api/coding/paas/v4"},
		{"zai/glm-5", "https://open.bigmodel.cn/api/coding/paas/v4"},
		{"anthropic/claude", ""},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			got := cfg.GetAPIBase(tt.model)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestGetAPIBaseMiniMaxDefault(t *testing.T) {
	cfg := DefaultConfig()
	got := cfg.GetAPIBase("minimax/MiniMax-M2")
	assert.Equal(t, "https://api.minimax.io/v1", got)
}

func TestGetAPIBaseDashScopeDefault(t *testing.T) {
	cfg := DefaultConfig()
	got := cfg.GetAPIBase("qwen-max")
	assert.Equal(t, "https://dashscope.aliyuncs.com/compatible-mode/v1", got)
}

func TestGetAPIBaseDeepSeekDefault(t *testing.T) {
	cfg := DefaultConfig()
	got := cfg.GetAPIBase("deepseek/deepseek-chat")
	assert.Equal(t, "https://api.deepseek.com/v1", got)
}

func TestGetAPIBaseMoonshotDefault(t *testing.T) {
	cfg := DefaultConfig()
	got := cfg.GetAPIBase("kimi-k2.5")
	assert.Equal(t, "https://api.moonshot.ai/v1", got)
}

func TestGetAPIBaseZhipuDefault(t *testing.T) {
	cfg := DefaultConfig()
	got := cfg.GetAPIBase("glm-4.5")
	assert.Equal(t, "https://open.bigmodel.cn/api/coding/paas/v4", got)
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
	cfg.Channels.WhatsApp.BridgeToken = "bridge-secret"
	err = SaveConfig(cfg)
	require.NoError(t, err)

	// 重新加载
	loaded, err := LoadConfig()
	require.NoError(t, err)
	assert.Equal(t, "test-key", loaded.Providers.OpenRouter.APIKey)
	assert.Equal(t, "test-model", loaded.Agents.Defaults.Model)
	assert.Equal(t, "bridge-secret", loaded.Channels.WhatsApp.BridgeToken)
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

func TestLoadConfigMCPServersCompatibility(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	configDir := GetConfigDir()
	require.NoError(t, os.MkdirAll(configDir, 0755))

	raw := `{
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
    }
  }
}`
	require.NoError(t, os.WriteFile(GetConfigPath(), []byte(raw), 0600))

	loaded, err := LoadConfig()
	require.NoError(t, err)
	require.Contains(t, loaded.Tools.MCPServers, "filesystem")
	assert.Equal(t, "npx", loaded.Tools.MCPServers["filesystem"].Command)
	assert.Equal(t, []string{"-y", "@modelcontextprotocol/server-filesystem", "/tmp"}, loaded.Tools.MCPServers["filesystem"].Args)
}

func TestLoadConfigMCPServersToolsOverrideTopLevel(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	configDir := GetConfigDir()
	require.NoError(t, os.MkdirAll(configDir, 0755))

	raw := `{
  "mcpServers": {
    "filesystem": { "command": "top-level-cmd" }
  },
  "tools": {
    "mcpServers": {
      "filesystem": { "command": "tools-cmd" }
    }
  }
}`
	require.NoError(t, os.WriteFile(GetConfigPath(), []byte(raw), 0600))

	loaded, err := LoadConfig()
	require.NoError(t, err)
	require.Contains(t, loaded.Tools.MCPServers, "filesystem")
	assert.Equal(t, "tools-cmd", loaded.Tools.MCPServers["filesystem"].Command)
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
	assert.FileExists(t, filepath.Join(workspace, "memory", "HISTORY.md"))
	assert.FileExists(t, filepath.Join(workspace, "memory", "heartbeat.md"))
}
