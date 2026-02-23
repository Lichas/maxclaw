package providers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// debug flag - set to false in production
const debug = false

// OpenAIProvider OpenAI 提供商实现
// 使用 OpenAI 兼容 API (string content) 以支持 DeepSeek 等提供商
type OpenAIProvider struct {
	apiKey       string
	apiBase      string
	defaultModel string
	maxTokens    int
	temperature  float64
	httpClient   *http.Client
	streamClient *http.Client
}

// NewOpenAIProvider 创建 OpenAI 提供商
func NewOpenAIProvider(apiKey, apiBase, defaultModel string, maxTokens int, temperature float64) (*OpenAIProvider, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	if apiBase == "" {
		apiBase = "https://api.openai.com/v1"
	}

	apiBase = strings.TrimRight(apiBase, "/")

	if defaultModel == "" {
		defaultModel = "gpt-4"
	}
	if maxTokens <= 0 {
		maxTokens = 1
	}

	return &OpenAIProvider{
		apiKey:       apiKey,
		apiBase:      apiBase,
		defaultModel: defaultModel,
		maxTokens:    maxTokens,
		temperature:  temperature,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		streamClient: &http.Client{},
	}, nil
}

// Chat 发送聊天请求
func (p *OpenAIProvider) Chat(ctx context.Context, messages []Message, tools []map[string]interface{}, model string) (*Response, error) {
	if model == "" {
		model = p.defaultModel
	}

	reqBody := buildChatRequest(messages, tools, model, false, p.maxTokens, p.temperature)
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	if debug {
		fmt.Printf("[OpenAIProvider] Request:\n%s\n", string(payload))
	}

	respBody, err := p.doRequest(ctx, payload, false)
	if err != nil {
		return nil, p.wrapModelRequestError("chat request failed", model, err)
	}

	var resp chatResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from model")
	}

	choice := resp.Choices[0]

	result := &Response{
		Content: choice.Message.Content,
	}

	if len(choice.Message.ToolCalls) > 0 {
		result.HasToolCalls = true
		result.ToolCalls = make([]ToolCall, len(choice.Message.ToolCalls))
		for i, tc := range choice.Message.ToolCalls {
			result.ToolCalls[i] = ToolCall{
				ID:   tc.ID,
				Type: tc.Type,
				Function: ToolCallFunction{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			}
		}
	}

	return result, nil
}

// GetDefaultModel 获取默认模型
func (p *OpenAIProvider) GetDefaultModel() string {
	return p.defaultModel
}

// ChatStream 流式聊天请求
func (p *OpenAIProvider) ChatStream(ctx context.Context, messages []Message, tools []map[string]interface{}, model string, handler StreamHandler) error {
	if model == "" {
		model = p.defaultModel
	}

	reqBody := buildChatRequest(messages, tools, model, true, p.maxTokens, p.temperature)
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to encode request: %w", err)
	}

	if debug {
		fmt.Printf("[OpenAIProvider] Stream Request:\n%s\n", string(payload))
	}

	stream, err := p.doStreamRequest(ctx, payload)
	if err != nil {
		wrappedErr := p.wrapModelRequestError("stream request failed", model, err)
		handler.OnError(wrappedErr)
		return wrappedErr
	}
	defer stream.Close()

	buildersByIndex := make(map[int]*toolCallBuilder)

	// Use a goroutine to read from stream so we can respond to context cancellation
	lines := make(chan string, 100)
	scanErr := make(chan error, 1)

	go func() {
		scanner := bufio.NewScanner(stream)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			lines <- scanner.Text()
		}
		if err := scanner.Err(); err != nil {
			scanErr <- err
		}
		close(lines)
	}()

	for {
		select {
		case <-ctx.Done():
			// Context cancelled - return gracefully
			return ctx.Err()
		case err := <-scanErr:
			wrappedErr := fmt.Errorf("stream error: %w", err)
			modelErr := p.wrapModelRequestError("stream read failed", model, wrappedErr)
			handler.OnError(modelErr)
			return modelErr
		case line, ok := <-lines:
			if !ok {
				// Stream closed normally
				goto complete
			}

			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if !strings.HasPrefix(line, "data:") {
				continue
			}

			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if data == "[DONE]" {
				goto complete
			}
			if data == "" {
				continue
			}

			var chunk chatStreamChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				handler.OnError(fmt.Errorf("stream decode error: %w", err))
				return err
			}

			if len(chunk.Choices) == 0 {
				continue
			}

			choice := chunk.Choices[0]
			delta := choice.Delta

			if choice.FinishReason != "" {
				fmt.Printf("[DEBUG] FinishReason: %s\n", choice.FinishReason)
			}

			if delta.Content != "" {
				handler.OnContent(delta.Content)
			}

			for _, tc := range delta.ToolCalls {
				idx := tc.Index
				builder, exists := buildersByIndex[idx]
				if !exists {
					builder = &toolCallBuilder{}
					buildersByIndex[idx] = builder
				}

				if tc.ID != "" {
					builder.ID = tc.ID
				}
				if tc.Function.Name != "" {
					builder.Name = tc.Function.Name
				}

				if !builder.Started && builder.ID != "" && builder.Name != "" {
					builder.Started = true
					handler.OnToolCallStart(builder.ID, builder.Name)
				}

				if tc.Function.Arguments != "" {
					builder.Arguments += tc.Function.Arguments
					if builder.ID != "" {
						handler.OnToolCallDelta(builder.ID, tc.Function.Arguments)
					}
				}
			}
		}
	}

