package channels

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWhatsAppSendAuthWithToken(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

	got := make(chan map[string]interface{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		var msg map[string]interface{}
		if err := json.Unmarshal(data, &msg); err == nil {
			got <- msg
		}
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	conn, _, err := websocket.DefaultDialer.DialContext(context.Background(), wsURL, nil)
	require.NoError(t, err)
	defer conn.Close()

	ch := NewWhatsAppChannel(&WhatsAppConfig{BridgeToken: "secret"})
	require.NoError(t, ch.sendAuth(conn))

	select {
	case msg := <-got:
		assert.Equal(t, "auth", msg["type"])
		assert.Equal(t, "secret", msg["token"])
	case <-time.After(2 * time.Second):
		t.Fatal("expected auth message from client")
	}
}

func TestWhatsAppSendAuthWithoutToken(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

	got := make(chan []byte, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		_ = conn.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
		_, data, err := conn.ReadMessage()
		if err == nil {
			got <- data
		}
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	conn, _, err := websocket.DefaultDialer.DialContext(context.Background(), wsURL, nil)
	require.NoError(t, err)
	defer conn.Close()

	ch := NewWhatsAppChannel(&WhatsAppConfig{BridgeToken: ""})
	require.NoError(t, ch.sendAuth(conn))

	select {
	case data := <-got:
		t.Fatalf("did not expect auth message, got: %s", string(data))
	case <-time.After(600 * time.Millisecond):
		// expected
	}
}
