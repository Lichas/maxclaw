package providers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const maxInlineImageBytes = 8 * 1024 * 1024

// debug flag - set to false in production
const debug = false

// OpenAIProvider OpenAI 提供商实现
// 使用 OpenAI 兼容 API (string content) 以支持 DeepSeek 等提供商
type OpenAIProvider struct {
	apiKey             string
	apiBase            string
	defaultModel       string
	maxTokens          int
	temperature        float64
	httpClient         *http.Client
	streamClient       *http.Client
	supportsImageInput func(model string) bool
}

// NewOpenAIProvider 创建 OpenAI 提供商
func NewOpenAIProvider(apiKey, apiBase, defaultModel string, maxTokens int, temperature float64, supportsImageInput func(model string) bool) (*OpenAIProvider, error) {
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
		streamClient:       &http.Client{},
		supportsImageInput: supportsImageInput,
	}, nil
}

// Chat 发送聊天请求
func (p *OpenAIProvider) Chat(ctx context.Context, messages []Message, tools []map[string]interface{}, model string) (*Response, error) {
	if model == "" {
		model = p.defaultModel
	}

	reqBody := buildChatRequest(messages, tools, model, p.SupportsImageInput(model), false, p.maxTokens, p.temperature)
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	if debug {
		fmt.Printf("[OpenAIProvider] Request:\n%s\n", string(payload))
	}

	respBody, err := p.doRequest(ctx, payload, false, model)
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

func (p *OpenAIProvider) SupportsImageInput(model string) bool {
	if model == "" {
		model = p.defaultModel
	}
	if p.supportsImageInput != nil {
		return p.supportsImageInput(model)
	}
	return SupportsImageInput(p.detectProvider(model), model)
}

// ChatStream 流式聊天请求
func (p *OpenAIProvider) ChatStream(ctx context.Context, messages []Message, tools []map[string]interface{}, model string, handler StreamHandler) error {
	if model == "" {
		model = p.defaultModel
	}

	reqBody := buildChatRequest(messages, tools, model, p.SupportsImageInput(model), true, p.maxTokens, p.temperature)
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to encode request: %w", err)
	}

	if debug {
		fmt.Printf("[OpenAIProvider] Stream Request:\n%s\n", string(payload))
	}

	stream, err := p.doStreamRequest(ctx, payload, model)
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

	for _, spec := range ProviderSpecs {
		if spec.MatchesModel(normalizedModel) {
			return spec.Name
		}
	}

	return DetectProviderNameFromAPIBase(p.apiBase)
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
func buildChatRequest(messages []Message, tools []map[string]interface{}, model string, allowImageInput bool, stream bool, maxTokens int, temperature float64) chatRequest {
	if maxTokens <= 0 {
		maxTokens = 1
	}

	reqBody := chatRequest{
		Model:       model,
		Messages:    convertToChatMessages(messages, allowImageInput),
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

// convertToChatMessages 转换消息格式为 OpenAI 兼容格式
func convertToChatMessages(messages []Message, allowContentParts bool) []chatMessage {
	result := make([]chatMessage, len(messages))
	for i, msg := range messages {
		cm := chatMessage{
			Role: msg.Role,
		}

		if allowContentParts && len(msg.Parts) > 0 {
			cm.Content = convertToChatContentParts(msg.Parts)
		} else if len(msg.Parts) > 0 {
			cm.Content = flattenContentParts(msg)
		} else {
			// 对于所有角色，都设置content字段（即使是空字符串）
			// 有些API实现要求content字段必须存在
			cm.Content = msg.Content
		}

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

func flattenContentParts(msg Message) string {
	var lines []string
	if strings.TrimSpace(msg.Content) != "" {
		lines = append(lines, msg.Content)
	}
	for _, part := range msg.Parts {
		switch part.Type {
		case "image_url":
			lines = append(lines, "User also attached an image, but the current model cannot inspect images directly.")
		case "text":
			text := strings.TrimSpace(part.Text)
			if text != "" && text != strings.TrimSpace(msg.Content) {
				lines = append(lines, text)
			}
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func convertToChatContentParts(parts []ContentPart) []chatContentPart {
	result := make([]chatContentPart, 0, len(parts))
	for _, part := range parts {
		switch part.Type {
		case "image_url":
			imageURL := buildProviderImageURL(part)
			if strings.TrimSpace(imageURL) == "" {
				continue
			}
			result = append(result, chatContentPart{
				Type: "image_url",
				ImageURL: &chatImageURL{
					URL: imageURL,
				},
			})
		default:
			result = append(result, chatContentPart{
				Type: "text",
				Text: part.Text,
			})
		}
	}
	return result
}

func buildProviderImageURL(part ContentPart) string {
	if dataURL := buildInlineDataURL(part); dataURL != "" {
		return dataURL
	}
	return strings.TrimSpace(part.ImageURL)
}

func buildInlineDataURL(part ContentPart) string {
	path := strings.TrimSpace(part.ImagePath)
	if path == "" {
		return ""
	}

	info, err := os.Stat(path)
	if err != nil || info.Size() <= 0 || info.Size() > maxInlineImageBytes {
		return ""
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	mimeType := strings.TrimSpace(part.MimeType)
	if mimeType == "" {
		if ext := strings.TrimSpace(filepath.Ext(path)); ext != "" {
			mimeType = mime.TypeByExtension(ext)
		}
	}
	if mimeType == "" {
		mimeType = http.DetectContentType(data)
	}
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	return "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data)
}

// doRequest 执行非流式请求（带重试机制）
func (p *OpenAIProvider) doRequest(ctx context.Context, payload []byte, stream bool, model string) ([]byte, error) {
	endpoint := p.apiBase + "/chat/completions"

	const maxRetries = 3
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// 重试前等待，使用指数退避
			backoff := time.Duration(attempt) * time.Second
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		setCommonHeaders(req, p.apiKey, p.apiBase, p.detectProvider(model))

		if stream {
			req.Header.Set("Accept", "text/event-stream")
		}

		resp, err := p.httpClient.Do(req)
		if err != nil {
			lastErr = err
			// 检查是否是可重试的错误
			if isRetryableError(err) {
				continue
			}
			return nil, fmt.Errorf("chat completion failed: %w", err)
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			if isRetryableError(err) {
				continue
			}
			return nil, fmt.Errorf("failed to read response: %w", err)
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			// 服务器错误时重试
			if resp.StatusCode >= 500 || resp.StatusCode == 429 {
				lastErr = fmt.Errorf("chat completion failed: %s", formatAPIError(body, resp.StatusCode))
				continue
			}
			return nil, fmt.Errorf("chat completion failed: %s", formatAPIError(body, resp.StatusCode))
		}

		return body, nil
	}

	return nil, fmt.Errorf("chat completion failed after %d attempts: %w", maxRetries, lastErr)
}

// isRetryableError 检查错误是否可重试
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	// 检查上下文取消或超时
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	// 检查网络错误
	if netErr, ok := err.(interface{ Timeout() bool }); ok && netErr.Timeout() {
		return true
	}
	// 检查临时错误
	if netErr, ok := err.(interface{ Temporary() bool }); ok && netErr.Temporary() {
		return true
	}
	// 检查常见错误字符串
	errStr := strings.ToLower(err.Error())
	retryableStrings := []string{
		"timeout", "deadline exceeded", "context canceled",
		"connection refused", "connection reset", "broken pipe",
		"no such host", "temporary", "retry",
	}
	for _, s := range retryableStrings {
		if strings.Contains(errStr, s) {
			return true
		}
	}
	return false
}

// doStreamRequest 执行流式请求（带重试机制）
func (p *OpenAIProvider) doStreamRequest(ctx context.Context, payload []byte, model string) (io.ReadCloser, error) {
	endpoint := p.apiBase + "/chat/completions"

	const maxRetries = 3
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// 重试前等待，使用指数退避
			backoff := time.Duration(attempt) * time.Second
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		setCommonHeaders(req, p.apiKey, p.apiBase, p.detectProvider(model))
		req.Header.Set("Accept", "text/event-stream")

		resp, err := p.streamClient.Do(req)
		if err != nil {
			lastErr = err
			// 检查是否是可重试的错误
			if isRetryableError(err) {
				continue
			}
			return nil, fmt.Errorf("stream request failed: %w", err)
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			// 服务器错误或限流时重试
			if resp.StatusCode >= 500 || resp.StatusCode == 429 {
				lastErr = fmt.Errorf("stream request failed: %s", formatAPIError(body, resp.StatusCode))
				continue
			}
			return nil, fmt.Errorf("stream request failed: %s", formatAPIError(body, resp.StatusCode))
		}

		return resp.Body, nil
	}

	return nil, fmt.Errorf("stream request failed after %d attempts: %w", maxRetries, lastErr)
}

func setCommonHeaders(req *http.Request, apiKey, apiBase, provider string) {
	req.Header.Set("Authorization", authorizationHeaderValue(apiKey, apiBase, provider))
	req.Header.Set("Content-Type", "application/json")
}

func authorizationHeaderValue(apiKey, apiBase, provider string) string {
	return "Bearer " + apiKey
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
	Content    interface{}    `json:"content"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
	ToolCalls  []chatToolCall `json:"tool_calls,omitempty"`
}

type chatContentPart struct {
	Type     string        `json:"type"`
	Text     string        `json:"text,omitempty"`
	ImageURL *chatImageURL `json:"image_url,omitempty"`
}

type chatImageURL struct {
	URL string `json:"url"`
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
