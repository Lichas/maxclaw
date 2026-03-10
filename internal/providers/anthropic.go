package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// AnthropicProvider uses the official Anthropic SDK for native Claude models.
type AnthropicProvider struct {
	client             anthropic.Client
	apiBase            string
	defaultModel       string
	maxTokens          int
	temperature        float64
	supportsImageInput func(model string) bool
}

// NewAnthropicProvider creates an Anthropic provider backed by anthropic-sdk-go.
func NewAnthropicProvider(apiKey, apiBase, defaultModel string, maxTokens int, temperature float64, supportsImageInput func(model string) bool) (*AnthropicProvider, error) {
	return newAnthropicProvider(apiKey, apiBase, defaultModel, maxTokens, temperature, supportsImageInput, nil)
}

func newAnthropicProvider(apiKey, apiBase, defaultModel string, maxTokens int, temperature float64, supportsImageInput func(model string) bool, httpClient *http.Client) (*AnthropicProvider, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf("API key is required")
	}
	if strings.TrimSpace(defaultModel) == "" {
		defaultModel = "claude-sonnet-4-5"
	}
	if maxTokens <= 0 {
		maxTokens = 1
	}

	apiBase = normalizeAnthropicBaseURL(apiBase)
	opts := []option.RequestOption{
		option.WithAPIKey(apiKey),
		option.WithBaseURL(apiBase),
	}
	if httpClient != nil {
		opts = append(opts, option.WithHTTPClient(httpClient))
	} else {
		opts = append(opts, option.WithHTTPClient(&http.Client{Timeout: 60 * time.Second}))
	}

	return &AnthropicProvider{
		client:             anthropic.NewClient(opts...),
		apiBase:            apiBase,
		defaultModel:       defaultModel,
		maxTokens:          maxTokens,
		temperature:        temperature,
		supportsImageInput: supportsImageInput,
	}, nil
}

func (p *AnthropicProvider) Chat(ctx context.Context, messages []Message, tools []map[string]interface{}, model string) (*Response, error) {
	params := p.buildMessageParams(messages, tools, model)

	resp, err := p.client.Messages.New(ctx, params)
	if err != nil {
		return nil, p.wrapModelRequestError("chat request failed", params.Model, err)
	}

	result := &Response{}
	for _, block := range resp.Content {
		switch block.Type {
		case "text":
			result.Content += block.AsText().Text
		case "tool_use":
			toolUse := block.AsToolUse()
			arguments := ""
			if data, err := json.Marshal(toolUse.Input); err == nil {
				arguments = string(data)
			}
			result.ToolCalls = append(result.ToolCalls, ToolCall{
				ID:   toolUse.ID,
				Type: "function",
				Function: ToolCallFunction{
					Name:      toolUse.Name,
					Arguments: arguments,
				},
			})
		}
	}
	result.HasToolCalls = len(result.ToolCalls) > 0
	return result, nil
}

func (p *AnthropicProvider) ChatStream(ctx context.Context, messages []Message, tools []map[string]interface{}, model string, handler StreamHandler) error {
	params := p.buildMessageParams(messages, tools, model)

	stream := p.client.Messages.NewStreaming(ctx, params)
	defer stream.Close()

	buildersByIndex := make(map[int64]*toolCallBuilder)
	for stream.Next() {
		event := stream.Current()
		switch current := event.AsAny().(type) {
		case anthropic.ContentBlockStartEvent:
			if current.ContentBlock.Type != "tool_use" {
				continue
			}
			toolUse := current.ContentBlock.AsToolUse()
			builder := &toolCallBuilder{
				ID:      toolUse.ID,
				Name:    toolUse.Name,
				Started: true,
			}
			if data, err := json.Marshal(toolUse.Input); err == nil && string(data) != "null" && string(data) != "{}" {
				builder.Arguments = string(data)
			}
			buildersByIndex[current.Index] = builder
			handler.OnToolCallStart(builder.ID, builder.Name)
			if builder.Arguments != "" {
				handler.OnToolCallDelta(builder.ID, builder.Arguments)
			}
		case anthropic.ContentBlockDeltaEvent:
			switch current.Delta.Type {
			case "text_delta":
				if current.Delta.Text != "" {
					handler.OnContent(current.Delta.Text)
				}
			case "input_json_delta":
				builder, ok := buildersByIndex[current.Index]
				if !ok || builder == nil || !builder.Started || current.Delta.PartialJSON == "" {
					continue
				}
				builder.Arguments += current.Delta.PartialJSON
				handler.OnToolCallDelta(builder.ID, current.Delta.PartialJSON)
			}
		case anthropic.ContentBlockStopEvent:
			builder, ok := buildersByIndex[current.Index]
			if !ok || builder == nil || !builder.Started || builder.ID == "" {
				continue
			}
			handler.OnToolCallEnd(builder.ID)
		}
	}

	if err := stream.Err(); err != nil {
		wrappedErr := p.wrapModelRequestError("stream request failed", params.Model, err)
		handler.OnError(wrappedErr)
		return wrappedErr
	}

	handler.OnComplete()
	return nil
}

