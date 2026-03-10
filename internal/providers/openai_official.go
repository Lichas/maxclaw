package providers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/shared"
)

// OpenAIOfficialProvider uses the official OpenAI SDK for native OpenAI models.
type OpenAIOfficialProvider struct {
	client             openai.Client
	apiBase            string
	defaultModel       string
	maxTokens          int
	temperature        float64
	supportsImageInput func(model string) bool
}

// NewOpenAIOfficialProvider creates an OpenAI provider backed by openai-go.
func NewOpenAIOfficialProvider(apiKey, apiBase, defaultModel string, maxTokens int, temperature float64, supportsImageInput func(model string) bool) (*OpenAIOfficialProvider, error) {
	return newOpenAIOfficialProvider(apiKey, apiBase, defaultModel, maxTokens, temperature, supportsImageInput, nil)
}

func newOpenAIOfficialProvider(apiKey, apiBase, defaultModel string, maxTokens int, temperature float64, supportsImageInput func(model string) bool, httpClient *http.Client) (*OpenAIOfficialProvider, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf("API key is required")
	}
	if strings.TrimSpace(apiBase) == "" {
		apiBase = "https://api.openai.com/v1"
	}
	if strings.TrimSpace(defaultModel) == "" {
		defaultModel = "gpt-4.1"
	}
	if maxTokens <= 0 {
		maxTokens = 1
	}

	opts := []option.RequestOption{
		option.WithAPIKey(apiKey),
		option.WithBaseURL(strings.TrimRight(apiBase, "/")),
	}
	if httpClient != nil {
		opts = append(opts, option.WithHTTPClient(httpClient))
	} else {
		opts = append(opts, option.WithHTTPClient(&http.Client{Timeout: 60 * time.Second}))
	}

	return &OpenAIOfficialProvider{
		client:             openai.NewClient(opts...),
		apiBase:            strings.TrimRight(apiBase, "/"),
		defaultModel:       defaultModel,
		maxTokens:          maxTokens,
		temperature:        temperature,
		supportsImageInput: supportsImageInput,
	}, nil
}

func (p *OpenAIOfficialProvider) Chat(ctx context.Context, messages []Message, tools []map[string]interface{}, model string) (*Response, error) {
	params := p.buildChatParams(messages, tools, model)

	resp, err := p.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return nil, p.wrapModelRequestError("chat request failed", params.Model, err)
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from model")
	}

	choice := resp.Choices[0]
	result := &Response{
		Content: choice.Message.Content,
	}
	for _, toolCall := range choice.Message.ToolCalls {
		if toolCall.Type != "function" {
			continue
		}
		call := toolCall.AsFunction()
		result.ToolCalls = append(result.ToolCalls, ToolCall{
			ID:   call.ID,
			Type: "function",
			Function: ToolCallFunction{
				Name:      call.Function.Name,
				Arguments: call.Function.Arguments,
			},
		})
	}
	result.HasToolCalls = len(result.ToolCalls) > 0
	return result, nil
}

func (p *OpenAIOfficialProvider) ChatStream(ctx context.Context, messages []Message, tools []map[string]interface{}, model string, handler StreamHandler) error {
	params := p.buildChatParams(messages, tools, model)

	stream := p.client.Chat.Completions.NewStreaming(ctx, params)
	defer stream.Close()

	buildersByIndex := make(map[int64]*toolCallBuilder)
	for stream.Next() {
		chunk := stream.Current()
		if len(chunk.Choices) == 0 {
			continue
		}

		delta := chunk.Choices[0].Delta
		if delta.Content != "" {
			handler.OnContent(delta.Content)
		}

		for _, tc := range delta.ToolCalls {
			builder, ok := buildersByIndex[tc.Index]
			if !ok {
				builder = &toolCallBuilder{}
				buildersByIndex[tc.Index] = builder
			}

			if tc.ID != "" {
				builder.ID = tc.ID
			}
			if tc.Function.Name != "" {
				builder.Name = tc.Function.Name
			}
			if tc.Function.Arguments != "" {
				builder.Arguments += tc.Function.Arguments
			}

			if !builder.Started && builder.ID != "" && builder.Name != "" {
				builder.Started = true
				handler.OnToolCallStart(builder.ID, builder.Name)
				if builder.Arguments != "" {
					handler.OnToolCallDelta(builder.ID, builder.Arguments)
				}
				continue
			}

			if builder.Started && tc.Function.Arguments != "" {
				handler.OnToolCallDelta(builder.ID, tc.Function.Arguments)
			}
		}
	}

	if err := stream.Err(); err != nil {
		wrappedErr := p.wrapModelRequestError("stream request failed", params.Model, err)
		handler.OnError(wrappedErr)
		return wrappedErr
	}

	for _, builder := range buildersByIndex {
		if builder != nil && builder.Started && builder.ID != "" {
			handler.OnToolCallEnd(builder.ID)
		}
	}

	handler.OnComplete()
	return nil
}

