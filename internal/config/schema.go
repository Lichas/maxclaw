package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/Lichas/nanobot-go/internal/providers"
)

// ProviderConfig  LLM 提供商配置
type ProviderConfig struct {
	APIKey  string `json:"apiKey" mapstructure:"apiKey"`
	APIBase string `json:"apiBase,omitempty" mapstructure:"apiBase"`
}

// ChannelsConfig 聊天频道配置
type ChannelsConfig struct {
	Telegram  TelegramConfig  `json:"telegram" mapstructure:"telegram"`
	Discord   DiscordConfig   `json:"discord" mapstructure:"discord"`
	WhatsApp  WhatsAppConfig  `json:"whatsapp" mapstructure:"whatsapp"`
	WebSocket WebSocketConfig `json:"websocket" mapstructure:"websocket"`
}

// TelegramConfig Telegram 配置
type TelegramConfig struct {
	Enabled   bool     `json:"enabled" mapstructure:"enabled"`
	Token     string   `json:"token" mapstructure:"token"`
	AllowFrom []string `json:"allowFrom" mapstructure:"allowFrom"`
	Proxy     string   `json:"proxy,omitempty" mapstructure:"proxy"`
}

// DiscordConfig Discord 配置
type DiscordConfig struct {
	Enabled   bool     `json:"enabled" mapstructure:"enabled"`
	Token     string   `json:"token" mapstructure:"token"`
	AllowFrom []string `json:"allowFrom" mapstructure:"allowFrom"`
}

// WhatsAppConfig WhatsApp 配置
type WhatsAppConfig struct {
	Enabled     bool     `json:"enabled" mapstructure:"enabled"`
	BridgeURL   string   `json:"bridgeUrl,omitempty" mapstructure:"bridgeUrl"`
	BridgeToken string   `json:"bridgeToken,omitempty" mapstructure:"bridgeToken"`
	AllowFrom   []string `json:"allowFrom" mapstructure:"allowFrom"`
	AllowSelf   bool     `json:"allowSelf,omitempty" mapstructure:"allowSelf"`
}

// WebSocketConfig WebSocket 频道配置
type WebSocketConfig struct {
	Enabled      bool     `json:"enabled" mapstructure:"enabled"`
	Host         string   `json:"host,omitempty" mapstructure:"host"`
	Port         int      `json:"port,omitempty" mapstructure:"port"`
	Path         string   `json:"path,omitempty" mapstructure:"path"`
	AllowOrigins []string `json:"allowOrigins,omitempty" mapstructure:"allowOrigins"`
}

// AgentDefaults 默认代理配置
type AgentDefaults struct {
	Workspace         string  `json:"workspace" mapstructure:"workspace"`
	Model             string  `json:"model" mapstructure:"model"`
	MaxTokens         int     `json:"maxTokens" mapstructure:"maxTokens"`
	Temperature       float64 `json:"temperature" mapstructure:"temperature"`
	MaxToolIterations int     `json:"maxToolIterations" mapstructure:"maxToolIterations"`
}

// AgentsConfig 代理配置
type AgentsConfig struct {
	Defaults AgentDefaults `json:"defaults" mapstructure:"defaults"`
}

// WebSearchConfig 网页搜索配置
type WebSearchConfig struct {
	APIKey     string `json:"apiKey" mapstructure:"apiKey"`
	MaxResults int    `json:"maxResults" mapstructure:"maxResults"`
}

// WebFetchConfig 网页抓取配置
type WebFetchConfig struct {
	Mode       string `json:"mode" mapstructure:"mode"`
	NodePath   string `json:"nodePath,omitempty" mapstructure:"nodePath"`
	ScriptPath string `json:"scriptPath,omitempty" mapstructure:"scriptPath"`
	Timeout    int    `json:"timeout,omitempty" mapstructure:"timeout"`
	UserAgent  string `json:"userAgent,omitempty" mapstructure:"userAgent"`
	WaitUntil  string `json:"waitUntil,omitempty" mapstructure:"waitUntil"`
}

// WebToolsConfig Web 工具配置
type WebToolsConfig struct {
	Search WebSearchConfig `json:"search" mapstructure:"search"`
	Fetch  WebFetchConfig  `json:"fetch" mapstructure:"fetch"`
}

// MCPServerConfig MCP 服务器配置（兼容 Claude Desktop / Cursor）
type MCPServerConfig struct {
	Command string            `json:"command,omitempty" mapstructure:"command"`
	Args    []string          `json:"args,omitempty" mapstructure:"args"`
	Env     map[string]string `json:"env,omitempty" mapstructure:"env"`
	URL     string            `json:"url,omitempty" mapstructure:"url"`
}

// ExecToolConfig Shell 执行配置
type ExecToolConfig struct {
	Timeout int `json:"timeout" mapstructure:"timeout"`
}

// ToolsConfig 工具配置
type ToolsConfig struct {
	Web                 WebToolsConfig             `json:"web" mapstructure:"web"`
	Exec                ExecToolConfig             `json:"exec" mapstructure:"exec"`
	RestrictToWorkspace bool                       `json:"restrictToWorkspace" mapstructure:"restrictToWorkspace"`
	MCPServers          map[string]MCPServerConfig `json:"mcpServers,omitempty" mapstructure:"mcpServers"`
}

