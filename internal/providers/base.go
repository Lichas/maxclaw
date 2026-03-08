package providers

import (
	"context"
)

type ContentPart struct {
	Type      string `json:"type"`
	Text      string `json:"text,omitempty"`
	ImageURL  string `json:"image_url,omitempty"`
	ImagePath string `json:"image_path,omitempty"`
	MimeType  string `json:"mime_type,omitempty"`
}

// Message 消息
type Message struct {
	Role       string `json:"role"`
	Content    string `json:"content"`
	Parts      []ContentPart
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
}

// ToolCall 工具调用
type ToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function ToolCallFunction `json:"function"`
}

// ToolCallFunction 工具调用函数
type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// Response LLM 响应
type Response struct {
	Content      string     `json:"content"`
	ToolCalls    []ToolCall `json:"tool_calls,omitempty"`
	HasToolCalls bool       `json:"has_tool_calls"`
}

// StreamHandler 流式响应处理器
type StreamHandler interface {
	OnContent(token string)           // 普通文本 token
	OnToolCallStart(id, name string)  // 开始 tool call
	OnToolCallDelta(id, delta string) // tool call 参数片段
	OnToolCallEnd(id string)          // tool call 结束
	OnComplete()                      // 流结束
	OnError(err error)                // 错误处理
}

// LLMProvider LLM 提供商接口
type LLMProvider interface {
	Chat(ctx context.Context, messages []Message, tools []map[string]interface{}, model string) (*Response, error)
	ChatStream(ctx context.Context, messages []Message, tools []map[string]interface{}, model string, handler StreamHandler) error
	GetDefaultModel() string
	SupportsImageInput(model string) bool
}
