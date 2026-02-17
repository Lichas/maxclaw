package providers

import (
	"encoding/json"
	"testing"
)

func TestConvertToChatMessagesAlwaysIncludesContentField(t *testing.T) {
	messages := []Message{
		{
			Role:    "assistant",
			Content: "",
			ToolCalls: []ToolCall{
				{
					ID:   "call_1",
					Type: "function",
					Function: ToolCallFunction{
						Name:      "cron",
						Arguments: `{"action":"list"}`,
					},
				},
			},
		},
		{
			Role:       "tool",
			Content:    "",
			ToolCallID: "call_1",
		},
	}

	converted := convertToChatMessages(messages)
	body, err := json.Marshal(converted)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded []map[string]interface{}
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if len(decoded) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(decoded))
	}

	for i, msg := range decoded {
		if _, ok := msg["content"]; !ok {
			t.Fatalf("message %d missing content field: %s", i, string(body))
		}
	}
}

func TestBuildChatRequestIncludesGenerationParamsAndClampsMaxTokens(t *testing.T) {
	req := buildChatRequest(
		[]Message{{Role: "user", Content: "hello"}},
		nil,
		"gpt-4o-mini",
		false,
		0,
		0.2,
	)

	if req.MaxTokens != 1 {
		t.Fatalf("expected max_tokens to be clamped to 1, got %d", req.MaxTokens)
	}
	if req.Temperature != 0.2 {
		t.Fatalf("expected temperature=0.2, got %v", req.Temperature)
	}

	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if got, ok := decoded["max_tokens"]; !ok || got.(float64) != 1 {
		t.Fatalf("expected max_tokens in payload, got %v", decoded["max_tokens"])
	}
	if got, ok := decoded["temperature"]; !ok || got.(float64) != 0.2 {
		t.Fatalf("expected temperature in payload, got %v", decoded["temperature"])
	}
}