func (p *AnthropicProvider) GetDefaultModel() string {
	return p.defaultModel
}

func (p *AnthropicProvider) SupportsImageInput(model string) bool {
	if strings.TrimSpace(model) == "" {
		model = p.defaultModel
	}
	if p.supportsImageInput != nil {
		return p.supportsImageInput(model)
	}
	return SupportsImageInput("anthropic", model)
}

func (p *AnthropicProvider) buildMessageParams(messages []Message, tools []map[string]interface{}, model string) anthropic.MessageNewParams {
	if strings.TrimSpace(model) == "" {
		model = p.defaultModel
	}

	system, anthropicMessages := convertToAnthropicMessages(messages, p.SupportsImageInput(model))
	params := anthropic.MessageNewParams{
		MaxTokens:   int64(p.maxTokens),
		Messages:    anthropicMessages,
		Model:       anthropic.Model(normalizeModelForProvider("anthropic", model)),
		Temperature: anthropic.Float(p.temperature),
		System:      system,
	}
	if len(tools) > 0 {
		params.Tools = convertToAnthropicTools(tools)
		params.ToolChoice = anthropic.ToolChoiceUnionParam{OfAuto: &anthropic.ToolChoiceAutoParam{}}
	}
	return params
}

func convertToAnthropicMessages(messages []Message, allowImageInput bool) ([]anthropic.TextBlockParam, []anthropic.MessageParam) {
	system := make([]anthropic.TextBlockParam, 0)
	result := make([]anthropic.MessageParam, 0, len(messages))

	for _, msg := range messages {
		switch msg.Role {
		case "system":
			text := strings.TrimSpace(flattenContentParts(msg))
			if text != "" {
				system = append(system, anthropic.TextBlockParam{Text: text})
			}
		case "assistant":
			blocks := anthropicBlocksForAssistant(msg)
			if len(blocks) == 0 {
				blocks = append(blocks, anthropic.NewTextBlock(""))
			}
			result = append(result, anthropic.NewAssistantMessage(blocks...))
		case "tool":
			result = append(result, anthropic.NewUserMessage(
				anthropic.NewToolResultBlock(msg.ToolCallID, flattenContentParts(msg), false),
			))
		default:
			blocks := anthropicBlocksForUser(msg, allowImageInput)
			if len(blocks) == 0 {
				blocks = append(blocks, anthropic.NewTextBlock(""))
			}
			result = append(result, anthropic.NewUserMessage(blocks...))
		}
	}

	return system, result
}

func anthropicBlocksForUser(msg Message, allowImageInput bool) []anthropic.ContentBlockParamUnion {
	if !allowImageInput || len(msg.Parts) == 0 {
		return []anthropic.ContentBlockParamUnion{anthropic.NewTextBlock(flattenContentParts(msg))}
	}

	blocks := make([]anthropic.ContentBlockParamUnion, 0, len(msg.Parts))
	for _, part := range msg.Parts {
		switch part.Type {
		case "image_url":
			block, ok := anthropicImageBlock(part)
			if ok {
				blocks = append(blocks, block)
			}
		default:
			blocks = append(blocks, anthropic.NewTextBlock(part.Text))
		}
	}
	if len(blocks) == 0 {
		blocks = append(blocks, anthropic.NewTextBlock(flattenContentParts(msg)))
	}
	return blocks
}

