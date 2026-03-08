package channels

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
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

func TestQQChannelHandleC2CMessageUsesUserOpenID(t *testing.T) {
	ch := NewQQChannel(&QQConfig{
		Enabled:     true,
		AccessToken: "1903066401:oX4NTL6ey96pKgoi",
		AllowFrom:   []string{"414797086"},
	})

	var got *Message
	ch.SetMessageHandler(func(msg *Message) {
		got = msg
	})

	ch.handleC2CMessage(&qqC2CMessageEvent{
		ID:      "msg-1",
		Content: "Hello QQ",
		Author: qqC2CAuthor{
			ID:          "opaque-id",
			UserOpenID:  "USER-OPENID",
			UnionOpenID: "UNION-OPENID",
		},
	})

	require.NotNil(t, got)
	assert.Equal(t, "Hello QQ", got.Text)
	assert.Equal(t, "USER-OPENID", got.ChatID)

	ch.mu.RLock()
	defer ch.mu.RUnlock()
	assert.Equal(t, "msg-1", ch.lastInboundMsg["USER-OPENID"])
}

func TestQQChannelHandleC2CMessageBlocksNonMatchingOpenIDAllowlist(t *testing.T) {
	ch := NewQQChannel(&QQConfig{
		Enabled:     true,
		AccessToken: "1903066401:oX4NTL6ey96pKgoi",
		AllowFrom:   []string{"ALLOWED-OPENID"},
	})

	var got *Message
	ch.SetMessageHandler(func(msg *Message) {
		got = msg
	})

	ch.handleC2CMessage(&qqC2CMessageEvent{
		ID:      "msg-1",
		Content: "Hello QQ",
		Author: qqC2CAuthor{
			UserOpenID: "OTHER-OPENID",
		},
	})

	assert.Nil(t, got)
}

func TestQQChannelSendMessageUsesLatestInboundMessageID(t *testing.T) {
	var (
		gotAuth string
		gotPath string
		gotBody map[string]interface{}
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		require.NoError(t, json.NewDecoder(r.Body).Decode(&gotBody))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp-1"}`))
	}))
	defer server.Close()

	serverURL, err := url.Parse(server.URL)
	require.NoError(t, err)

	ch := NewQQChannel(&QQConfig{
		Enabled:     true,
		AccessToken: "1903066401:oX4NTL6ey96pKgoi",
	})
	ch.httpClient = &http.Client{
		Transport: &rewriteHostTransport{
			target: http.DefaultTransport,
			base:   serverURL,
		},
	}

	ch.mu.Lock()
	ch.tokenSource = oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "test-access-token"})
	ch.lastInboundMsg["USER-OPENID"] = "msg-1"
	ch.mu.Unlock()

	require.NoError(t, ch.SendMessage("USER-OPENID", "reply text"))
	assert.Equal(t, "QQBot test-access-token", gotAuth)
	assert.Equal(t, "/v2/users/USER-OPENID/messages", gotPath)
	assert.Equal(t, "reply text", gotBody["content"])
	assert.Equal(t, "msg-1", gotBody["msg_id"])
	assert.Equal(t, float64(1), gotBody["msg_seq"])
	assert.Equal(t, float64(0), gotBody["msg_type"])
}

type rewriteHostTransport struct {
	target http.RoundTripper
	base   *url.URL
}

func (t *rewriteHostTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(context.Background())
	cloned.URL.Scheme = t.base.Scheme
	cloned.URL.Host = t.base.Host
	return t.target.RoundTrip(cloned)
}
