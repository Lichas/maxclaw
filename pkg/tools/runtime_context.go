package tools

import "context"

type runtimeContextKey string

const (
	runtimeChannelKey runtimeContextKey = "channel"
	runtimeChatIDKey  runtimeContextKey = "chat_id"
)

// WithRuntimeContext injects channel/chat metadata for tools in the current request.
func WithRuntimeContext(ctx context.Context, channel, chatID string) context.Context {
	ctx = context.WithValue(ctx, runtimeChannelKey, channel)
	ctx = context.WithValue(ctx, runtimeChatIDKey, chatID)
	return ctx
}

// RuntimeContextFrom extracts channel/chat metadata from context.
func RuntimeContextFrom(ctx context.Context) (channel, chatID string) {
	if ctx == nil {
		return "", ""
	}

	if v, ok := ctx.Value(runtimeChannelKey).(string); ok {
		channel = v
	}
	if v, ok := ctx.Value(runtimeChatIDKey).(string); ok {
		chatID = v
	}
	return channel, chatID
}
