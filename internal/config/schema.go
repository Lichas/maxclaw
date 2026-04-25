package config

import (
	"encoding/json"
	"reflect"
	"strings"

	"github.com/Lichas/maxclaw/internal/providers"
)

// ProviderConfig  LLM 提供商配置
type ProviderConfig struct {
	APIKey    string                `json:"apiKey" mapstructure:"apiKey"`
	APIBase   string                `json:"apiBase,omitempty" mapstructure:"apiBase"`
	APIFormat string                `json:"apiFormat,omitempty" mapstructure:"apiFormat"`
	Models    []ProviderModelConfig `json:"models,omitempty" mapstructure:"models"`
}

type ProviderModelConfig struct {
	ID                 string `json:"id" mapstructure:"id"`
	Name               string `json:"name,omitempty" mapstructure:"name"`
	MaxTokens          int    `json:"maxTokens,omitempty" mapstructure:"maxTokens"`
	Enabled            bool   `json:"enabled" mapstructure:"enabled"`
	SupportsImageInput *bool  `json:"supportsImageInput,omitempty" mapstructure:"supportsImageInput"`
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

// QQConfig QQ 机器人配置（腾讯官方 QQBot）
type QQConfig struct {
	Enabled     bool     `json:"enabled" mapstructure:"enabled"`
	AppID       string   `json:"appId,omitempty" mapstructure:"appId"`
	AppSecret   string   `json:"appSecret,omitempty" mapstructure:"appSecret"`
	WSURL       string   `json:"wsUrl,omitempty" mapstructure:"wsUrl"`
	AccessToken string   `json:"accessToken,omitempty" mapstructure:"accessToken"`
	ListenAddr  string   `json:"listenAddr,omitempty" mapstructure:"listenAddr"`
	WebhookPath string   `json:"webhookPath,omitempty" mapstructure:"webhookPath"`
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
	OpenRouter ProviderConfig            `json:"openrouter" mapstructure:"openrouter"`
	Anthropic  ProviderConfig            `json:"anthropic" mapstructure:"anthropic"`
	OpenAI     ProviderConfig            `json:"openai" mapstructure:"openai"`
	DeepSeek   ProviderConfig            `json:"deepseek" mapstructure:"deepseek"`
	Zhipu      ProviderConfig            `json:"zhipu" mapstructure:"zhipu"`
	Groq       ProviderConfig            `json:"groq" mapstructure:"groq"`
	Gemini     ProviderConfig            `json:"gemini" mapstructure:"gemini"`
	DashScope  ProviderConfig            `json:"dashscope" mapstructure:"dashscope"`
	Moonshot   ProviderConfig            `json:"moonshot" mapstructure:"moonshot"`
	MiniMax    ProviderConfig            `json:"minimax" mapstructure:"minimax"`
	VLLM       ProviderConfig            `json:"vllm" mapstructure:"vllm"`
	Custom     map[string]ProviderConfig `json:"-" mapstructure:"custom"`
}

// ToMap 将 ProvidersConfig 转换为 map[string]ProviderConfig
func (p ProvidersConfig) ToMap() map[string]ProviderConfig {
	out := map[string]ProviderConfig{
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
	for k, v := range p.Custom {
		out[k] = v
	}
	return out
}

// ProvidersConfigFromMap 从 map 创建 ProvidersConfig
func ProvidersConfigFromMap(m map[string]ProviderConfig) ProvidersConfig {
	custom := make(map[string]ProviderConfig)
	known := map[string]bool{
		"openrouter": true,
		"anthropic":  true,
		"openai":     true,
		"deepseek":   true,
		"zhipu":      true,
		"groq":       true,
		"gemini":     true,
		"dashscope":  true,
		"moonshot":   true,
		"minimax":    true,
		"vllm":       true,
	}
	for k, v := range m {
		if !known[k] {
			custom[k] = v
		}
	}
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
		Custom:     custom,
	}
}

// MarshalJSON 将 ProvidersConfig 平铺序列化（自定义 provider 直接展开到 providers 下）
func (p ProvidersConfig) MarshalJSON() ([]byte, error) {
	m := p.ToMap()
	return json.Marshal(m)
}

// UnmarshalJSON 从平铺的 providers JSON 反序列化，未知 key 归入 Custom
func (p *ProvidersConfig) UnmarshalJSON(data []byte) error {
	var m map[string]ProviderConfig
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	*p = ProvidersConfigFromMap(m)
	return nil
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
				Enabled:     false,
				AppID:       "",
				AppSecret:   "",
				AccessToken: "",
				ListenAddr:  "0.0.0.0:18793",
				WebhookPath: "/qq/events",
				AllowFrom:   []string{},
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

	// 在所有已配置 provider（已知+自定义）的 models 列表中精确匹配
	if cfg, _, ok := c.findProviderForModel(model); ok && cfg.APIKey != "" {
		return cfg.APIKey
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
			return normalizeProviderAPIBase(spec.Name, model, cfg.APIBase)
		}
		if spec.DefaultAPIBase != "" {
			return normalizeProviderAPIBase(spec.Name, model, spec.DefaultAPIBase)
		}
	}

	// 在所有已配置 provider（已知+自定义）的 models 列表中精确匹配
	if !matchedProvider {
		if cfg, providerName, ok := c.findProviderForModel(model); ok && cfg.APIBase != "" {
			return normalizeProviderAPIBase(providerName, model, cfg.APIBase)
		}
	}

	// vLLM local models often use raw IDs like "meta-llama/Llama-3.1-8B-Instruct"
	// (without an explicit provider prefix). If such a model is configured and
	// providers.vllm.apiBase is set, route API base to vLLM.
	if !matchedProvider && looksLikeRawModelID(model) {
		if cfg, ok := providerMap["vllm"]; ok && cfg.APIBase != "" {
			return cfg.APIBase
		}
		// Fallback: if no vLLM base is set, align with GetAPIKey's fallback
		// behavior and use the first provider that has an apiBase configured.
		// This fixes OpenRouter models like "tencent/hy3-preview:free" that
		// lack a matching keyword in ProviderSpecs but are accessed via an
		// aggregator that is already configured (e.g. OpenRouter).
		for _, spec := range providers.ProviderSpecs {
			if cfg, ok := providerMap[spec.Name]; ok && cfg.APIBase != "" {
				return normalizeProviderAPIBase(spec.Name, model, cfg.APIBase)
			}
		}
	}

	return ""
}

