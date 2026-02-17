package channels

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTelegramChannelIsAllowed(t *testing.T) {
	t.Run("allow all when allowFrom empty", func(t *testing.T) {
		ch := NewTelegramChannel(&TelegramConfig{
			Token:     "token",
			Enabled:   true,
			AllowFrom: []string{},
		})
		assert.True(t, ch.isAllowed(123456, "alice"))
	})

	t.Run("allow by numeric user id", func(t *testing.T) {
		ch := NewTelegramChannel(&TelegramConfig{
			Token:     "token",
			Enabled:   true,
			AllowFrom: []string{"123456"},
		})
		assert.True(t, ch.isAllowed(123456, "alice"))
		assert.False(t, ch.isAllowed(999999, "alice"))
	})

	t.Run("allow by username with or without at prefix", func(t *testing.T) {
		ch := NewTelegramChannel(&TelegramConfig{
			Token:     "token",
			Enabled:   true,
			AllowFrom: []string{"alice", "@bob"},
		})
		assert.True(t, ch.isAllowed(1, "alice"))
		assert.True(t, ch.isAllowed(2, "@alice"))
		assert.True(t, ch.isAllowed(3, "bob"))
		assert.False(t, ch.isAllowed(4, "charlie"))
	})

	t.Run("username missing should not match username allow list", func(t *testing.T) {
		ch := NewTelegramChannel(&TelegramConfig{
			Token:     "token",
			Enabled:   true,
			AllowFrom: []string{"alice"},
		})
		assert.False(t, ch.isAllowed(123456, ""))
	})
}
