package config

import (
	"reflect"
	"strings"

	"github.com/Lichas/maxclaw/internal/providers"
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
	Slack     SlackConfig     `json:"slack" mapstructure:"slack"`
	Email     EmailConfig     `json:"email" mapstructure:"email"`
	QQ        QQConfig        `json:"qq" mapstructure:"qq"`
	Feishu    FeishuConfig    `json:"feishu" mapstructure:"feishu"`
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

// SlackConfig Slack Socket Mode 配置
type SlackConfig struct {
	Enabled   bool     `json:"enabled" mapstructure:"enabled"`
	BotToken  string   `json:"botToken,omitempty" mapstructure:"botToken"`
	AppToken  string   `json:"appToken,omitempty" mapstructure:"appToken"`
	AllowFrom []string `json:"allowFrom" mapstructure:"allowFrom"`
}

// EmailConfig Email(IMAP/SMTP) 配置
type EmailConfig struct {
	Enabled             bool     `json:"enabled" mapstructure:"enabled"`
	ConsentGranted      bool     `json:"consentGranted" mapstructure:"consentGranted"`
	IMAPHost            string   `json:"imapHost,omitempty" mapstructure:"imapHost"`
	IMAPPort            int      `json:"imapPort,omitempty" mapstructure:"imapPort"`
	IMAPUsername        string   `json:"imapUsername,omitempty" mapstructure:"imapUsername"`
	IMAPPassword        string   `json:"imapPassword,omitempty" mapstructure:"imapPassword"`
	IMAPMailbox         string   `json:"imapMailbox,omitempty" mapstructure:"imapMailbox"`
	IMAPUseSSL          bool     `json:"imapUseSSL,omitempty" mapstructure:"imapUseSSL"`
	SMTPHost            string   `json:"smtpHost,omitempty" mapstructure:"smtpHost"`
	SMTPPort            int      `json:"smtpPort,omitempty" mapstructure:"smtpPort"`
	SMTPUsername        string   `json:"smtpUsername,omitempty" mapstructure:"smtpUsername"`
	SMTPPassword        string   `json:"smtpPassword,omitempty" mapstructure:"smtpPassword"`
	SMTPUseTLS          bool     `json:"smtpUseTLS,omitempty" mapstructure:"smtpUseTLS"`
	SMTPUseSSL          bool     `json:"smtpUseSSL,omitempty" mapstructure:"smtpUseSSL"`
	FromAddress         string   `json:"fromAddress,omitempty" mapstructure:"fromAddress"`
	AutoReplyEnabled    bool     `json:"autoReplyEnabled,omitempty" mapstructure:"autoReplyEnabled"`
	PollIntervalSeconds int      `json:"pollIntervalSeconds,omitempty" mapstructure:"pollIntervalSeconds"`
	MarkSeen            bool     `json:"markSeen,omitempty" mapstructure:"markSeen"`
	AllowFrom           []string `json:"allowFrom" mapstructure:"allowFrom"`
}

// QQConfig QQ 私聊配置（OneBot WebSocket）
type QQConfig struct {
	Enabled     bool     `json:"enabled" mapstructure:"enabled"`
	WSURL       string   `json:"wsUrl,omitempty" mapstructure:"wsUrl"`
	AccessToken string   `json:"accessToken,omitempty" mapstructure:"accessToken"`
	AllowFrom   []string `json:"allowFrom" mapstructure:"allowFrom"`
}

// FeishuConfig Feishu/Lark 配置
type FeishuConfig struct {
	Enabled           bool     `json:"enabled" mapstructure:"enabled"`
	AppID             string   `json:"appId,omitempty" mapstructure:"appId"`
	AppSecret         string   `json:"appSecret,omitempty" mapstructure:"appSecret"`
	VerificationToken string   `json:"verificationToken,omitempty" mapstructure:"verificationToken"`
	ListenAddr        string   `json:"listenAddr,omitempty" mapstructure:"listenAddr"`
	WebhookPath       string   `json:"webhookPath,omitempty" mapstructure:"webhookPath"`
	AllowFrom         []string `json:"allowFrom" mapstructure:"allowFrom"`
}

// AgentDefaults 默认代理配置
type AgentDefaults struct {
	Workspace          string   `json:"workspace" mapstructure:"workspace"`
	Model              string   `json:"model" mapstructure:"model"`
	MaxTokens          int      `json:"maxTokens" mapstructure:"maxTokens"`
	Temperature        float64  `json:"temperature" mapstructure:"temperature"`
	MaxToolIterations  int      `json:"maxToolIterations" mapstructure:"maxToolIterations"`
	ExecutionMode      string   `json:"executionMode,omitempty" mapstructure:"executionMode"`
	EnableGlobalSkills bool     `json:"enableGlobalSkills" mapstructure:"enableGlobalSkills"`
	GlobalSkillsPaths  []string `json:"globalSkillsPaths,omitempty" mapstructure:"globalSkillsPaths"`
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
	Mode            string               `json:"mode" mapstructure:"mode"`
	NodePath        string               `json:"nodePath,omitempty" mapstructure:"nodePath"`
	ScriptPath      string               `json:"scriptPath,omitempty" mapstructure:"scriptPath"`
	Timeout         int                  `json:"timeout,omitempty" mapstructure:"timeout"`
	UserAgent       string               `json:"userAgent,omitempty" mapstructure:"userAgent"`
	WaitUntil       string               `json:"waitUntil,omitempty" mapstructure:"waitUntil"`
	RenderWaitMs    int                  `json:"renderWaitMs,omitempty" mapstructure:"renderWaitMs"`
	SmartWaitMs     int                  `json:"smartWaitMs,omitempty" mapstructure:"smartWaitMs"`
	StableWaitMs    int                  `json:"stableWaitMs,omitempty" mapstructure:"stableWaitMs"`
	WaitForSelector string               `json:"waitForSelector,omitempty" mapstructure:"waitForSelector"`
	WaitForText     string               `json:"waitForText,omitempty" mapstructure:"waitForText"`
	WaitForNoText   string               `json:"waitForNoText,omitempty" mapstructure:"waitForNoText"`
	Chrome          WebFetchChromeConfig `json:"chrome,omitempty" mapstructure:"chrome"`
}

// WebFetchChromeConfig Chrome 抓取配置
type WebFetchChromeConfig struct {
	CDPEndpoint      string `json:"cdpEndpoint,omitempty" mapstructure:"cdpEndpoint"`
	ProfileName      string `json:"profileName,omitempty" mapstructure:"profileName"`
	UserDataDir      string `json:"userDataDir,omitempty" mapstructure:"userDataDir"`
	Channel          string `json:"channel,omitempty" mapstructure:"channel"`
	Headless         bool   `json:"headless,omitempty" mapstructure:"headless"`
	AutoStartCDP     bool   `json:"autoStartCDP,omitempty" mapstructure:"autoStartCDP"`
	TakeoverExisting bool   `json:"takeoverExisting,omitempty" mapstructure:"takeoverExisting"`
	HostUserDataDir  string `json:"hostUserDataDir,omitempty" mapstructure:"hostUserDataDir"`
	LaunchTimeoutMs  int    `json:"launchTimeoutMs,omitempty" mapstructure:"launchTimeoutMs"`
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
	Headers map[string]string `json:"headers,omitempty" mapstructure:"headers"`
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
	Zhipu      ProviderConfig `json:"zhipu" mapstructure:"zhipu"`
	Groq       ProviderConfig `json:"groq" mapstructure:"groq"`
	Gemini     ProviderConfig `json:"gemini" mapstructure:"gemini"`
	DashScope  ProviderConfig `json:"dashscope" mapstructure:"dashscope"`
	Moonshot   ProviderConfig `json:"moonshot" mapstructure:"moonshot"`
	MiniMax    ProviderConfig `json:"minimax" mapstructure:"minimax"`
	VLLM       ProviderConfig `json:"vllm" mapstructure:"vllm"`
}

// ToMap 将 ProvidersConfig 转换为 map[string]ProviderConfig
func (p ProvidersConfig) ToMap() map[string]ProviderConfig {
	return map[string]ProviderConfig{
		"openrouter": p.OpenRouter,
		"anthropic":  p.Anthropic,
		"openai":     p.OpenAI,
		"deepseek":   p.DeepSeek,
		"zhipu":      p.Zhipu,
		"groq":       p.Groq,
		"gemini":     p.Gemini,
		"dashscope":  p.DashScope,
		"moonshot":   p.Moonshot,
		"minimax":    p.MiniMax,
		"vllm":       p.VLLM,
	}
}

// ProvidersConfigFromMap 从 map 创建 ProvidersConfig
func ProvidersConfigFromMap(m map[string]ProviderConfig) ProvidersConfig {
	return ProvidersConfig{
		OpenRouter: m["openrouter"],
		Anthropic:  m["anthropic"],
		OpenAI:     m["openai"],
		DeepSeek:   m["deepseek"],
		Zhipu:      m["zhipu"],
		Groq:       m["groq"],
		Gemini:     m["gemini"],
		DashScope:  m["dashscope"],
		Moonshot:   m["moonshot"],
		MiniMax:    m["minimax"],
		VLLM:       m["vllm"],
	}
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
	workspace := GetWorkspacePath()

	return &Config{
		Agents: AgentsConfig{
			Defaults: AgentDefaults{
				Workspace:          workspace,
				Model:              "anthropic/claude-opus-4-5",
				MaxTokens:          8192,
				Temperature:        0.7,
				MaxToolIterations:  200,
				ExecutionMode:      ExecutionModeAsk,
				EnableGlobalSkills: true, // 默认启用 ~/.agents/skills/
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
			Slack: SlackConfig{
				Enabled:   false,
				AllowFrom: []string{},
			},
			Email: EmailConfig{
				Enabled:             false,
				ConsentGranted:      false,
				IMAPPort:            993,
				IMAPMailbox:         "INBOX",
				IMAPUseSSL:          true,
				SMTPPort:            587,
				SMTPUseTLS:          true,
				SMTPUseSSL:          false,
				AutoReplyEnabled:    true,
				PollIntervalSeconds: 30,
				MarkSeen:            true,
				AllowFrom:           []string{},
			},
			QQ: QQConfig{
				Enabled:   false,
				WSURL:     "ws://localhost:3002",
				AllowFrom: []string{},
			},
			Feishu: FeishuConfig{
				Enabled:     false,
				ListenAddr:  "0.0.0.0:18792",
				WebhookPath: "/feishu/events",
				AllowFrom:   []string{},
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
					Mode:         "http",
					Timeout:      30,
					UserAgent:    "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
					WaitUntil:    "domcontentloaded",
					RenderWaitMs: 600,
					SmartWaitMs:  4000,
					StableWaitMs: 500,
					Chrome: WebFetchChromeConfig{
						ProfileName:     "chrome",
						Channel:         "chrome",
						Headless:        true,
						AutoStartCDP:    true,
						LaunchTimeoutMs: 15000,
					},
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
	matchedProvider := false
	for _, spec := range providers.ProviderSpecs {
		if !spec.MatchesModel(model) {
			continue
		}
		matchedProvider = true
		if cfg, ok := providerMap[spec.Name]; ok && cfg.APIBase != "" {
			return cfg.APIBase
		}
		if spec.DefaultAPIBase != "" {
			return spec.DefaultAPIBase
		}
	}

	// vLLM local models often use raw IDs like "meta-llama/Llama-3.1-8B-Instruct"
	// (without an explicit provider prefix). If such a model is configured and
	// providers.vllm.apiBase is set, route API base to vLLM.
	if !matchedProvider && looksLikeRawModelID(model) {
		if cfg, ok := providerMap["vllm"]; ok && cfg.APIBase != "" {
			return cfg.APIBase
		}
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

func looksLikeRawModelID(model string) bool {
	if model == "" || !strings.Contains(model, "/") {
		return false
	}

	prefix := strings.SplitN(model, "/", 2)[0]
	if prefix == "" {
		return false
	}

	for _, spec := range providers.ProviderSpecs {
		if strings.EqualFold(prefix, spec.Name) {
			return false
		}
	}

	return true
}
