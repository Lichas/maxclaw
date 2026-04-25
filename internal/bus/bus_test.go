package bus

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMessageBus(t *testing.T) {
	bus := NewMessageBus(10)
	assert.NotNil(t, bus)
	assert.False(t, bus.IsClosed())
}

func TestPublishAndConsumeInbound(t *testing.T) {
	bus := NewMessageBus(10)
	defer bus.Close()

	msg := NewInboundMessage("telegram", "user123", "chat456", "Hello!")

	// 发布消息
	err := bus.PublishInbound(msg)
	require.NoError(t, err)

	// 消费消息
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	received, err := bus.ConsumeInbound(ctx)
	require.NoError(t, err)
	assert.Equal(t, "telegram", received.Channel)
	assert.Equal(t, "user123", received.SenderID)
	assert.Equal(t, "chat456", received.ChatID)
	assert.Equal(t, "Hello!", received.Content)
	assert.Equal(t, "telegram:chat456", received.SessionKey)
}

func TestPublishAndConsumeOutbound(t *testing.T) {
	bus := NewMessageBus(10)
	defer bus.Close()

	msg := NewOutboundMessage("telegram", "chat456", "Response!")

	// 发布消息
	err := bus.PublishOutbound(msg)
	require.NoError(t, err)

	// 消费消息
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	received, err := bus.ConsumeOutbound(ctx)
	require.NoError(t, err)
	assert.Equal(t, "telegram", received.Channel)
	assert.Equal(t, "chat456", received.ChatID)
	assert.Equal(t, "Response!", received.Content)
}

func TestTryConsume(t *testing.T) {
	bus := NewMessageBus(10)
	defer bus.Close()

	// 没有消息时
	msg, ok := bus.TryConsumeInbound()
	assert.False(t, ok)
	assert.Nil(t, msg)

	// 发布消息
	bus.PublishInbound(NewInboundMessage("cli", "user", "chat", "test"))

	// 现在有消息了
	msg, ok = bus.TryConsumeInbound()
	assert.True(t, ok)
	assert.NotNil(t, msg)
	assert.Equal(t, "test", msg.Content)
}

func TestBufferFull(t *testing.T) {
	// 创建一个容量为 1 的总线
	bus := NewMessageBus(1)
	defer bus.Close()

	// 第一条应该成功
	err := bus.PublishInbound(NewInboundMessage("cli", "user", "chat1", "msg1"))
	require.NoError(t, err)

	// 第二条应该失败（缓冲区已满）
	err = bus.PublishInbound(NewInboundMessage("cli", "user", "chat2", "msg2"))
	assert.Equal(t, ErrBufferFull, err)
}

func TestClose(t *testing.T) {
	bus := NewMessageBus(10)

	assert.False(t, bus.IsClosed())

	bus.Close()

	assert.True(t, bus.IsClosed())

	// 关闭后发布应该失败
	err := bus.PublishInbound(NewInboundMessage("cli", "user", "chat", "test"))
	assert.Equal(t, ErrBusClosed, err)
}

func TestConsumeTimeout(t *testing.T) {
	bus := NewMessageBus(10)
	defer bus.Close()

	// 使用超时的 context
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := bus.ConsumeInbound(ctx)
	assert.Equal(t, context.DeadlineExceeded, err)
}

func TestConcurrentAccess(t *testing.T) {
	bus := NewMessageBus(100)
	defer bus.Close()

	// 并发发布
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(i int) {
			msg := NewInboundMessage("cli", "user", "chat", string(rune('a'+i)))
			bus.PublishInbound(msg)
			done <- true
		}(i)
	}

	// 等待所有发布完成
	for i := 0; i < 10; i++ {
		<-done
	}

	// 验证所有消息都被消费
	count := 0
	for {
		_, ok := bus.TryConsumeInbound()
		if !ok {
			break
		}
		count++
		if count >= 10 {
			break
		}
	}

	assert.Equal(t, 10, count)
}

func TestSubscribeOutbound(t *testing.T) {
	bus := NewMessageBus(10)
	defer bus.Close()

	var (
		mu       sync.Mutex
		received []*OutboundMessage
	)

	unsubscribe := bus.SubscribeOutbound(func(msg *OutboundMessage) {
		mu.Lock()
		defer mu.Unlock()
		received = append(received, msg)
	})

	err := bus.PublishOutbound(NewOutboundMessage("desktop", "desktop:test", "hello"))
	require.NoError(t, err)

	mu.Lock()
	require.Len(t, received, 1)
	assert.Equal(t, "hello", received[0].Content)
	mu.Unlock()

	unsubscribe()

	err = bus.PublishOutbound(NewOutboundMessage("desktop", "desktop:test", "ignored"))
	require.NoError(t, err)

	mu.Lock()
	assert.Len(t, received, 1)
	mu.Unlock()
}
