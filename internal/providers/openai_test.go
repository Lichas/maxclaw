package providers

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

	converted := convertToChatMessages(messages, true)
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
				{Type: "image_url", ImageURL: "https://example.com/test.png", ImagePath: filepath.Join(t.TempDir(), "missing.png")},
			},
		},
	}, true)

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

func TestConvertToChatMessagesInlinesLocalImageAsDataURL(t *testing.T) {
	tmpDir := t.TempDir()
	imagePath := filepath.Join(tmpDir, "image.png")
	if err := os.WriteFile(imagePath, []byte("fake image bytes"), 0644); err != nil {
		t.Fatalf("write image failed: %v", err)
	}

	converted := convertToChatMessages([]Message{
		{
			Role:    "user",
			Content: "User sent an image.",
			Parts: []ContentPart{
				{Type: "text", Text: "User sent an image."},
				{Type: "image_url", ImagePath: imagePath, MimeType: "image/png"},
			},
		},
	}, true)

	body, err := json.Marshal(converted)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded []map[string]interface{}
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	content := decoded[0]["content"].([]interface{})
	imagePart := content[1].(map[string]interface{})
	imageURL := imagePart["image_url"].(map[string]interface{})
	value, _ := imageURL["url"].(string)
	if !strings.HasPrefix(value, "data:image/png;base64,") {
		t.Fatalf("expected inline data URL, got %q", value)
	}
}

func TestBuildChatRequestIncludesGenerationParamsAndClampsMaxTokens(t *testing.T) {
	req := buildChatRequest(
		[]Message{{Role: "user", Content: "hello"}},
		nil,
		"gpt-4o-mini",
		true,
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

func TestConvertToChatMessagesFlattensImagePartsWhenModelDoesNotSupportThem(t *testing.T) {
	converted := convertToChatMessages([]Message{
		{
			Role:    "user",
			Content: "User sent an image.",
			Parts: []ContentPart{
				{Type: "text", Text: "User sent an image."},
				{Type: "image_url", ImageURL: "https://example.com/test.png"},
			},
		},
	}, false)

	body, err := json.Marshal(converted)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded []map[string]interface{}
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	content, ok := decoded[0]["content"].(string)
	if !ok {
		t.Fatalf("expected flattened string content, got %T", decoded[0]["content"])
	}
	if content != "User sent an image.\nUser also attached an image, but the current model cannot inspect images directly." {
		t.Fatalf("unexpected flattened content: %q", content)
	}
}

func TestSupportsImageInputDisablesDeepSeekChat(t *testing.T) {
	if SupportsImageInput("deepseek", "deepseek-chat") {
		t.Fatal("expected deepseek-chat to disable image content parts")
	}
	if !SupportsImageInput("deepseek", "deepseek-vl") {
		t.Fatal("expected deepseek-vl to allow image content parts")
	}
	if !SupportsImageInput("zhipu", "glm-5") {
		t.Fatal("expected glm-5 to allow image content parts")
	}
}

func TestOpenAIProviderSupportsImageInputUsesResolver(t *testing.T) {
	provider, err := NewOpenAIProvider("sk-test", "https://example.com/v1", "glm-5", 1024, 0.1, func(model string) bool {
		return strings.EqualFold(model, "glm-5")
	})
	if err != nil {
		t.Fatalf("NewOpenAIProvider failed: %v", err)
	}

	if !provider.SupportsImageInput("glm-5") {
		t.Fatal("expected resolver to enable image input for glm-5")
	}
	if provider.SupportsImageInput("deepseek-chat") {
		t.Fatal("expected resolver to keep unrelated models disabled")
	}
}
