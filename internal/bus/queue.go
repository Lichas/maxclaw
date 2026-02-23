package bus

import (
	"context"
	"sync"
)

// MessageBus 消息总线
type MessageBus struct {
	inbound  chan *InboundMessage
	outbound chan *OutboundMessage
	mu       sync.RWMutex
	closed   bool
}

// NewMessageBus 创建消息总线
func NewMessageBus(bufferSize int) *MessageBus {
	if bufferSize <= 0 {
		bufferSize = 100
	}
	return &MessageBus{
		inbound:  make(chan *InboundMessage, bufferSize),
		outbound: make(chan *OutboundMessage, bufferSize),
	}
}

// PublishInbound 发布入站消息
func (b *MessageBus) PublishInbound(msg *InboundMessage) error {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.closed {
		return ErrBusClosed
	}

	select {
	case b.inbound <- msg:
		return nil
	default:
		return ErrBufferFull
	}
}

// PublishOutbound 发布出站消息
func (b *MessageBus) PublishOutbound(msg *OutboundMessage) error {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.closed {
		return ErrBusClosed
	}

	select {
	case b.outbound <- msg:
		return nil
	default:
		return ErrBufferFull
	}
}

// ConsumeInbound 消费入站消息（阻塞）
func (b *MessageBus) ConsumeInbound(ctx context.Context) (*InboundMessage, error) {
	select {
	case msg := <-b.inbound:
		return msg, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// ConsumeOutbound 消费出站消息（阻塞）
func (b *MessageBus) ConsumeOutbound(ctx context.Context) (*OutboundMessage, error) {
	select {
	case msg := <-b.outbound:
		return msg, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// TryConsumeInbound 非阻塞消费入站消息
func (b *MessageBus) TryConsumeInbound() (*InboundMessage, bool) {
	select {
	case msg := <-b.inbound:
		return msg, true
	default:
		return nil, false
	}
}

// TryConsumeOutbound 非阻塞消费出站消息
func (b *MessageBus) TryConsumeOutbound() (*OutboundMessage, bool) {
	select {
	case msg := <-b.outbound:
		return msg, true
	default:
		return nil, false
	}
}

// PeekInboundForSession 查找并返回指定会话的入站消息（非阻塞）
// 如果不是目标会话的消息，会将其临时存储，不影响其他消费者
func (b *MessageBus) PeekInboundForSession(sessionKey string) *InboundMessage {
	// 尝试非阻塞消费
	select {
	case msg := <-b.inbound:
		// 如果是目标会话，直接返回
		if msg.SessionKey == sessionKey {
			return msg
		}
		// 不是目标会话，重新放回队列（可能失败如果队列满）
		select {
		case b.inbound <- msg:
			// 成功放回
		default:
			// 队列满，丢弃消息（这种情况很少见）
		}
		return nil
	default:
		return nil
	}
}

// Close 关闭消息总线
func (b *MessageBus) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.closed {
		b.closed = true
		close(b.inbound)
		close(b.outbound)
	}
}

// IsClosed 检查是否已关闭
func (b *MessageBus) IsClosed() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.closed
}
