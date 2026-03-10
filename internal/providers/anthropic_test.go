package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAnthropicProviderUsesSDKEndpointAndNormalizesModel(t *testing.T) {
	var (
		requestPath string
		apiKey      string
		model       string
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		apiKey = r.Header.Get("X-Api-Key")

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		model, _ = body["model"].(string)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"msg_123","type":"message","role":"assistant","model":"claude-sonnet-4-5","content":[{"type":"text","text":"ok"}],"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":1,"output_tokens":1}}`))
	}))
	defer server.Close()

	provider, err := newAnthropicProvider("sk-anthropic", server.URL+"/v1", "anthropic/claude-sonnet-4-5", 64, 0.2, nil, server.Client())
	if err != nil {
		t.Fatalf("newAnthropicProvider failed: %v", err)
	}

	resp, err := provider.Chat(context.Background(), []Message{{Role: "user", Content: "ping"}}, nil, "")
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}
	if resp.Content != "ok" {
		t.Fatalf("unexpected response content: %q", resp.Content)
	}
	if requestPath != "/v1/messages" {
		t.Fatalf("unexpected request path: %q", requestPath)
	}
	if apiKey != "sk-anthropic" {
		t.Fatalf("unexpected api key header: %q", apiKey)
	}
	if model != "claude-sonnet-4-5" {
		t.Fatalf("expected normalized model, got %q", model)
	}
}