complete:

	for _, builder := range buildersByIndex {
		if builder != nil && builder.Arguments != "" && builder.ID != "" {
			handler.OnToolCallEnd(builder.ID)
		}
	}

	handler.OnComplete()
	return nil
}

func (p *OpenAIProvider) wrapModelRequestError(prefix, model string, err error) error {
	return fmt.Errorf("%s provider=%s model=%s api_base=%s: %w", prefix, p.detectProvider(model), model, p.apiBase, err)
}

func (p *OpenAIProvider) detectProvider(model string) string {
	normalizedModel := strings.ToLower(strings.TrimSpace(model))
	if normalizedModel != "" && strings.Contains(normalizedModel, "/") {
		prefix := strings.SplitN(normalizedModel, "/", 2)[0]
		switch prefix {
		case "openrouter", "anthropic", "openai", "deepseek", "zhipu", "groq", "gemini", "dashscope", "moonshot", "minimax", "vllm":
			return prefix
		}
	}

	base := strings.ToLower(p.apiBase)
	switch {
	case strings.Contains(base, "openrouter.ai"):
		return "openrouter"
	case strings.Contains(base, "api.anthropic.com"):
		return "anthropic"
	case strings.Contains(base, "api.openai.com"):
		return "openai"
	case strings.Contains(base, "deepseek.com"):
		return "deepseek"
	case strings.Contains(base, "bigmodel.cn"):
		return "zhipu"
	case strings.Contains(base, "groq.com"):
		return "groq"
	case strings.Contains(base, "generativelanguage.googleapis.com"):
		return "gemini"
	case strings.Contains(base, "dashscope.aliyuncs.com"):
		return "dashscope"
	case strings.Contains(base, "moonshot"):
		return "moonshot"
	case strings.Contains(base, "minimax"):
		return "minimax"
	}

	return "unknown"
}

// toolCallBuilder 工具调用构建器
type toolCallBuilder struct {
	ID        string
	Name      string
	Arguments string
	Started   bool
}

// isCompleteJSON 检查 JSON 是否完整
func isCompleteJSON(s string) bool {
	if s == "" {
		return false
	}
	// 简单检查：括号是否匹配
	var objCount, arrCount int
	inString := false
	escaped := false

	for _, r := range s {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if r == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		switch r {
		case '{':
			objCount++
		case '}':
			objCount--
		case '[':
			arrCount++
		case ']':
			arrCount--
		}
	}

	return objCount == 0 && arrCount == 0 && !inString
}

// buildChatRequest 构造请求体
func buildChatRequest(messages []Message, tools []map[string]interface{}, model string, stream bool, maxTokens int, temperature float64) chatRequest {
	if maxTokens <= 0 {
		maxTokens = 1
	}

	reqBody := chatRequest{
		Model:       model,
		Messages:    convertToChatMessages(messages),
		Stream:      stream,
		MaxTokens:   maxTokens,
		Temperature: temperature,
	}

	if len(tools) > 0 {
		reqBody.Tools = tools
		reqBody.ToolChoice = "auto"
	}

	return reqBody
}

