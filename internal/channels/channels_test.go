package channels

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRegistry(t *testing.T) {
	registry := NewRegistry()
	require.NotNil(t, registry)
	assert.Empty(t, registry.GetAll())
}

func TestRegistryRegisterAndGet(t *testing.T) {
	registry := NewRegistry()

	// 创建模拟频道
	tgConfig := &TelegramConfig{Token: "test-token", Enabled: true}
	tgChannel := NewTelegramChannel(tgConfig)

	// 注册
	registry.Register(tgChannel)

	// 获取
	found, ok := registry.Get("telegram")
	assert.True(t, ok)
	assert.Equal(t, tgChannel, found)

	// 获取不存在的
	_, ok = registry.Get("non-existent")
	assert.False(t, ok)
}

func TestRegistryGetAll(t *testing.T) {
	registry := NewRegistry()

	// 注册多个频道
	tg := NewTelegramChannel(&TelegramConfig{Token: "tg", Enabled: true})
	dc := NewDiscordChannel(&DiscordConfig{Token: "dc", Enabled: true})

	registry.Register(tg)
	registry.Register(dc)

	all := registry.GetAll()
	assert.Len(t, all, 2)
}

func TestRegistryGetEnabled(t *testing.T) {
	registry := NewRegistry()

	// 启用的频道
	enabledTG := NewTelegramChannel(&TelegramConfig{Token: "tg", Enabled: true})
	// 禁用的频道（空 token 视为禁用）
	disabledDC := NewDiscordChannel(&DiscordConfig{Token: "", Enabled: false})

	registry.Register(enabledTG)
	registry.Register(disabledDC)

	enabled := registry.GetEnabled()
	assert.Len(t, enabled, 1)
	assert.Equal(t, "telegram", enabled[0].Name())
}

func TestTelegramChannelName(t *testing.T) {
	ch := NewTelegramChannel(&TelegramConfig{Token: "test", Enabled: true})
	assert.Equal(t, "telegram", ch.Name())
}

func TestTelegramChannelIsEnabled(t *testing.T) {
	tests := []struct {
		name    string
		config  *TelegramConfig
		enabled bool
	}{
		{
			name:    "enabled with token",
			config:  &TelegramConfig{Token: "valid-token", Enabled: true},
			enabled: true,
		},
		{
			name:    "disabled by flag",
			config:  &TelegramConfig{Token: "valid-token", Enabled: false},
			enabled: false,
		},
		{
			name:    "disabled by empty token",
			config:  &TelegramConfig{Token: "", Enabled: true},
			enabled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch := NewTelegramChannel(tt.config)
			assert.Equal(t, tt.enabled, ch.IsEnabled())
		})
	}
}

func TestDiscordChannelName(t *testing.T) {
	ch := NewDiscordChannel(&DiscordConfig{Token: "test", Enabled: true})
	assert.Equal(t, "discord", ch.Name())
}

func TestDiscordChannelIsEnabled(t *testing.T) {
	tests := []struct {
		name    string
		config  *DiscordConfig
		enabled bool
	}{
		{
			name:    "enabled with token",
			config:  &DiscordConfig{Token: "valid-token", Enabled: true},
			enabled: true,
		},
		{
			name:    "disabled by flag",
			config:  &DiscordConfig{Token: "valid-token", Enabled: false},
			enabled: false,
		},
		{
			name:    "disabled by empty token",
			config:  &DiscordConfig{Token: "", Enabled: true},
			enabled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch := NewDiscordChannel(tt.config)
			assert.Equal(t, tt.enabled, ch.IsEnabled())
		})
	}
}

func TestWebSocketChannelName(t *testing.T) {
	ch := NewWebSocketChannel(&WebSocketConfig{Enabled: true})
	assert.Equal(t, "websocket", ch.Name())
}

