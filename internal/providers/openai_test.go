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

func TestConvertToChatMessagesSupportsImageParts(t *testing.T) {
	converted := convertToChatMessages([]Message{
		{
			Role:    "user",
			Content: "User sent an image.",
			Parts: []ContentPart{
				{Type: "text", Text: "User sent an image."},
				{Type: "image_url", ImageURL: "https://example.com/test.png"},
			},
		},
	})

	body, err := json.Marshal(converted)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded []map[string]interface{}
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	content, ok := decoded[0]["content"].([]interface{})
	if !ok {
		t.Fatalf("expected content array, got %T", decoded[0]["content"])
	}
	if len(content) != 2 {
		t.Fatalf("expected 2 content parts, got %d", len(content))
	}

	imagePart, ok := content[1].(map[string]interface{})
	if !ok {
		t.Fatalf("expected image part object, got %T", content[1])
	}
	if imagePart["type"] != "image_url" {
		t.Fatalf("expected image_url part, got %v", imagePart["type"])
	}
	imageURL, ok := imagePart["image_url"].(map[string]interface{})
	if !ok || imageURL["url"] != "https://example.com/test.png" {
		t.Fatalf("unexpected image_url payload: %#v", imagePart["image_url"])
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
