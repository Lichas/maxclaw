package providers

import "testing"

func TestProviderSpecMatchesModel(t *testing.T) {
	spec := ProviderSpec{Name: "moonshot", Keywords: []string{"moonshot", "kimi"}}
	if !spec.MatchesModel("Moonshot/Kimi-K2.5") {
		t.Fatalf("expected moonshot spec to match model")
	}
	if spec.MatchesModel("openrouter/gpt-4o") {
		t.Fatalf("did not expect moonshot spec to match openrouter model")
	}
}

func TestProviderSpecsContainsMiniMax(t *testing.T) {
	found := false
	for _, spec := range ProviderSpecs {
		if spec.Name == "minimax" {
			found = true
			if spec.DefaultAPIBase == "" {
				t.Fatalf("expected minimax default api base")
			}
			break
		}
	}
	if !found {
		t.Fatalf("expected minimax spec in ProviderSpecs")
	}
}

func TestProviderSpecsContainsDashScopeForQwen(t *testing.T) {
	found := false
	for _, spec := range ProviderSpecs {
		if spec.Name == "dashscope" {
			found = true
			if !spec.MatchesModel("qwen-max") {
				t.Fatalf("expected dashscope spec to match qwen model")
			}
			break
		}
	}
	if !found {
		t.Fatalf("expected dashscope spec in ProviderSpecs")
	}
}
