package tools

import (
	"context"
	"fmt"
	"sync"
)

// MessageCallback 消息发送回调函数类型
type MessageCallback func(channel, chatID, content string) error

// MessageTool 消息发送工具
type MessageTool struct {
	BaseTool
	callback MessageCallback
	mu       sync.RWMutex
	channel  string
	chatID   string
}

// NewMessageTool 创建消息发送工具
func NewMessageTool(callback MessageCallback) *MessageTool {
	return &MessageTool{
		BaseTool: BaseTool{
			name:        "message",
			description: "Send a message to the user through the current channel. Use to communicate results, ask questions, or provide updates.",
			parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"content": map[string]interface{}{
						"type":        "string",
						"description": "Message content to send",
						"minLength":   1,
					},
					"channel": map[string]interface{}{
						"type":        "string",
						"description": "Channel to send to (optional, uses current if not specified)",
					},
					"chat_id": map[string]interface{}{
						"type":        "string",
						"description": "Chat ID to send to (optional, uses current if not specified)",
					},
				},
				"required": []string{"content"},
			},
		},
		callback: callback,
	}
}

// SetContext 设置当前上下文
func (t *MessageTool) SetContext(channel, chatID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.channel = channel
	t.chatID = chatID
}

// Execute 执行消息发送
func (t *MessageTool) Execute(ctx context.Context, params map[string]interface{}) (string, error) {
	content, _ := params["content"].(string)
	if content == "" {
		return "", fmt.Errorf("content is required")
	}

	t.mu.RLock()
	channel := t.channel
	chatID := t.chatID
	t.mu.RUnlock()

	// 优先使用当前请求上下文，避免并发请求时的上下文串线
	if ctxChannel, ctxChatID := RuntimeContextFrom(ctx); ctxChannel != "" || ctxChatID != "" {
		if channel == "" {
			channel = ctxChannel
		}
		if chatID == "" {
			chatID = ctxChatID
		}
	}

	// 允许通过参数覆盖
	if v, ok := params["channel"].(string); ok && v != "" {
		channel = v
	}
	if v, ok := params["chat_id"].(string); ok && v != "" {
		chatID = v
	}

	if channel == "" || chatID == "" {
		return "", fmt.Errorf("channel and chat_id must be set")
	}

	if t.callback != nil {
		if err := t.callback(channel, chatID, content); err != nil {
			return "", fmt.Errorf("failed to send message: %w", err)
		}
	}

	return "Message sent successfully", nil
}