func TestWebSocketChannelIsEnabled(t *testing.T) {
	ch := NewWebSocketChannel(&WebSocketConfig{Enabled: true})
	assert.True(t, ch.IsEnabled())

	disabled := NewWebSocketChannel(&WebSocketConfig{Enabled: false})
	assert.False(t, disabled.IsEnabled())
}

func TestWhatsAppChannelName(t *testing.T) {
	ch := NewWhatsAppChannel(&WhatsAppConfig{
		Enabled:   true,
		BridgeURL: "ws://localhost:3001",
	})
	assert.Equal(t, "whatsapp", ch.Name())
}

func TestWhatsAppChannelIsEnabled(t *testing.T) {
	tests := []struct {
		name    string
		config  *WhatsAppConfig
		enabled bool
	}{
		{
			name: "enabled with required fields",
			config: &WhatsAppConfig{
				Enabled:   true,
				BridgeURL: "ws://localhost:3001",
			},
			enabled: true,
		},
		{
			name: "disabled by flag",
			config: &WhatsAppConfig{
				Enabled:   false,
				BridgeURL: "ws://localhost:3001",
			},
			enabled: false,
		},
		{
			name: "disabled missing bridge url",
			config: &WhatsAppConfig{
				Enabled:   true,
				BridgeURL: "",
			},
			enabled: false,
		},
		{
			name: "disabled missing bridge url (whitespace)",
			config: &WhatsAppConfig{
				Enabled:   true,
				BridgeURL: "   ",
			},
			enabled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch := NewWhatsAppChannel(tt.config)
			assert.Equal(t, tt.enabled, ch.IsEnabled())
		})
	}
}

func TestSlackChannelName(t *testing.T) {
	ch := NewSlackChannel(&SlackConfig{Enabled: true, BotToken: "xoxb-1", AppToken: "xapp-1"})
	assert.Equal(t, "slack", ch.Name())
}

func TestSlackChannelIsEnabled(t *testing.T) {
	tests := []struct {
		name    string
		config  *SlackConfig
		enabled bool
	}{
		{
			name: "enabled with tokens",
			config: &SlackConfig{
				Enabled:  true,
				BotToken: "xoxb-1",
				AppToken: "xapp-1",
			},
			enabled: true,
		},
		{
			name: "disabled without app token",
			config: &SlackConfig{
				Enabled:  true,
				BotToken: "xoxb-1",
			},
			enabled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch := NewSlackChannel(tt.config)
			assert.Equal(t, tt.enabled, ch.IsEnabled())
		})
	}
}

func TestEmailChannelName(t *testing.T) {
	ch := NewEmailChannel(&EmailConfig{})
	assert.Equal(t, "email", ch.Name())
}

func TestEmailChannelIsEnabled(t *testing.T) {
	ch := NewEmailChannel(&EmailConfig{
		Enabled:        true,
		ConsentGranted: true,
		IMAPHost:       "imap.example.com",
		IMAPUsername:   "bot@example.com",
		IMAPPassword:   "x",
		SMTPHost:       "smtp.example.com",
	})
	assert.True(t, ch.IsEnabled())

	disabled := NewEmailChannel(&EmailConfig{Enabled: false})
	assert.False(t, disabled.IsEnabled())
}

func TestQQChannelName(t *testing.T) {
	ch := NewQQChannel(&QQConfig{Enabled: true, WSURL: "ws://localhost:3002"})
	assert.Equal(t, "qq", ch.Name())
}

func TestQQChannelIsEnabled(t *testing.T) {
	ch := NewQQChannel(&QQConfig{Enabled: true, WSURL: "ws://localhost:3002"})
	assert.True(t, ch.IsEnabled())

	disabled := NewQQChannel(&QQConfig{Enabled: true, WSURL: ""})
	assert.False(t, disabled.IsEnabled())
}

func TestFeishuChannelName(t *testing.T) {
	ch := NewFeishuChannel(&FeishuConfig{Enabled: true, AppID: "cli", AppSecret: "sec"})
	assert.Equal(t, "feishu", ch.Name())
}

