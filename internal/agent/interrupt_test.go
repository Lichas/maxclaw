package agent

import (
	"context"
	"testing"
	"time"

	"github.com/Lichas/maxclaw/internal/bus"
)

func TestInterruptibleContext_Cancel(t *testing.T) {
	ctx := context.Background()
	msgBus := bus.NewMessageBus(10)
	ic := NewInterruptibleContext(ctx, msgBus)

	done := make(chan bool)
	go func() {
		<-ic.Done()
		done <- true
	}()

	ic.RequestInterrupt(InterruptRequest{
		Mode: InterruptCancel,
	})

	select {
	case <-done:
		// 成功取消
	case <-time.After(time.Second):
		t.Fatal("cancel did not work")
	}
}

func TestInterruptibleContext_Append(t *testing.T) {
	ctx := context.Background()
	msgBus := bus.NewMessageBus(10)
	ic := NewInterruptibleContext(ctx, msgBus)

	msg := &bus.InboundMessage{Content: "补充信息"}
	ic.RequestInterrupt(InterruptRequest{
		Message: msg,
		Mode:    InterruptAppend,
	})

	appends := ic.GetPendingAppends()
	if len(appends) != 1 {
		t.Fatalf("expected 1 append, got %d", len(appends))
	}

	if appends[0].Content != "补充信息" {
		t.Errorf("expected content '补充信息', got %s", appends[0].Content)
	}
}

func TestInterruptibleContext_NoCancelOnAppend(t *testing.T) {
	ctx := context.Background()
	msgBus := bus.NewMessageBus(10)
	ic := NewInterruptibleContext(ctx, msgBus)

	msg := &bus.InboundMessage{Content: "补充信息"}
	ic.RequestInterrupt(InterruptRequest{
		Message: msg,
		Mode:    InterruptAppend,
	})

	// 补充模式不应该取消上下文
	select {
	case <-ic.Done():
		t.Fatal("append should not cancel context")
	case <-time.After(100 * time.Millisecond):
		// 正确：上下文没有被取消
	}
}

func TestInterruptibleContext_GetInterrupts(t *testing.T) {
	ctx := context.Background()
	msgBus := bus.NewMessageBus(10)
	ic := NewInterruptibleContext(ctx, msgBus)

	msg1 := &bus.InboundMessage{Content: "消息1"}
	msg2 := &bus.InboundMessage{Content: "消息2"}

	ic.RequestInterrupt(InterruptRequest{
		Message: msg1,
		Mode:    InterruptAppend,
	})
	ic.RequestInterrupt(InterruptRequest{
		Message: msg2,
		Mode:    InterruptCancel,
	})

	interrupts := ic.GetInterrupts()
	if len(interrupts) != 2 {
		t.Fatalf("expected 2 interrupts, got %d", len(interrupts))
	}
}

func TestInterruptibleContext_IsCancelled(t *testing.T) {
	ctx := context.Background()
	msgBus := bus.NewMessageBus(10)
	ic := NewInterruptibleContext(ctx, msgBus)

	if ic.IsCancelled() {
		t.Fatal("should not be cancelled initially")
	}

	ic.RequestInterrupt(InterruptRequest{
		Mode: InterruptCancel,
	})

	if !ic.IsCancelled() {
		t.Fatal("should be cancelled after cancel request")
	}
}

func TestInterruptibleContext_OnInterruptCallback(t *testing.T) {
	ctx := context.Background()
	msgBus := bus.NewMessageBus(10)
	ic := NewInterruptibleContext(ctx, msgBus)

	callbackCalled := make(chan InterruptRequest, 1)
	ic.SetOnInterrupt(func(req InterruptRequest) {
		callbackCalled <- req
	})

	msg := &bus.InboundMessage{Content: "测试消息"}
	ic.RequestInterrupt(InterruptRequest{
		Message: msg,
		Mode:    InterruptCancel,
	})

	select {
	case req := <-callbackCalled:
		if req.Mode != InterruptCancel {
			t.Errorf("expected mode cancel, got %s", req.Mode)
		}
	case <-time.After(time.Second):
		t.Fatal("callback was not called")
	}
}
