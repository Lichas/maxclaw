package channels

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Lichas/maxclaw/internal/logging"
	"github.com/gorilla/websocket"
)

// WebSocketConfig WebSocket 频道配置
// 用于与外部客户端通过 WebSocket 收发消息
type WebSocketConfig struct {
	Enabled      bool     `json:"enabled"`
	Host         string   `json:"host"`
	Port         int      `json:"port"`
	Path         string   `json:"path"`
	AllowOrigins []string `json:"allowOrigins"`
}

// WebSocketChannel WebSocket 频道
// 启动 HTTP 服务器并在指定路径提供 WS 连接
type WebSocketChannel struct {
	config         *WebSocketConfig
	messageHandler func(msg *Message)
	server         *http.Server
	listener       net.Listener
	stopChan       chan struct{}
	wg             sync.WaitGroup
	enabled        bool

	mu       sync.RWMutex
	clients  map[string]*websocket.Conn
	upgrader websocket.Upgrader
}

// NewWebSocketChannel 创建 WebSocket 频道
func NewWebSocketChannel(config *WebSocketConfig) *WebSocketChannel {
	cfg := *config
	if cfg.Host == "" {
		cfg.Host = "0.0.0.0"
	}
	if cfg.Port == 0 {
		cfg.Port = 18791
	}
	if cfg.Path == "" {
		cfg.Path = "/ws"
	}

	ch := &WebSocketChannel{
		config:   &cfg,
		stopChan: make(chan struct{}),
		enabled:  cfg.Enabled,
		clients:  make(map[string]*websocket.Conn),
	}

	ch.upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			if len(cfg.AllowOrigins) == 0 {
				return true
			}
			origin := r.Header.Get("Origin")
			for _, allowed := range cfg.AllowOrigins {
				if allowed == origin {
					return true
				}
			}
			return false
		},
	}

	return ch
}

// Name 返回频道名称
func (w *WebSocketChannel) Name() string {
	return "websocket"
}

// IsEnabled 是否启用
func (w *WebSocketChannel) IsEnabled() bool {
	return w.enabled
}

// SetMessageHandler 设置消息处理器
func (w *WebSocketChannel) SetMessageHandler(handler func(msg *Message)) {
	w.messageHandler = handler
}

// Start 启动 WebSocket 频道
func (w *WebSocketChannel) Start(ctx context.Context) error {
	if !w.enabled {
		return nil
	}

	mux := http.NewServeMux()
	mux.HandleFunc(w.config.Path, w.handleWebSocket)

	addr := fmt.Sprintf("%s:%d", w.config.Host, w.config.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	w.listener = listener
	w.server = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		if err := w.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			// ignore server closed
		}
	}()

	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		select {
		case <-ctx.Done():
		case <-w.stopChan:
		}
		_ = w.server.Shutdown(context.Background())
	}()

	return nil
}

// Stop 停止 WebSocket 频道
func (w *WebSocketChannel) Stop() error {
	if !w.enabled {
		return nil
	}

	close(w.stopChan)
	w.closeAll()
	w.wg.Wait()
	return nil
}

// SendMessage 发送消息
func (w *WebSocketChannel) SendMessage(chatID string, text string) error {
	if !w.enabled {
		return fmt.Errorf("websocket channel not enabled")
	}
	if chatID == "" {
		return fmt.Errorf("chatID is required")
	}

	w.mu.RLock()
	conn := w.clients[chatID]
	w.mu.RUnlock()
	if conn == nil {
		return fmt.Errorf("client not connected: %s", chatID)
	}

	payload := map[string]interface{}{
		"type":    "message",
		"chatId":  chatID,
		"sender":  "assistant",
		"content": text,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	if conn == nil {
		return fmt.Errorf("client not connected: %s", chatID)
	}
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		if lg := logging.Get(); lg != nil && lg.Channels != nil {
			lg.Channels.Printf("websocket send error chat=%s err=%v", chatID, err)
		}
		return err
	}
	if lg := logging.Get(); lg != nil && lg.Channels != nil {
		lg.Channels.Printf("websocket send chat=%s text=%q", chatID, logging.Truncate(text, 300))
	}
	return nil
}

func (w *WebSocketChannel) handleWebSocket(rw http.ResponseWriter, r *http.Request) {
	conn, err := w.upgrader.Upgrade(rw, r, nil)
	if err != nil {
		return
	}

	chatID := r.URL.Query().Get("chatId")
	if chatID == "" {
		chatID = r.URL.Query().Get("clientId")
	}
	if chatID == "" {
		chatID = "ws-" + randomID(8)
	}

	w.addClient(chatID, conn)
	_ = w.sendHello(conn, chatID)

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			break
		}
		w.handleInbound(chatID, data)
	}

	w.removeClient(chatID)
	_ = conn.Close()
}

func (w *WebSocketChannel) handleInbound(defaultChatID string, data []byte) {
	var msg wsInboundMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return
	}

	if msg.Type != "" && msg.Type != "message" {
		return
	}
	content := strings.TrimSpace(msg.Content)
	if content == "" {
		return
	}

	chatID := msg.ChatID
	if chatID == "" {
		chatID = defaultChatID
	}

	sender := msg.Sender
	if sender == "" {
		sender = chatID
	}

	if w.messageHandler != nil {
		w.messageHandler(&Message{
			ID:      msg.ID,
			Text:    content,
			Sender:  sender,
			ChatID:  chatID,
			Channel: "websocket",
			Raw:     msg,
		})
	}
	if lg := logging.Get(); lg != nil && lg.Channels != nil {
		lg.Channels.Printf("websocket inbound chat=%s sender=%s text=%q", chatID, sender, logging.Truncate(content, 300))
	}
}

func (w *WebSocketChannel) sendHello(conn *websocket.Conn, chatID string) error {
	payload := map[string]interface{}{
		"type":   "hello",
		"chatId": chatID,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return conn.WriteMessage(websocket.TextMessage, data)
}

func (w *WebSocketChannel) addClient(chatID string, conn *websocket.Conn) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if old := w.clients[chatID]; old != nil {
		_ = old.Close()
	}
	w.clients[chatID] = conn
}

func (w *WebSocketChannel) removeClient(chatID string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.clients, chatID)
}

func (w *WebSocketChannel) closeAll() {
	w.mu.Lock()
	defer w.mu.Unlock()
	for id, conn := range w.clients {
		_ = conn.Close()
		delete(w.clients, id)
	}
}

func randomID(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

type wsInboundMessage struct {
	Type    string `json:"type"`
	ID      string `json:"id"`
	ChatID  string `json:"chatId"`
	Sender  string `json:"sender"`
	Content string `json:"content"`
}
