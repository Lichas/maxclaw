package providers

import "strings"

type ProviderSpec struct {
	Name           string
	Keywords       []string
	DefaultAPIBase string
}

func (s ProviderSpec) MatchesModel(model string) bool {
	model = strings.ToLower(model)
	for _, kw := range s.Keywords {
		if strings.Contains(model, strings.ToLower(kw)) {
			return true
		}
	}
	return false
}

// ProviderSpecs defines provider detection order and default API base behavior.
// Adding a new provider should only require:
// 1) add ProvidersConfig field in config/schema.go
// 2) append one ProviderSpec here
var ProviderSpecs = []ProviderSpec{
	{Name: "openrouter", Keywords: []string{"openrouter"}, DefaultAPIBase: "https://openrouter.ai/api/v1"},
	{Name: "deepseek", Keywords: []string{"deepseek"}, DefaultAPIBase: "https://api.deepseek.com/v1"},
	{Name: "zhipu", Keywords: []string{"zhipu", "glm", "zai"}, DefaultAPIBase: "https://open.bigmodel.cn/api/coding/paas/v4"},
	{Name: "anthropic", Keywords: []string{"anthropic", "claude"}},
	{Name: "openai", Keywords: []string{"openai", "gpt"}},
	{Name: "gemini", Keywords: []string{"gemini"}},
	{Name: "dashscope", Keywords: []string{"dashscope", "qwen"}, DefaultAPIBase: "https://dashscope.aliyuncs.com/compatible-mode/v1"},
	{Name: "groq", Keywords: []string{"groq"}},
	{Name: "moonshot", Keywords: []string{"moonshot", "kimi"}, DefaultAPIBase: "https://api.moonshot.ai/v1"},
	{Name: "minimax", Keywords: []string{"minimax"}, DefaultAPIBase: "https://api.minimax.io/v1"},
	{Name: "vllm", Keywords: []string{"vllm"}},
}