func (p *OpenAIOfficialProvider) GetDefaultModel() string {
	return p.defaultModel
}

func (p *OpenAIOfficialProvider) SupportsImageInput(model string) bool {
	if strings.TrimSpace(model) == "" {
		model = p.defaultModel
	}
	if p.supportsImageInput != nil {
		return p.supportsImageInput(model)
	}
	return SupportsImageInput("openai", model)
}

func (p *OpenAIOfficialProvider) buildChatParams(messages []Message, tools []map[string]interface{}, model string) openai.ChatCompletionNewParams {
	if strings.TrimSpace(model) == "" {
		model = p.defaultModel
	}
	normalizedModel := normalizeModelForProvider("openai", model)

	params := openai.ChatCompletionNewParams{
		Messages:    convertToOfficialOpenAIMessages(messages, p.SupportsImageInput(model)),
		Model:       shared.ChatModel(normalizedModel),
		MaxTokens:   openai.Int(int64(p.maxTokens)),
		Temperature: openai.Float(p.temperature),
	}
	if len(tools) > 0 {
		params.Tools = convertToOfficialOpenAITools(tools)
	}
	return params
}

func convertToOfficialOpenAIMessages(messages []Message, allowImageInput bool) []openai.ChatCompletionMessageParamUnion {
	result := make([]openai.ChatCompletionMessageParamUnion, 0, len(messages))
	for _, msg := range messages {
		switch msg.Role {
		case "system":
			result = append(result, openai.SystemMessage(flattenContentParts(msg)))
		case "assistant":
			assistant := openai.ChatCompletionAssistantMessageParam{}
			content := strings.TrimSpace(flattenContentParts(msg))
			if content != "" || len(msg.ToolCalls) == 0 {
				assistant.Content.OfString = openai.String(content)
			}
			if len(msg.ToolCalls) > 0 {
				assistant.ToolCalls = convertAssistantToolCalls(msg.ToolCalls)
			}
			result = append(result, openai.ChatCompletionMessageParamUnion{OfAssistant: &assistant})
		case "tool":
			result = append(result, openai.ToolMessage(flattenContentParts(msg), msg.ToolCallID))
		default:
			if allowImageInput && len(msg.Parts) > 0 {
				result = append(result, openai.UserMessage(convertToOfficialOpenAIContentParts(msg.Parts)))
			} else {
				result = append(result, openai.UserMessage(flattenContentParts(msg)))
			}
		}
	}
	return result
}

func convertToOfficialOpenAIContentParts(parts []ContentPart) []openai.ChatCompletionContentPartUnionParam {
	result := make([]openai.ChatCompletionContentPartUnionParam, 0, len(parts))
	for _, part := range parts {
		switch part.Type {
		case "image_url":
			imageURL := buildProviderImageURL(part)
			if strings.TrimSpace(imageURL) == "" {
				continue
			}
			result = append(result, openai.ImageContentPart(openai.ChatCompletionContentPartImageImageURLParam{
				URL: imageURL,
			}))
		default:
			result = append(result, openai.TextContentPart(part.Text))
		}
	}
	return result
}

func convertAssistantToolCalls(toolCalls []ToolCall) []openai.ChatCompletionMessageToolCallUnionParam {
	result := make([]openai.ChatCompletionMessageToolCallUnionParam, 0, len(toolCalls))
	for _, tc := range toolCalls {
		result = append(result, openai.ChatCompletionMessageToolCallUnionParam{
			OfFunction: &openai.ChatCompletionMessageFunctionToolCallParam{
				ID: tc.ID,
				Function: openai.ChatCompletionMessageFunctionToolCallFunctionParam{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			},
		})
	}
	return result
}

func convertToOfficialOpenAITools(tools []map[string]interface{}) []openai.ChatCompletionToolUnionParam {
	result := make([]openai.ChatCompletionToolUnionParam, 0, len(tools))
	for _, tool := range tools {
		function, _ := tool["function"].(map[string]interface{})
		name, _ := function["name"].(string)
		if strings.TrimSpace(name) == "" {
			continue
		}

		description, _ := function["description"].(string)
		parameters, _ := function["parameters"].(map[string]interface{})
		result = append(result, openai.ChatCompletionFunctionTool(shared.FunctionDefinitionParam{
			Name:        name,
			Description: openai.String(description),
			Parameters:  openai.FunctionParameters(parameters),
		}))
	}
	return result
}

func (p *OpenAIOfficialProvider) wrapModelRequestError(prefix string, model shared.ChatModel, err error) error {
	return fmt.Errorf("%s provider=openai model=%s api_base=%s: %w", prefix, model, p.apiBase, err)
}
