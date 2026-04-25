package config

import "testing"

func TestGetAPIBaseFallsBackToFirstConfiguredProviderForRawModelID(t *testing.T) {
	// OpenRouter models like "tencent/hy3-preview:free" don't match any
	// ProviderSpec keyword, but should route to OpenRouter when it is the
	// first configured provider (matching GetAPIKey's fallback behavior).
	cfg := &Config{
		Agents: AgentsConfig{
			Defaults: AgentDefaults{Model: "tencent/hy3-preview:free"},
		},
		Providers: ProvidersConfig{
			OpenRouter: ProviderConfig{
				APIKey:  "sk-or-v1-test",
				APIBase: "https://openrouter.ai/api/v1",
			},
		},
	}

	base := cfg.GetAPIBase("tencent/hy3-preview:free")
	if base != "https://openrouter.ai/api/v1" {
		t.Fatalf("expected OpenRouter apiBase for raw model ID, got %q", base)
	}
}

func TestGetAPIBaseVLLMTakesPriorityOverFallback(t *testing.T) {
	// When both vLLM and OpenRouter are configured, vLLM should still win
	// for raw model IDs because it is explicitly checked first.
	cfg := &Config{
		Agents: AgentsConfig{
			Defaults: AgentDefaults{Model: "meta-llama/Llama-3.1-8B-Instruct"},
		},
		Providers: ProvidersConfig{
			OpenRouter: ProviderConfig{
				APIKey:  "sk-or-v1-test",
				APIBase: "https://openrouter.ai/api/v1",
			},
			VLLM: ProviderConfig{
				APIKey:  "sk-vllm-test",
				APIBase: "http://localhost:8000/v1",
			},
		},
	}

	base := cfg.GetAPIBase("meta-llama/Llama-3.1-8B-Instruct")
	if base != "http://localhost:8000/v1" {
		t.Fatalf("expected vLLM apiBase to take priority, got %q", base)
	}
}

func TestGetAPIBaseRoutesViaKnownProviderModelRegistry(t *testing.T) {
	// When a known provider (e.g. openrouter) registers a model in its
	// models list, the system should route to that provider directly
	// instead of relying on model name keyword guessing.
	cfg := &Config{
		Agents: AgentsConfig{
			Defaults: AgentDefaults{Model: "tencent/hy3-preview:free"},
		},
		Providers: ProvidersConfig{
			OpenRouter: ProviderConfig{
				APIKey:  "sk-or-v1-test",
				APIBase: "https://openrouter.ai/api/v1",
				Models: []ProviderModelConfig{
					{ID: "tencent/hy3-preview:free", Enabled: true},
				},
			},
		},
	}

	// apiKey
	key := cfg.GetAPIKey("tencent/hy3-preview:free")
	if key != "sk-or-v1-test" {
		t.Fatalf("expected OpenRouter apiKey, got %q", key)
	}

	// apiBase
	base := cfg.GetAPIBase("tencent/hy3-preview:free")
	if base != "https://openrouter.ai/api/v1" {
		t.Fatalf("expected OpenRouter apiBase, got %q", base)
	}

	// apiFormat (defaults to openai when not explicitly set)
	format := cfg.GetAPIFormat("tencent/hy3-preview:free")
	if format != "openai" {
		t.Fatalf("expected openai format, got %q", format)
	}
}

func TestGetAPIKeyRoutesViaKnownProviderModelRegistry(t *testing.T) {
	// DeepSeek provider is a known provider. If user explicitly registers
	// a model in its models list, the system should still find it.
	cfg := &Config{
		Agents: AgentsConfig{
			Defaults: AgentDefaults{Model: "my-custom-deepseek-model"},
		},
		Providers: ProvidersConfig{
			DeepSeek: ProviderConfig{
				APIKey:  "sk-deepseek-test",
				APIBase: "https://api.deepseek.com/v1",
				Models: []ProviderModelConfig{
					{ID: "my-custom-deepseek-model", Enabled: true},
				},
			},
		},
	}

	key := cfg.GetAPIKey("my-custom-deepseek-model")
	if key != "sk-deepseek-test" {
		t.Fatalf("expected DeepSeek apiKey, got %q", key)
	}
}

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