// GatewayConfig 网关配置
type GatewayConfig struct {
	Host string `json:"host" mapstructure:"host"`
	Port int    `json:"port" mapstructure:"port"`
}

// ProvidersConfig 所有 LLM 提供商配置
type ProvidersConfig struct {
	OpenRouter ProviderConfig `json:"openrouter" mapstructure:"openrouter"`
	Anthropic  ProviderConfig `json:"anthropic" mapstructure:"anthropic"`
	OpenAI     ProviderConfig `json:"openai" mapstructure:"openai"`
	DeepSeek   ProviderConfig `json:"deepseek" mapstructure:"deepseek"`
	Groq       ProviderConfig `json:"groq" mapstructure:"groq"`
	Gemini     ProviderConfig `json:"gemini" mapstructure:"gemini"`
	DashScope  ProviderConfig `json:"dashscope" mapstructure:"dashscope"`
	Moonshot   ProviderConfig `json:"moonshot" mapstructure:"moonshot"`
	MiniMax    ProviderConfig `json:"minimax" mapstructure:"minimax"`
	VLLM       ProviderConfig `json:"vllm" mapstructure:"vllm"`
}

// Config 根配置
type Config struct {
	Agents    AgentsConfig    `json:"agents" mapstructure:"agents"`
	Channels  ChannelsConfig  `json:"channels" mapstructure:"channels"`
	Providers ProvidersConfig `json:"providers" mapstructure:"providers"`
	Gateway   GatewayConfig   `json:"gateway" mapstructure:"gateway"`
	Tools     ToolsConfig     `json:"tools" mapstructure:"tools"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	homeDir, _ := os.UserHomeDir()
	workspace := filepath.Join(homeDir, ".nanobot", "workspace")

	return &Config{
		Agents: AgentsConfig{
			Defaults: AgentDefaults{
				Workspace:         workspace,
				Model:             "anthropic/claude-opus-4-5",
				MaxTokens:         8192,
				Temperature:       0.7,
				MaxToolIterations: 20,
			},
		},
		Channels: ChannelsConfig{
			Telegram: TelegramConfig{
				Enabled:   false,
				AllowFrom: []string{},
			},
			Discord: DiscordConfig{
				Enabled:   false,
				AllowFrom: []string{},
			},
			WhatsApp: WhatsAppConfig{
				Enabled:     false,
				BridgeURL:   "ws://localhost:3001",
				BridgeToken: "",
				AllowFrom:   []string{},
				AllowSelf:   false,
			},
			WebSocket: WebSocketConfig{
				Enabled:      false,
				Host:         "0.0.0.0",
				Port:         18791,
				Path:         "/ws",
				AllowOrigins: []string{},
			},
		},
		Providers: ProvidersConfig{},
		Gateway: GatewayConfig{
			Host: "0.0.0.0",
			Port: 18890,
		},
		Tools: ToolsConfig{
			Web: WebToolsConfig{
				Search: WebSearchConfig{
					MaxResults: 5,
				},
				Fetch: WebFetchConfig{
					Mode:      "http",
					Timeout:   30,
					UserAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
					WaitUntil: "domcontentloaded",
				},
			},
			Exec: ExecToolConfig{
				Timeout: 60,
			},
			RestrictToWorkspace: false,
			MCPServers:          map[string]MCPServerConfig{},
		},
	}
}

// GetAPIKey 根据模型名称获取 API Key
func (c *Config) GetAPIKey(model string) string {
	if model == "" {
		model = c.Agents.Defaults.Model
	}
	model = strings.ToLower(model)

	providerMap := c.providerConfigMap()

	for _, spec := range providers.ProviderSpecs {
		if spec.MatchesModel(model) {
			if cfg, ok := providerMap[spec.Name]; ok && cfg.APIKey != "" {
				return cfg.APIKey
			}
		}
	}

	// Fallback: 按 ProviderSpecs 声明顺序返回第一个可用 key
	for _, spec := range providers.ProviderSpecs {
		if cfg, ok := providerMap[spec.Name]; ok && cfg.APIKey != "" {
			return cfg.APIKey
		}
	}

	return ""
}

// GetAPIBase 根据模型名称获取 API Base URL
func (c *Config) GetAPIBase(model string) string {
	if model == "" {
		model = c.Agents.Defaults.Model
	}
	model = strings.ToLower(model)

	providerMap := c.providerConfigMap()
	for _, spec := range providers.ProviderSpecs {
		if !spec.MatchesModel(model) {
			continue
		}
		if cfg, ok := providerMap[spec.Name]; ok && cfg.APIBase != "" {
			return cfg.APIBase
		}
		if spec.DefaultAPIBase != "" {
			return spec.DefaultAPIBase
		}
		return ""
	}

	return ""
}

func (c *Config) providerConfigMap() map[string]ProviderConfig {
	out := make(map[string]ProviderConfig)
	val := reflect.ValueOf(c.Providers)
	typ := reflect.TypeOf(c.Providers)
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		tag := field.Tag.Get("json")
		name := strings.Split(tag, ",")[0]
		if name == "" || name == "-" {
			continue
		}
		cfg, ok := val.Field(i).Interface().(ProviderConfig)
		if !ok {
			continue
		}
		out[name] = cfg
	}
	return out
}
