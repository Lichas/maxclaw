package channels

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Lichas/maxclaw/internal/logging"
	"github.com/gorilla/websocket"
)

// WhatsAppConfig WhatsApp 配置（桥接模式）
// 通过 WebSocket 连接到 Node.js Bridge (Baileys)
type WhatsAppConfig struct {
	Enabled     bool     `json:"enabled"`
	BridgeURL   string   `json:"bridgeUrl"`
	BridgeToken string   `json:"bridgeToken"`
	AllowFrom   []string `json:"allowFrom"`
	AllowSelf   bool     `json:"allowSelf"`
}

// WhatsAppChannel WhatsApp 频道
// 通过 WebSocket 与桥接服务通信
// 消息协议参考 maxclaw Python 版 whatsapp.py 与 bridge/src
type WhatsAppChannel struct {
	config         *WhatsAppConfig
	messageHandler func(msg *Message)
	stopChan       chan struct{}
	wg             sync.WaitGroup
	enabled        bool

	mu        sync.RWMutex
	conn      *websocket.Conn
	connected bool
	status    string
	lastQR    string
	lastQRAt  time.Time

	outboundMu sync.Mutex
	outbound   []outboundRecord
}

// NewWhatsAppChannel 创建 WhatsApp 频道
func NewWhatsAppChannel(config *WhatsAppConfig) *WhatsAppChannel {
	enabled := config.Enabled && strings.TrimSpace(config.BridgeURL) != ""
	return &WhatsAppChannel{
		config:   config,
		stopChan: make(chan struct{}),
		enabled:  enabled,
	}
}

// Name 返回频道名称
func (w *WhatsAppChannel) Name() string {
	return "whatsapp"
}

// IsEnabled 是否启用
func (w *WhatsAppChannel) IsEnabled() bool {
	return w.enabled
}

// SetMessageHandler 设置消息处理器
func (w *WhatsAppChannel) SetMessageHandler(handler func(msg *Message)) {
	w.messageHandler = handler
}

// Start 启动 WhatsApp 频道（连接桥接服务）
func (w *WhatsAppChannel) Start(ctx context.Context) error {
	if !w.enabled {
		return nil
	}

	w.wg.Add(1)
	go w.connectLoop(ctx)

	return nil
}

// Stop 停止 WhatsApp 频道
func (w *WhatsAppChannel) Stop() error {
	if !w.enabled {
		return nil
	}

	close(w.stopChan)
	w.closeConn()
	w.wg.Wait()
	return nil
}

// SendMessage 发送消息
func (w *WhatsAppChannel) SendMessage(chatID string, text string) error {
	if !w.enabled {
		return fmt.Errorf("whatsapp channel not enabled")
	}

	w.mu.RLock()
	conn := w.conn
	w.mu.RUnlock()

	if conn == nil || !w.connected {
		return fmt.Errorf("whatsapp bridge not connected")
	}

	payload := map[string]interface{}{
		"type": "send",
		"to":   chatID,
		"text": text,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	if w.conn == nil {
		return fmt.Errorf("whatsapp bridge not connected")
	}
	if err := w.conn.WriteMessage(websocket.TextMessage, data); err != nil {
		if lg := logging.Get(); lg != nil && lg.Channels != nil {
			lg.Channels.Printf("whatsapp send error chat=%s err=%v", chatID, err)
		}
		return err
	}

	w.rememberOutbound(chatID, text)
	if lg := logging.Get(); lg != nil && lg.Channels != nil {
		lg.Channels.Printf("whatsapp send chat=%s text=%q", chatID, logging.Truncate(text, 300))
	}
	return nil
}

func (w *WhatsAppChannel) connectLoop(ctx context.Context) {
	defer w.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopChan:
			return
		default:
		}

		conn, _, err := websocket.DefaultDialer.DialContext(ctx, w.config.BridgeURL, nil)
		if err != nil {
			w.setConnected(false, nil)
			if !w.waitRetry(ctx, 5*time.Second) {
				return
			}
			continue
		}

		if err := w.sendAuth(conn); err != nil {
			_ = conn.Close()
			w.setConnected(false, nil)
			if lg := logging.Get(); lg != nil && lg.Channels != nil {
				lg.Channels.Printf("whatsapp auth handshake failed: %v", err)
			}
			if !w.waitRetry(ctx, 5*time.Second) {
				return
			}
			continue
		}

		w.setConnected(true, conn)
		w.readLoop(ctx, conn)
		w.setConnected(false, nil)

		if !w.waitRetry(ctx, 5*time.Second) {
			return
		}
	}
}

func (w *WhatsAppChannel) sendAuth(conn *websocket.Conn) error {
	token := strings.TrimSpace(w.config.BridgeToken)
	if token == "" {
		return nil
	}
	payload := map[string]string{
		"type":  "auth",
		"token": token,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	defer conn.SetWriteDeadline(time.Time{})
	return conn.WriteMessage(websocket.TextMessage, data)
}

func (w *WhatsAppChannel) readLoop(ctx context.Context, conn *websocket.Conn) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopChan:
			return
		default:
		}

		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}

		w.handleBridgeMessage(data)
	}
}