// GetAPIFormat returns the wire format configured for the target provider.
func (c *Config) GetAPIFormat(model string) string {
	if model == "" {
		model = c.Agents.Defaults.Model
	}
	model = strings.ToLower(model)

	providerMap := c.providerConfigMap()
	for _, spec := range providers.ProviderSpecs {
		if !spec.MatchesModel(model) {
			continue
		}
		if cfg, ok := providerMap[spec.Name]; ok && strings.TrimSpace(cfg.APIFormat) != "" {
			return strings.ToLower(strings.TrimSpace(cfg.APIFormat))
		}
		if spec.Name == "anthropic" {
			return "anthropic"
		}
		return "openai"
	}

	// 在所有已配置 provider（已知+自定义）的 models 列表中精确匹配
	if cfg, _, ok := c.findProviderForModel(model); ok && strings.TrimSpace(cfg.APIFormat) != "" {
		return strings.ToLower(strings.TrimSpace(cfg.APIFormat))
	}

	return "openai"
}

func normalizeProviderAPIBase(providerName, model, apiBase string) string {
	normalizedModel := strings.ToLower(strings.TrimSpace(model))
	normalizedBase := strings.TrimRight(strings.TrimSpace(apiBase), "/")
	if normalizedBase == "" {
		return apiBase
	}

	switch providerName {
	case "anthropic":
		if strings.HasSuffix(normalizedBase, "/v1") {
			return strings.TrimSuffix(normalizedBase, "/v1")
		}
	case "zhipu":
		if zhipuVisionModel(normalizedModel) {
			if strings.HasSuffix(normalizedBase, "/api/coding/paas/v4") {
				return strings.TrimSuffix(normalizedBase, "/api/coding/paas/v4") + "/api/paas/v4"
			}
		}
	case "minimax":
		if strings.Contains(normalizedBase, "api.minimax.com") {
			return strings.Replace(apiBase, "api.minimax.com", "api.minimaxi.com", 1)
		}
	}

	return apiBase
}

