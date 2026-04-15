package providers

import "strings"

const (
	providerKindCompatOpenAI = "compat-openai"
	providerKindOpenAI       = "openai"
	providerKindAnthropic    = "anthropic"
)

// NewProvider creates the appropriate runtime provider implementation for the
// configured model/provider pair.
func NewProvider(apiKey, apiBase, apiFormat, defaultModel string, maxTokens int, temperature float64, supportsImageInput func(model string) bool) (LLMProvider, error) {
	switch ResolveProviderKind(defaultModel, apiBase, apiFormat) {
	case providerKindAnthropic:
		return NewAnthropicProvider(apiKey, apiBase, defaultModel, maxTokens, temperature, supportsImageInput)
	case providerKindOpenAI:
		return NewOpenAIOfficialProvider(apiKey, apiBase, defaultModel, maxTokens, temperature, supportsImageInput)
	default:
		return NewOpenAIProvider(apiKey, apiBase, defaultModel, maxTokens, temperature, supportsImageInput)
	}
}

// ResolveProviderKind returns the concrete provider implementation kind to use
// at runtime.
func ResolveProviderKind(model, apiBase, apiFormat string) string {
	providerName := DetectProviderName(model)
	if providerName == "unknown" {
		providerName = DetectProviderNameFromAPIBase(apiBase)
	}

	// Kimi For Coding API (api.kimi.com/coding) requires Anthropic Messages API format.
	// See: https://www.kimi.com/coding/docs/third-party-agents.html
	if strings.Contains(strings.ToLower(strings.TrimSpace(apiBase)), "api.kimi.com/coding") {
		return providerKindAnthropic
	}

	switch providerName {
	case "anthropic":
		if strings.EqualFold(strings.TrimSpace(apiFormat), "anthropic") || apiFormat == "" {
			return providerKindAnthropic
		}
	case "openai":
		if strings.EqualFold(strings.TrimSpace(apiFormat), "openai") || apiFormat == "" {
			return providerKindOpenAI
		}
	}

	if strings.EqualFold(strings.TrimSpace(apiFormat), "anthropic") && DetectProviderNameFromAPIBase(apiBase) == "anthropic" {
		return providerKindAnthropic
	}
	if strings.EqualFold(strings.TrimSpace(apiFormat), "openai") && DetectProviderNameFromAPIBase(apiBase) == "openai" {
		return providerKindOpenAI
	}

	return providerKindCompatOpenAI
}

// DetectProviderNameFromAPIBase infers the provider family from the configured
// API base.
func DetectProviderNameFromAPIBase(apiBase string) string {
	base := strings.ToLower(strings.TrimSpace(apiBase))
	switch {
	case strings.Contains(base, "openrouter.ai"):
		return "openrouter"
	case strings.Contains(base, "api.anthropic.com"):
		return "anthropic"
	case strings.Contains(base, "api.openai.com"):
		return "openai"
	case strings.Contains(base, "deepseek.com"):
		return "deepseek"
	case strings.Contains(base, "bigmodel.cn"):
		return "zhipu"
	case strings.Contains(base, "groq.com"):
		return "groq"
	case strings.Contains(base, "generativelanguage.googleapis.com"):
		return "gemini"
	case strings.Contains(base, "dashscope.aliyuncs.com"):
		return "dashscope"
	case strings.Contains(base, "moonshot"):
		return "moonshot"
	case strings.Contains(base, "minimax"):
		return "minimax"
	default:
		return "unknown"
	}
}

func normalizeModelForProvider(providerName, model string) string {
	trimmed := strings.TrimSpace(model)
	if trimmed == "" {
		return trimmed
	}

	parts := strings.SplitN(trimmed, "/", 2)
	if len(parts) == 2 && strings.EqualFold(parts[0], providerName) {
		return parts[1]
	}
	// 对于未知 provider（如自定义 provider），也去掉前缀（xiaomi/mimo-v2-flash → mimo-v2-flash）
	if providerName == "unknown" && len(parts) == 2 {
		return parts[1]
	}
	return trimmed
}

func normalizeAnthropicBaseURL(apiBase string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(apiBase), "/")
	if trimmed == "" {
		return "https://api.anthropic.com"
	}
	if strings.HasSuffix(trimmed, "/v1") {
		return strings.TrimSuffix(trimmed, "/v1")
	}
	return trimmed
}