func (w *WhatsAppChannel) handleBridgeMessage(data []byte) {
	var msg bridgeMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return
	}

	switch msg.Type {
	case "message":
		if msg.Content == "" || msg.Sender == "" {
			return
		}
		if msg.FromMe && !w.config.AllowSelf {
			return
		}
		if msg.FromMe && w.config.AllowSelf && w.isOutboundEcho(msg.Sender, msg.Content) {
			return
		}
		if !w.isAllowedSender(msg.Sender) {
			return
		}

		senderID := normalizeSender(msg.Sender)
		chatID := msg.Sender

		if w.messageHandler != nil {
			w.messageHandler(&Message{
				ID:      msg.ID,
				Text:    msg.Content,
				Sender:  senderID,
				ChatID:  chatID,
				Channel: "whatsapp",
				Raw:     msg,
			})
		}
		if lg := logging.Get(); lg != nil && lg.Channels != nil {
			lg.Channels.Printf("whatsapp inbound chat=%s sender=%s fromMe=%v text=%q", chatID, senderID, msg.FromMe, logging.Truncate(msg.Content, 300))
		}
	case "status":
		if msg.Status == "connected" {
			w.setStatusConnected(true)
			w.setStatus("connected")
		} else if msg.Status == "disconnected" {
			w.setStatusConnected(false)
			w.setStatus("disconnected")
		}
	case "qr":
		if msg.QR != "" {
			w.setQR(msg.QR)
		}
	case "error":
		// ignore bridge errors
	}
}

func (w *WhatsAppChannel) isAllowedSender(sender string) bool {
	if len(w.config.AllowFrom) == 0 {
		return true
	}

	normalized := normalizeSender(sender)
	for _, allowed := range w.config.AllowFrom {
		if allowed == sender || allowed == normalized {
			return true
		}
	}
	return false
}

func normalizeSender(sender string) string {
	if at := strings.Index(sender, "@"); at >= 0 {
		return sender[:at]
	}
	return sender
}

func (w *WhatsAppChannel) setConnected(connected bool, conn *websocket.Conn) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.connected = connected
	w.conn = conn
}

func (w *WhatsAppChannel) setStatusConnected(connected bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.conn != nil {
		w.connected = connected
	}
}

func (w *WhatsAppChannel) setStatus(status string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.status = status
}

func (w *WhatsAppChannel) setQR(qr string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.lastQR = qr
	w.lastQRAt = time.Now()
}

// WhatsAppStatus 当前状态快照
type WhatsAppStatus struct {
	Enabled   bool      `json:"enabled"`
	Connected bool      `json:"connected"`
	Status    string    `json:"status"`
	LastQR    string    `json:"qr,omitempty"`
	LastQRAt  time.Time `json:"qrAt,omitempty"`
}

// Status 返回 WhatsApp 频道状态（供 Web UI 使用）
func (w *WhatsAppChannel) Status() WhatsAppStatus {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return WhatsAppStatus{
		Enabled:   w.enabled,
		Connected: w.connected,
		Status:    w.status,
		LastQR:    w.lastQR,
		LastQRAt:  w.lastQRAt,
	}
}

func (w *WhatsAppChannel) closeConn() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.conn != nil {
		_ = w.conn.Close()
		w.conn = nil
	}
	w.connected = false
}

func (w *WhatsAppChannel) waitRetry(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-w.stopChan:
		return false
	case <-timer.C:
		return true
	}
}

type bridgeMessage struct {
	Type      string `json:"type"`
	ID        string `json:"id"`
	Sender    string `json:"sender"`
	Content   string `json:"content"`
	Timestamp int64  `json:"timestamp"`
	IsGroup   bool   `json:"isGroup"`
	FromMe    bool   `json:"fromMe"`
	Status    string `json:"status"`
	Error     string `json:"error"`
	QR        string `json:"qr"`
}

type outboundRecord struct {
	chatID string
	text   string
	at     time.Time
}

func (w *WhatsAppChannel) rememberOutbound(chatID, text string) {
	w.outboundMu.Lock()
	defer w.outboundMu.Unlock()

	w.outbound = append(w.outbound, outboundRecord{
		chatID: chatID,
		text:   text,
		at:     time.Now(),
	})
	w.cleanupOutboundLocked()
}

func (w *WhatsAppChannel) isOutboundEcho(chatID, text string) bool {
	w.outboundMu.Lock()
	defer w.outboundMu.Unlock()

	now := time.Now()
	keep := w.outbound[:0]
	for _, rec := range w.outbound {
		if now.Sub(rec.at) <= 45*time.Second {
			keep = append(keep, rec)
		}
	}
	w.outbound = keep

	for _, rec := range w.outbound {
		if rec.chatID == chatID && rec.text == text {
			return true
		}
	}
	return false
}

func (w *WhatsAppChannel) cleanupOutboundLocked() {
	cutoff := time.Now().Add(-45 * time.Second)
	keep := w.outbound[:0]
	for _, rec := range w.outbound {
		if rec.at.After(cutoff) {
			keep = append(keep, rec)
		}
	}
	w.outbound = keep
}
