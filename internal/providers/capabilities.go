package providers

import "strings"

// DetectProviderName infers a provider name from the configured model id.
func DetectProviderName(model string) string {
	for _, spec := range ProviderSpecs {
		if spec.MatchesModel(model) {
			return spec.Name
		}
	}
	return "unknown"
}

// SupportsImageInput reports whether the target model should receive image parts.
func SupportsImageInput(providerName, model string) bool {
	providerName = strings.ToLower(strings.TrimSpace(providerName))
	modelName := strings.ToLower(strings.TrimSpace(model))

	switch providerName {
	case "deepseek":
		return strings.Contains(modelName, "vl")
	case "openai":
		return strings.Contains(modelName, "gpt-4o") ||
			strings.Contains(modelName, "gpt-4.1") ||
			strings.HasPrefix(modelName, "o1") ||
			strings.HasPrefix(modelName, "o3")
	case "anthropic":
		return strings.Contains(modelName, "claude-3") ||
			strings.Contains(modelName, "claude-sonnet-4") ||
			strings.Contains(modelName, "claude-opus-4")
	case "zhipu":
		return strings.Contains(modelName, "glm-5") ||
			strings.Contains(modelName, "glm-4.6v") ||
			strings.Contains(modelName, "glm-ocr") ||
			strings.Contains(modelName, "vision") ||
			strings.Contains(modelName, "vl")
	case "gemini":
		return true
	case "openrouter":
		return strings.Contains(modelName, "gpt-4o") ||
			strings.Contains(modelName, "gpt-4.1") ||
			strings.Contains(modelName, "claude-3") ||
			strings.Contains(modelName, "claude-sonnet-4") ||
			strings.Contains(modelName, "gemini") ||
			strings.Contains(modelName, "vl") ||
			strings.Contains(modelName, "vision")
	default:
		return strings.Contains(modelName, "vision") || strings.Contains(modelName, "vl")
	}
}