func TestFeishuChannelIsEnabled(t *testing.T) {
	ch := NewFeishuChannel(&FeishuConfig{Enabled: true, AppID: "cli", AppSecret: "sec"})
	assert.True(t, ch.IsEnabled())

	disabled := NewFeishuChannel(&FeishuConfig{Enabled: true, AppID: "cli"})
	assert.False(t, disabled.IsEnabled())
}

func TestDiscordHelperFunctions(t *testing.T) {
	t.Run("ParseMention", func(t *testing.T) {
		assert.Equal(t, "<@123456>", ParseMention("123456"))
	})

	t.Run("ParseChannel", func(t *testing.T) {
		assert.Equal(t, "<#123456>", ParseChannel("123456"))
	})

	t.Run("EscapeMarkdown", func(t *testing.T) {
		text := "Hello *world* _test_"
		escaped := EscapeMarkdown(text)
		assert.Contains(t, escaped, "\\*")
		assert.Contains(t, escaped, "\\_")
	})
}

func TestMessageStruct(t *testing.T) {
	msg := &Message{
		ID:      "123",
		Text:    "Hello",
		Sender:  "user1",
		ChatID:  "chat123",
		Channel: "telegram",
		Raw:     map[string]interface{}{"extra": "data"},
	}

	assert.Equal(t, "123", msg.ID)
	assert.Equal(t, "Hello", msg.Text)
	assert.Equal(t, "user1", msg.Sender)
	assert.Equal(t, "chat123", msg.ChatID)
	assert.Equal(t, "telegram", msg.Channel)
	assert.NotNil(t, msg.Raw)
}

func TestSetMessageHandler(t *testing.T) {
	t.Run("Telegram", func(t *testing.T) {
		ch := NewTelegramChannel(&TelegramConfig{Token: "test", Enabled: true})
		ch.SetMessageHandler(func(msg *Message) {
			// handler set
		})
		// 验证处理器已设置（不触发实际调用）
		assert.NotNil(t, ch.messageHandler)
	})

	t.Run("Discord", func(t *testing.T) {
		ch := NewDiscordChannel(&DiscordConfig{Token: "test", Enabled: true})
		ch.SetMessageHandler(func(msg *Message) {
			// handler set
		})
		assert.NotNil(t, ch.messageHandler)
	})
}

func TestSendMessageNotEnabled(t *testing.T) {
	t.Run("Telegram", func(t *testing.T) {
		ch := NewTelegramChannel(&TelegramConfig{Token: "", Enabled: false})
		err := ch.SendMessage("123", "Hello")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not enabled")
	})

	t.Run("Discord", func(t *testing.T) {
		ch := NewDiscordChannel(&DiscordConfig{Token: "", Enabled: false})
		err := ch.SendMessage("123", "Hello")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not enabled")
	})

	t.Run("WhatsApp", func(t *testing.T) {
		ch := NewWhatsAppChannel(&WhatsAppConfig{Enabled: false})
		err := ch.SendMessage("123", "Hello")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not enabled")
	})

	t.Run("Slack", func(t *testing.T) {
		ch := NewSlackChannel(&SlackConfig{Enabled: false})
		err := ch.SendMessage("C123", "Hello")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not enabled")
	})

	t.Run("QQ", func(t *testing.T) {
		ch := NewQQChannel(&QQConfig{Enabled: false})
		err := ch.SendMessage("123", "Hello")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not enabled")
	})

	t.Run("Feishu", func(t *testing.T) {
		ch := NewFeishuChannel(&FeishuConfig{Enabled: false})
		err := ch.SendMessage("ou_x", "Hello")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not enabled")
	})

	t.Run("Email", func(t *testing.T) {
		ch := NewEmailChannel(&EmailConfig{Enabled: false})
		err := ch.SendMessage("u@example.com", "Hello")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not enabled")
	})
}
