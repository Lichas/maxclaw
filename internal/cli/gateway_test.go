package cli

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Lichas/maxclaw/internal/bus"
	"github.com/Lichas/maxclaw/internal/channels"
	"github.com/Lichas/maxclaw/internal/config"
)

type mockChannel struct {
	name      string
	enabled   bool
	sendErr   error
	mu        sync.Mutex
	sendCalls int
	lastChat  string
	lastText  string
}

func (m *mockChannel) Name() string { return m.name }

func (m *mockChannel) Start(ctx context.Context) error { return nil }

func (m *mockChannel) Stop() error { return nil }

func (m *mockChannel) SendMessage(chatID string, text string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sendCalls++
	m.lastChat = chatID
	m.lastText = text
	return m.sendErr
}

func (m *mockChannel) SetMessageHandler(handler func(msg *channels.Message)) {}

func (m *mockChannel) IsEnabled() bool { return m.enabled }

func (m *mockChannel) snapshot() (calls int, chat, text string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sendCalls, m.lastChat, m.lastText
}

func eventually(t *testing.T, timeout time.Duration, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("condition not met within %s", timeout)
}

func TestHandleOutboundMessagesSendSuccess(t *testing.T) {
	messageBus := bus.NewMessageBus(10)
	registry := channels.NewRegistry()
	ch := &mockChannel{name: "telegram", enabled: true}
	registry.Register(ch)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go handleOutboundMessages(ctx, messageBus, registry)

	if err := messageBus.PublishOutbound(bus.NewOutboundMessage("telegram", "chat-42", "hello")); err != nil {
		t.Fatalf("publish outbound: %v", err)
	}

	eventually(t, time.Second, func() bool {
		calls, _, _ := ch.snapshot()
		return calls == 1
	})

	calls, chat, text := ch.snapshot()
	if calls != 1 || chat != "chat-42" || text != "hello" {
		t.Fatalf("unexpected send snapshot calls=%d chat=%q text=%q", calls, chat, text)
	}
}

func TestHandleOutboundMessagesDropEmptyChat(t *testing.T) {
	messageBus := bus.NewMessageBus(10)
	registry := channels.NewRegistry()
	ch := &mockChannel{name: "telegram", enabled: true}
	registry.Register(ch)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go handleOutboundMessages(ctx, messageBus, registry)

	if err := messageBus.PublishOutbound(bus.NewOutboundMessage("telegram", "", "hello")); err != nil {
		t.Fatalf("publish outbound: %v", err)
	}

	time.Sleep(120 * time.Millisecond)
	calls, _, _ := ch.snapshot()
	if calls != 0 {
		t.Fatalf("expected no send calls, got %d", calls)
	}
}

func TestHandleOutboundMessagesContinueAfterError(t *testing.T) {
	messageBus := bus.NewMessageBus(10)
	registry := channels.NewRegistry()
	ch := &mockChannel{name: "telegram", enabled: true, sendErr: errors.New("network down")}
	registry.Register(ch)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go handleOutboundMessages(ctx, messageBus, registry)

	if err := messageBus.PublishOutbound(bus.NewOutboundMessage("telegram", "chat-1", "a")); err != nil {
		t.Fatalf("publish outbound #1: %v", err)
	}
	if err := messageBus.PublishOutbound(bus.NewOutboundMessage("telegram", "chat-2", "b")); err != nil {
		t.Fatalf("publish outbound #2: %v", err)
	}

	eventually(t, time.Second, func() bool {
		calls, _, _ := ch.snapshot()
		return calls >= 2
	})
}

func TestBuildGatewayProviderWithoutAPIKeyFallsBack(t *testing.T) {
	cfg := config.DefaultConfig()
	provider, warning, err := buildGatewayProvider(cfg, "", "")
	if err != nil {
		t.Fatalf("buildGatewayProvider returned error: %v", err)
	}
	if provider == nil {
		t.Fatal("expected fallback provider, got nil")
	}
	if warning == "" {
		t.Fatal("expected startup warning for missing API key")
	}

	streamErr := provider.ChatStream(context.Background(), nil, nil, "", nil)
	if streamErr == nil {
		t.Fatal("expected stream error when API key is missing")
	}
	if !strings.Contains(streamErr.Error(), "no API key configured") {
		t.Fatalf("unexpected stream error: %v", streamErr)
	}
}

func TestBuildGatewayProviderWithAPIKeyUsesOpenAIProvider(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Model = "gpt-4o-mini"

	provider, warning, err := buildGatewayProvider(cfg, "sk-test", "https://example.com/v1")
	if err != nil {
		t.Fatalf("buildGatewayProvider returned error: %v", err)
	}
	if provider == nil {
		t.Fatal("expected provider, got nil")
	}
	if warning != "" {
		t.Fatalf("expected no warning with API key, got: %s", warning)
	}
}