func zhipuVisionModel(model string) bool {
	model = strings.ToLower(strings.TrimSpace(model))
	return strings.Contains(model, "glm-4.6v") ||
		strings.Contains(model, "glm-ocr") ||
		strings.Contains(model, "vision") ||
		strings.Contains(model, "vl")
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
	for k, v := range c.Providers.Custom {
		out[k] = v
	}
	return out
}

// findProviderForModel 在所有已配置 provider（已知+自定义）的 models 列表中查找指定模型。
// 返回匹配到的 provider 配置、provider 名称和是否找到。
func (c *Config) findProviderForModel(model string) (ProviderConfig, string, bool) {
	model = strings.ToLower(strings.TrimSpace(model))
	if model == "" {
		return ProviderConfig{}, "", false
	}

	providerMap := c.providerConfigMap()

	for providerName, cfg := range providerMap {
		for _, m := range cfg.Models {
			id := strings.ToLower(strings.TrimSpace(m.ID))
			if id == model {
				return cfg, providerName, true
			}
			// 支持 providerName/model 或 model 带前缀的写法
			if id == providerName+"/"+model || model == providerName+"/"+id {
				return cfg, providerName, true
			}
		}
	}
	return ProviderConfig{}, "", false
}

// findCustomProviderForModel 查找包含指定模型的自定义 provider 配置（向后兼容）。
func (c *Config) findCustomProviderForModel(model string) (ProviderConfig, string, bool) {
	cfg, name, ok := c.findProviderForModel(model)
	if !ok {
		return ProviderConfig{}, "", false
	}
	// 过滤掉已知 provider，只保留真正的自定义 provider
	for _, spec := range providers.ProviderSpecs {
		if strings.EqualFold(spec.Name, name) {
			return ProviderConfig{}, "", false
		}
	}
	return cfg, name, true
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

// SupportsImageInput reports whether the target model should receive multimodal image parts.
// Explicit per-model config wins; otherwise we fall back to provider heuristics.
func (c *Config) SupportsImageInput(model string) bool {
	if model == "" {
		model = c.Agents.Defaults.Model
	}
	if model == "" {
		return false
	}

	if configured, ok := c.lookupProviderModelConfig(model); ok && configured.SupportsImageInput != nil {
		return *configured.SupportsImageInput
	}

	return providers.SupportsImageInput(providers.DetectProviderName(model), model)
}

func (c *Config) lookupProviderModelConfig(model string) (*ProviderModelConfig, bool) {
	inputAliases := modelAliases(model)
	for _, providerCfg := range c.providerConfigMap() {
		for i := range providerCfg.Models {
			cfg := &providerCfg.Models[i]
			if aliasesOverlap(inputAliases, modelAliases(cfg.ID)) {
				return cfg, true
			}
		}
	}
	return nil, false
}

func modelAliases(model string) map[string]struct{} {
	aliases := make(map[string]struct{})
	normalized := strings.ToLower(strings.TrimSpace(model))
	if normalized == "" {
		return aliases
	}

	aliases[normalized] = struct{}{}
	if strings.Contains(normalized, "/") {
		parts := strings.SplitN(normalized, "/", 2)
		if len(parts) == 2 && parts[1] != "" {
			aliases[parts[1]] = struct{}{}
		}
	}

	if providerName := providers.DetectProviderName(normalized); providerName != "" && providerName != "unknown" {
		aliases[providerName+"/"+normalized] = struct{}{}
		prefix := providerName + "/"
		if strings.HasPrefix(normalized, prefix) {
			trimmed := strings.TrimPrefix(normalized, prefix)
			if trimmed != "" {
				aliases[trimmed] = struct{}{}
			}
		}
	}

	return aliases
}

func aliasesOverlap(left, right map[string]struct{}) bool {
	for alias := range left {
		if _, ok := right[alias]; ok {
			return true
		}
	}
	return false
}