func anthropicBlocksForAssistant(msg Message) []anthropic.ContentBlockParamUnion {
	blocks := make([]anthropic.ContentBlockParamUnion, 0, len(msg.ToolCalls)+1)
	if text := strings.TrimSpace(flattenContentParts(msg)); text != "" {
		blocks = append(blocks, anthropic.NewTextBlock(text))
	}
	for _, toolCall := range msg.ToolCalls {
		blocks = append(blocks, anthropic.NewToolUseBlock(toolCall.ID, decodeToolArguments(toolCall.Function.Arguments), toolCall.Function.Name))
	}
	return blocks
}

func anthropicImageBlock(part ContentPart) (anthropic.ContentBlockParamUnion, bool) {
	imageURL := buildProviderImageURL(part)
	if strings.TrimSpace(imageURL) == "" {
		return anthropic.ContentBlockParamUnion{}, false
	}

	if mediaType, data, ok := parseImageDataURL(imageURL); ok {
		return anthropic.NewImageBlockBase64(mediaType, data), true
	}

	return anthropic.NewImageBlock(anthropic.URLImageSourceParam{URL: imageURL}), true
}

func convertToAnthropicTools(tools []map[string]interface{}) []anthropic.ToolUnionParam {
	result := make([]anthropic.ToolUnionParam, 0, len(tools))
	for _, tool := range tools {
		function, _ := tool["function"].(map[string]interface{})
		name, _ := function["name"].(string)
		if strings.TrimSpace(name) == "" {
			continue
		}

		description, _ := function["description"].(string)
		parameters, _ := function["parameters"].(map[string]interface{})
		schema := anthropic.ToolInputSchemaParam{}
		if properties, ok := parameters["properties"]; ok {
			schema.Properties = properties
		}
		if required, ok := parameters["required"].([]string); ok {
			schema.Required = required
		} else if requiredAny, ok := parameters["required"].([]interface{}); ok {
			required := make([]string, 0, len(requiredAny))
			for _, item := range requiredAny {
				if value, ok := item.(string); ok {
					required = append(required, value)
				}
			}
			schema.Required = required
		}
		if len(parameters) > 0 {
			schema.ExtraFields = make(map[string]any)
			for key, value := range parameters {
				if key == "type" || key == "properties" || key == "required" {
					continue
				}
				schema.ExtraFields[key] = value
			}
			if len(schema.ExtraFields) == 0 {
				schema.ExtraFields = nil
			}
		}

		result = append(result, anthropic.ToolUnionParam{
			OfTool: &anthropic.ToolParam{
				Name:        name,
				Description: anthropic.String(description),
				InputSchema: schema,
			},
		})
	}
	return result
}

func decodeToolArguments(arguments string) any {
	arguments = strings.TrimSpace(arguments)
	if arguments == "" {
		return map[string]any{}
	}

	var decoded any
	if err := json.Unmarshal([]byte(arguments), &decoded); err != nil {
		return map[string]any{"raw": arguments}
	}
	return decoded
}

func parseImageDataURL(value string) (mediaType string, data string, ok bool) {
	if !strings.HasPrefix(value, "data:") {
		return "", "", false
	}
	parts := strings.SplitN(value, ",", 2)
	if len(parts) != 2 {
		return "", "", false
	}

	meta := strings.TrimPrefix(parts[0], "data:")
	metaParts := strings.Split(meta, ";")
	if len(metaParts) < 2 || metaParts[len(metaParts)-1] != "base64" {
		return "", "", false
	}

	return metaParts[0], parts[1], true
}

func (p *AnthropicProvider) wrapModelRequestError(prefix string, model anthropic.Model, err error) error {
	return fmt.Errorf("%s provider=anthropic model=%s api_base=%s: %w", prefix, model, p.apiBase, err)
}
