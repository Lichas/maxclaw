package providers

import "testing"

func TestResolveProviderKind(t *testing.T) {
	tests := []struct {
		name      string
		model     string
		apiBase   string
		apiFormat string
		expected  string
	}{
		{name: "anthropic model uses official provider", model: "anthropic/claude-sonnet-4-5", apiFormat: "anthropic", expected: providerKindAnthropic},
		{name: "openai model uses official provider", model: "openai/gpt-5.1", apiFormat: "openai", expected: providerKindOpenAI},
		{name: "openrouter stays compat", model: "openrouter/auto", apiBase: "https://openrouter.ai/api/v1", apiFormat: "openai", expected: providerKindCompatOpenAI},
		{name: "anthropic api base can recover official provider", model: "custom", apiBase: "https://api.anthropic.com", apiFormat: "anthropic", expected: providerKindAnthropic},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ResolveProviderKind(tt.model, tt.apiBase, tt.apiFormat); got != tt.expected {
				t.Fatalf("ResolveProviderKind(%q, %q, %q)=%q want %q", tt.model, tt.apiBase, tt.apiFormat, got, tt.expected)
			}
		})
	}
}

func TestNewProviderUsesOfficialImplementations(t *testing.T) {
	openaiProvider, err := NewProvider("sk-openai", "https://api.openai.com/v1", "openai", "openai/gpt-5.1", 32, 0, nil)
	if err != nil {
		t.Fatalf("NewProvider openai failed: %v", err)
	}
	if _, ok := openaiProvider.(*OpenAIOfficialProvider); !ok {
		t.Fatalf("expected OpenAIOfficialProvider, got %T", openaiProvider)
	}

	anthropicProvider, err := NewProvider("sk-anthropic", "https://api.anthropic.com", "anthropic", "anthropic/claude-sonnet-4-5", 32, 0, nil)
	if err != nil {
		t.Fatalf("NewProvider anthropic failed: %v", err)
	}
	if _, ok := anthropicProvider.(*AnthropicProvider); !ok {
		t.Fatalf("expected AnthropicProvider, got %T", anthropicProvider)
	}

	compatProvider, err := NewProvider("sk-openrouter", "https://openrouter.ai/api/v1", "openai", "openrouter/auto", 32, 0, nil)
	if err != nil {
		t.Fatalf("NewProvider compat failed: %v", err)
	}
	if _, ok := compatProvider.(*OpenAIProvider); !ok {
		t.Fatalf("expected OpenAIProvider compatibility implementation, got %T", compatProvider)
	}
}