// convertToChatMessages 转换消息格式为 OpenAI 兼容格式 (string content)
func convertToChatMessages(messages []Message) []chatMessage {
	result := make([]chatMessage, len(messages))
	for i, msg := range messages {
		cm := chatMessage{
			Role: msg.Role,
		}

		// 对于所有角色，都设置content字段（即使是空字符串）
		// 有些API实现要求content字段必须存在
		cm.Content = msg.Content

		if msg.Role == "tool" {
			cm.ToolCallID = msg.ToolCallID
		}

		if len(msg.ToolCalls) > 0 {
			cm.ToolCalls = make([]chatToolCall, len(msg.ToolCalls))
			for j, tc := range msg.ToolCalls {
				cm.ToolCalls[j] = chatToolCall{
					ID:   tc.ID,
					Type: tc.Type,
					Function: chatToolCallFunction{
						Name:      tc.Function.Name,
						Arguments: tc.Function.Arguments,
					},
				}
			}
		}

		result[i] = cm
	}
	return result
}

// doRequest 执行非流式请求
func (p *OpenAIProvider) doRequest(ctx context.Context, payload []byte, stream bool) ([]byte, error) {
	endpoint := p.apiBase + "/chat/completions"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	setCommonHeaders(req, p.apiKey)

	if stream {
		req.Header.Set("Accept", "text/event-stream")
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("chat completion failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("chat completion failed: %s", formatAPIError(body, resp.StatusCode))
	}

	return body, nil
}

// doStreamRequest 执行流式请求
func (p *OpenAIProvider) doStreamRequest(ctx context.Context, payload []byte) (io.ReadCloser, error) {
	endpoint := p.apiBase + "/chat/completions"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	setCommonHeaders(req, p.apiKey)
	req.Header.Set("Accept", "text/event-stream")

	resp, err := p.streamClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("stream request failed: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("stream request failed: %s", formatAPIError(body, resp.StatusCode))
	}

	return resp.Body, nil
}

func setCommonHeaders(req *http.Request, apiKey string) {
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
}

func formatAPIError(body []byte, status int) string {
	var apiErr chatErrorResponse
	if err := json.Unmarshal(body, &apiErr); err == nil && apiErr.Error.Message != "" {
		return fmt.Sprintf("status %d: %s", status, apiErr.Error.Message)
	}
	return fmt.Sprintf("status %d: %s", status, strings.TrimSpace(string(body)))
}

// ---- OpenAI-compatible request/response structs ----

type chatRequest struct {
	Model       string                   `json:"model"`
	Messages    []chatMessage            `json:"messages"`
	Tools       []map[string]interface{} `json:"tools,omitempty"`
	ToolChoice  interface{}              `json:"tool_choice,omitempty"`
	Stream      bool                     `json:"stream,omitempty"`
	MaxTokens   int                      `json:"max_tokens"`
	Temperature float64                  `json:"temperature"`
}

type chatMessage struct {
	Role       string         `json:"role"`
	Content    string         `json:"content"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
	ToolCalls  []chatToolCall `json:"tool_calls,omitempty"`
}

type chatToolCall struct {
	ID       string               `json:"id,omitempty"`
	Type     string               `json:"type,omitempty"`
	Function chatToolCallFunction `json:"function,omitempty"`
}

type chatToolCallFunction struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content   string         `json:"content"`
			ToolCalls []chatToolCall `json:"tool_calls,omitempty"`
		} `json:"message"`
	} `json:"choices"`
}

type chatStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content   string              `json:"content,omitempty"`
			ToolCalls []chatToolCallDelta `json:"tool_calls,omitempty"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason,omitempty"`
	} `json:"choices"`
}

type chatToolCallDelta struct {
	Index    int                  `json:"index"`
	ID       string               `json:"id,omitempty"`
	Type     string               `json:"type,omitempty"`
	Function chatToolCallFunction `json:"function,omitempty"`
}

type chatErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Param   string `json:"param"`
		Code    string `json:"code"`
	} `json:"error"`
}
