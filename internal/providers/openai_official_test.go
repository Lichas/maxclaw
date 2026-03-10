package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenAIOfficialProviderUsesSDKEndpointAndNormalizesModel(t *testing.T) {
	var (
		requestPath string
		authHeader  string
		model       string
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		authHeader = r.Header.Get("Authorization")

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		model, _ = body["model"].(string)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl_123","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`))
	}))
	defer server.Close()

	provider, err := newOpenAIOfficialProvider("sk-openai", server.URL+"/v1", "openai/gpt-5.1", 64, 0.2, nil, server.Client())
	if err != nil {
		t.Fatalf("newOpenAIOfficialProvider failed: %v", err)
	}

	resp, err := provider.Chat(context.Background(), []Message{{Role: "user", Content: "ping"}}, nil, "")
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}
	if resp.Content != "ok" {
		t.Fatalf("unexpected response content: %q", resp.Content)
	}
	if requestPath != "/v1/chat/completions" {
		t.Fatalf("unexpected request path: %q", requestPath)
	}
	if authHeader != "Bearer sk-openai" {
		t.Fatalf("unexpected auth header: %q", authHeader)
	}
	if model != "gpt-5.1" {
		t.Fatalf("expected normalized model, got %q", model)
	}
}
