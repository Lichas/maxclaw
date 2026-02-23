package agent

import (
	"context"
	"sync"
	"time"

	"github.com/Lichas/maxclaw/internal/bus"
)

// InterruptMode 插话模式
type InterruptMode string

const (
	InterruptNone   InterruptMode = ""       // 无中断
	InterruptCancel InterruptMode = "cancel" // 打断重来
	InterruptAppend InterruptMode = "append" // 补充上下文
)

// InterruptRequest 中断请求
type InterruptRequest struct {
	Message   *bus.InboundMessage
	Mode      InterruptMode
	Timestamp time.Time
}

// InterruptibleContext 可中断的上下文
type InterruptibleContext struct {
	ctx         context.Context
	cancel      context.CancelFunc
	mu          sync.RWMutex
	interrupts  []InterruptRequest
	onInterrupt func(InterruptRequest)
	appendQueue []*bus.InboundMessage
	parentBus   *bus.MessageBus
}

// NewInterruptibleContext 创建可中断上下文
func NewInterruptibleContext(parent context.Context, b *bus.MessageBus) *InterruptibleContext {
	ctx, cancel := context.WithCancel(parent)
	return &InterruptibleContext{
		ctx:         ctx,
		cancel:      cancel,
		interrupts:  make([]InterruptRequest, 0),
		appendQueue: make([]*bus.InboundMessage, 0),
		parentBus:   b,
	}
}

// RequestInterrupt 请求中断
func (ic *InterruptibleContext) RequestInterrupt(req InterruptRequest) {
	ic.mu.Lock()
	defer ic.mu.Unlock()

	ic.interrupts = append(ic.interrupts, req)

	if req.Mode == InterruptCancel {
		ic.cancel()
	}

	if req.Mode == InterruptAppend && req.Message != nil {
		ic.appendQueue = append(ic.appendQueue, req.Message)
	}

	if ic.onInterrupt != nil {
		go ic.onInterrupt(req)
	}
}

// GetPendingAppends 获取待处理的补充消息
func (ic *InterruptibleContext) GetPendingAppends() []*bus.InboundMessage {
	ic.mu.Lock()
	defer ic.mu.Unlock()

	result := ic.appendQueue
	ic.appendQueue = make([]*bus.InboundMessage, 0)
	return result
}

// GetInterrupts 获取所有中断请求
func (ic *InterruptibleContext) GetInterrupts() []InterruptRequest {
	ic.mu.RLock()
	defer ic.mu.RUnlock()

	result := make([]InterruptRequest, len(ic.interrupts))
	copy(result, ic.interrupts)
	return result
}

// SetOnInterrupt 设置中断回调
func (ic *InterruptibleContext) SetOnInterrupt(cb func(InterruptRequest)) {
	ic.mu.Lock()
	defer ic.mu.Unlock()
	ic.onInterrupt = cb
}

// Done 返回完成通道
func (ic *InterruptibleContext) Done() <-chan struct{} {
	return ic.ctx.Done()
}

// Err 返回错误
func (ic *InterruptibleContext) Err() error {
	return ic.ctx.Err()
}

// Context 返回底层 context
func (ic *InterruptibleContext) Context() context.Context {
	return ic.ctx
}

// IsCancelled 检查是否被取消
func (ic *InterruptibleContext) IsCancelled() bool {
	select {
	case <-ic.ctx.Done():
		return true
	default:
		return false
	}
}
