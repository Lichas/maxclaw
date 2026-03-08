package config

import "testing"

func TestSupportsImageInputUsesExplicitModelCapability(t *testing.T) {
	enabled := true
	cfg := &Config{
		Agents: AgentsConfig{
			Defaults: AgentDefaults{Model: "glm-5"},
		},
		Providers: ProvidersConfig{
			Zhipu: ProviderConfig{
				Models: []ProviderModelConfig{
					{ID: "glm-5", Enabled: true, SupportsImageInput: &enabled},
				},
			},
		},
	}

	if !cfg.SupportsImageInput("glm-5") {
		t.Fatal("expected explicit model capability to enable image input")
	}
	if !cfg.SupportsImageInput("zhipu/glm-5") {
		t.Fatal("expected provider-prefixed model id to match configured capability")
	}
}

func TestSupportsImageInputFallsBackToProviderHeuristic(t *testing.T) {
	cfg := &Config{}

	if cfg.SupportsImageInput("deepseek-chat") {
		t.Fatal("expected deepseek-chat to keep image input disabled by fallback heuristic")
	}
	if !cfg.SupportsImageInput("deepseek-vl") {
		t.Fatal("expected deepseek-vl to allow image input by fallback heuristic")
	}
}
