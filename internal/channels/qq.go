package channels

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
)

// QQConfig QQ 私聊配置（OneBot WebSocket）
type QQConfig struct {
	Enabled     bool
	WSURL       string
	AccessToken string
	AllowFrom   []string
}

// QQChannel QQ 私聊频道
type QQChannel struct {
	config         *QQConfig
	messageHandler func(msg *Message)

	conn     *websocket.Conn
	stopChan chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
	mu       sync.RWMutex
}

// NewQQChannel 创建 QQ 频道
func NewQQChannel(config *QQConfig) *QQChannel {
	if config == nil {
		config = &QQConfig{}
	}
	return &QQChannel{
		config:   config,
		stopChan: make(chan struct{}),
	}
}

// Name 返回频道名
func (q *QQChannel) Name() string {
	return "qq"
}

// IsEnabled 是否启用
func (q *QQChannel) IsEnabled() bool {
	return q.config.Enabled && strings.TrimSpace(q.config.WSURL) != ""
}

// SetMessageHandler 设置消息处理器
func (q *QQChannel) SetMessageHandler(handler func(msg *Message)) {
	q.messageHandler = handler
}

// Start 启动 QQ 连接
func (q *QQChannel) Start(ctx context.Context) error {
	if !q.IsEnabled() {
		return nil
	}

	headers := http.Header{}
	if token := strings.TrimSpace(q.config.AccessToken); token != "" {
		headers.Set("Authorization", "Bearer "+token)
	}

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, q.config.WSURL, headers)
	if err != nil {
		return err
	}

	q.mu.Lock()
	q.conn = conn
	q.mu.Unlock()

	q.wg.Add(1)
	go func() {
		defer q.wg.Done()
		q.readLoop()
	}()

	q.wg.Add(1)
	go func() {
		defer q.wg.Done()
		select {
		case <-ctx.Done():
		case <-q.stopChan:
		}
		q.mu.Lock()
		if q.conn != nil {
			_ = q.conn.Close()
			q.conn = nil
		}
		q.mu.Unlock()
	}()

	return nil
}

// Stop 停止 QQ 连接
func (q *QQChannel) Stop() error {
	q.stopOnce.Do(func() {
		close(q.stopChan)
	})

	q.mu.Lock()
	if q.conn != nil {
		_ = q.conn.Close()
		q.conn = nil
	}
	q.mu.Unlock()

	q.wg.Wait()
	return nil
}

// SendMessage 发送 QQ 私聊消息
func (q *QQChannel) SendMessage(chatID string, text string) error {
	if !q.IsEnabled() {
		return fmt.Errorf("qq channel not enabled")
	}
	q.mu.RLock()
	conn := q.conn
	q.mu.RUnlock()
	if conn == nil {
		return fmt.Errorf("qq channel not connected")
	}

	var userID interface{} = chatID
	if n, err := strconv.ParseInt(chatID, 10, 64); err == nil {
		userID = n
	}

	req := map[string]interface{}{
		"action": "send_private_msg",
		"params": map[string]interface{}{
			"user_id": userID,
			"message": text,
		},
	}
	return conn.WriteJSON(req)
}

type qqInboundMessage struct {
	PostType    string          `json:"post_type"`
	MessageType string          `json:"message_type"`
	UserID      json.RawMessage `json:"user_id"`
	MessageID   json.RawMessage `json:"message_id"`
	RawMessage  string          `json:"raw_message"`
	Message     string          `json:"message"`
}

func (q *QQChannel) readLoop() {
	q.mu.RLock()
	conn := q.conn
	q.mu.RUnlock()
	if conn == nil {
		return
	}

	for {
		var in qqInboundMessage
		if err := conn.ReadJSON(&in); err != nil {
			return
		}
		if in.PostType != "message" || in.MessageType != "private" {
			continue
		}

		sender := trimRawJSON(in.UserID)
		if sender == "" || !q.isAllowed(sender) {
			continue
		}

		text := strings.TrimSpace(in.RawMessage)
		if text == "" {
			text = strings.TrimSpace(in.Message)
		}
		if text == "" || q.messageHandler == nil {
			continue
		}

		q.messageHandler(&Message{
			ID:      trimRawJSON(in.MessageID),
			Text:    text,
			Sender:  sender,
			ChatID:  sender,
			Channel: "qq",
			Raw:     in,
		})
	}
}

func (q *QQChannel) isAllowed(sender string) bool {
	if len(q.config.AllowFrom) == 0 {
		return true
	}
	for _, v := range q.config.AllowFrom {
		if strings.TrimSpace(v) == sender {
			return true
		}
	}
	return false
}

func trimRawJSON(v json.RawMessage) string {
	s := strings.TrimSpace(string(v))
	s = strings.TrimPrefix(s, "\"")
	s = strings.TrimSuffix(s, "\"")
	return s
}
