package channels

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tencent-connect/botgo/dto"
)

func TestResolveQQBotCredentials(t *testing.T) {
	t.Run("parses openclaw token", func(t *testing.T) {
		appID, appSecret := ResolveQQBotCredentials("", "", "1903066401:oX4NTL6ey96pKgoi")
		assert.Equal(t, "1903066401", appID)
		assert.Equal(t, "oX4NTL6ey96pKgoi", appSecret)
	})

	t.Run("uses access token as secret when app id already set", func(t *testing.T) {
		appID, appSecret := ResolveQQBotCredentials("1903066401", "", "oX4NTL6ey96pKgoi")
		assert.Equal(t, "1903066401", appID)
		assert.Equal(t, "oX4NTL6ey96pKgoi", appSecret)
	})

	t.Run("explicit secret wins", func(t *testing.T) {
		appID, appSecret := ResolveQQBotCredentials("1903066401", "real-secret", "1903066401:ignored")
		assert.Equal(t, "1903066401", appID)
		assert.Equal(t, "real-secret", appSecret)
	})
}

func TestQQChannelHandleOfficialC2CMessage(t *testing.T) {
	ch := NewQQChannel(&QQConfig{
		Enabled:     true,
		AccessToken: "1903066401:oX4NTL6ey96pKgoi",
		AllowFrom:   []string{"414797086"},
	})

	var got *Message
	ch.SetMessageHandler(func(msg *Message) {
		got = msg
	})

	handler := ch.handleOfficialC2CMessage()
	err := handler(nil, &dto.WSC2CMessageData{
		ID:      "msg-1",
		Content: "Hello QQ",
		Author: &dto.User{
			ID: "414797086",
		},
	})
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "Hello QQ", got.Text)
	assert.Equal(t, "414797086", got.ChatID)

	ch.mu.RLock()
	defer ch.mu.RUnlock()
	assert.Equal(t, "msg-1", ch.lastInboundMsg["414797086"])
}

func TestQQChannelSendMessageUsesLatestInboundMessageID(t *testing.T) {
	ch := NewQQChannel(&QQConfig{
		Enabled:     true,
		AccessToken: "1903066401:oX4NTL6ey96pKgoi",
	})

	ch.mu.Lock()
	ch.lastInboundMsg["414797086"] = "msg-1"
	ch.postC2C = func(_ context.Context, userID string, msg dto.APIMessage) error {
		assert.Equal(t, "414797086", userID)
		toCreate, ok := msg.(*dto.MessageToCreate)
		require.True(t, ok)
		assert.Equal(t, "reply text", toCreate.Content)
		assert.Equal(t, "msg-1", toCreate.MsgID)
		assert.Equal(t, dto.TextMsg, toCreate.MsgType)
		return nil
	}
	ch.mu.Unlock()

	require.NoError(t, ch.SendMessage("414797086", "reply text"))
}
