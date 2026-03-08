package channels

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	qqtoken "github.com/tencent-connect/botgo/token"
	"golang.org/x/oauth2"
)

const (
	qqGatewayURL               = "https://api.sgroup.qq.com/gateway"
	qqIntentGroupAndC2C uint32 = 1 << 25
)

var qqReconnectDelays = []time.Duration{
	1 * time.Second,
	2 * time.Second,
	5 * time.Second,
	10 * time.Second,
	30 * time.Second,
}

// QQConfig QQ 机器人配置（腾讯官方 QQBot）
type QQConfig struct {
	Enabled     bool
	AppID       string
	AppSecret   string
	AccessToken string
	ListenAddr  string
	WebhookPath string
	WSURL       string
	AllowFrom   []string
}

// ResolveQQBotCredentials resolves the official QQ bot credentials from
// explicit app fields or an OpenClaw-compatible `appid:appsecret` token.
func ResolveQQBotCredentials(appID, appSecret, accessToken string) (string, string) {
	resolvedAppID := strings.TrimSpace(appID)
	resolvedSecret := strings.TrimSpace(appSecret)
	tokenValue := strings.TrimSpace(accessToken)

	if tokenValue == "" {
		return resolvedAppID, resolvedSecret
	}

	if parts := strings.SplitN(tokenValue, ":", 2); len(parts) == 2 {
		if resolvedAppID == "" {
			resolvedAppID = strings.TrimSpace(parts[0])
		}
		if resolvedSecret == "" {
			resolvedSecret = strings.TrimSpace(parts[1])
		}
		return resolvedAppID, resolvedSecret
	}

	if resolvedAppID != "" && resolvedSecret == "" {
		resolvedSecret = tokenValue
	}

	return resolvedAppID, resolvedSecret
}

type qqGatewayPayload struct {
	Op int             `json:"op"`
	D  json.RawMessage `json:"d,omitempty"`
	S  *uint32         `json:"s,omitempty"`
	T  string          `json:"t,omitempty"`
	ID string          `json:"id,omitempty"`
}

type qqGatewayHello struct {
	HeartbeatInterval int `json:"heartbeat_interval"`
}

type qqGatewayReady struct {
	SessionID string `json:"session_id"`
}

type qqGatewayInfo struct {
	URL string `json:"url"`
}

type qqC2CAuthor struct {
	ID          string `json:"id"`
	UserOpenID  string `json:"user_openid"`
	UnionOpenID string `json:"union_openid"`
}

type qqC2CMessageEvent struct {
	ID        string      `json:"id"`
	Content   string      `json:"content"`
	Timestamp string      `json:"timestamp"`
	Author    qqC2CAuthor `json:"author"`
}

// QQChannel QQ 机器人频道
type QQChannel struct {
	config         *QQConfig
	messageHandler func(msg *Message)
	httpClient     *http.Client
	stopChan       chan struct{}
	stopOnce       sync.Once
	wg             sync.WaitGroup

	mu             sync.RWMutex
	tokenSource    oauth2.TokenSource
	conn           *websocket.Conn
	lastInboundMsg map[string]string
	replySeq       map[string]uint32
	sessionID      string
	lastSeq        uint32
}

// NewQQChannel 创建 QQ 频道
func NewQQChannel(config *QQConfig) *QQChannel {
	if config == nil {
		config = &QQConfig{}
	}

	return &QQChannel{
		config:         config,
		httpClient:     &http.Client{Timeout: 20 * time.Second},
		stopChan:       make(chan struct{}),
		lastInboundMsg: make(map[string]string),
		replySeq:       make(map[string]uint32),
	}
}

// Name 返回频道名
func (q *QQChannel) Name() string {
	return "qq"
}

// IsEnabled 是否启用
func (q *QQChannel) IsEnabled() bool {
	appID, appSecret := ResolveQQBotCredentials(q.config.AppID, q.config.AppSecret, q.config.AccessToken)
	return q.config.Enabled && appID != "" && appSecret != ""
}

// SetMessageHandler 设置消息处理器
func (q *QQChannel) SetMessageHandler(handler func(msg *Message)) {
	q.messageHandler = handler
}

// Start 启动 QQBot Gateway WebSocket
func (q *QQChannel) Start(ctx context.Context) error {
	if !q.IsEnabled() {
		return nil
	}

	appID, appSecret := ResolveQQBotCredentials(q.config.AppID, q.config.AppSecret, q.config.AccessToken)
	tokenSource := qqtoken.NewQQBotTokenSource(&qqtoken.QQBotCredentials{
		AppID:     appID,
		AppSecret: appSecret,
	})

	if err := qqtoken.StartRefreshAccessToken(ctx, tokenSource); err != nil {
		return fmt.Errorf("start qq token refresh: %w", err)
	}

	q.mu.Lock()
	q.tokenSource = tokenSource
	q.mu.Unlock()

	accessToken, err := q.currentAccessToken()
	if err != nil {
		return err
	}
	if _, err := q.getGatewayURL(ctx, accessToken); err != nil {
		return err
	}

	q.wg.Add(1)
	go func() {
		defer q.wg.Done()
		q.runGatewayLoop(ctx)
	}()

	q.wg.Add(1)
	go func() {
		defer q.wg.Done()
		select {
		case <-ctx.Done():
		case <-q.stopChan:
		}
		q.closeConn()
	}()

	return nil
}

// Stop 停止 QQ 连接
func (q *QQChannel) Stop() error {
	q.stopOnce.Do(func() {
		close(q.stopChan)
	})

	q.closeConn()
	q.wg.Wait()
	return nil
}

// SendMessage 发送 QQ 私聊消息
func (q *QQChannel) SendMessage(chatID string, text string) error {
	if !q.IsEnabled() {
		return fmt.Errorf("qq channel not enabled")
	}

	openID := strings.TrimSpace(chatID)
	content := strings.TrimSpace(text)
	if openID == "" {
		return fmt.Errorf("qq chat id is empty")
	}
	if content == "" {
		return fmt.Errorf("qq message content is empty")
	}

	accessToken, err := q.currentAccessToken()
	if err != nil {
		return err
	}

	q.mu.Lock()
	msgID := q.lastInboundMsg[openID]
	msgSeq := q.nextReplySeqLocked(msgID)
	q.mu.Unlock()

	body := map[string]interface{}{
		"content":  content,
		"msg_type": 0,
	}
	if msgSeq > 0 {
		body["msg_seq"] = msgSeq
	}
	if msgID != "" {
		body["msg_id"] = msgID
	}

	return q.apiJSON(context.Background(), accessToken, http.MethodPost, "/v2/users/"+openID+"/messages", body, nil)
}

func (q *QQChannel) runGatewayLoop(ctx context.Context) {
	attempt := 0
	for {
		if ctx.Err() != nil || q.isStopped() {
			return
		}

		err := q.connectAndServe(ctx)
		if ctx.Err() != nil || q.isStopped() {
			return
		}
		if err == nil {
			attempt = 0
			continue
		}

		delay := qqReconnectDelays[min(attempt, len(qqReconnectDelays)-1)]
		attempt++

		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-q.stopChan:
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func (q *QQChannel) connectAndServe(ctx context.Context) error {
	accessToken, err := q.currentAccessToken()
	if err != nil {
		return err
	}

	gatewayURL, err := q.getGatewayURL(ctx, accessToken)
	if err != nil {
		return err
	}

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, gatewayURL, nil)
	if err != nil {
		return err
	}
	defer conn.Close()

	q.mu.Lock()
	q.conn = conn
	q.mu.Unlock()
	defer q.closeConn()

	heartbeatStop := make(chan struct{})
	defer close(heartbeatStop)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-q.stopChan:
			return nil
		default:
		}

		_, data, err := conn.ReadMessage()
		if err != nil {
			return err
		}

		var payload qqGatewayPayload
		if err := json.Unmarshal(data, &payload); err != nil {
			continue
		}

		if payload.S != nil {
			q.mu.Lock()
			q.lastSeq = *payload.S
			q.mu.Unlock()
		}

		switch payload.Op {
		case 10:
			var hello qqGatewayHello
			if err := json.Unmarshal(payload.D, &hello); err != nil {
				return err
			}
			if err := q.startHeartbeat(conn, time.Duration(hello.HeartbeatInterval)*time.Millisecond, heartbeatStop); err != nil {
				return err
			}
			if err := q.sendIdentifyOrResume(conn, accessToken); err != nil {
				return err
			}
		case 0:
			switch payload.T {
			case "READY":
				var ready qqGatewayReady
				if err := json.Unmarshal(payload.D, &ready); err == nil {
					q.mu.Lock()
					q.sessionID = strings.TrimSpace(ready.SessionID)
					q.mu.Unlock()
				}
			case "RESUMED":
				// Session resumed successfully.
			case "C2C_MESSAGE_CREATE":
				var event qqC2CMessageEvent
				if err := json.Unmarshal(payload.D, &event); err != nil {
					continue
				}
				q.handleC2CMessage(&event)
			}
		case 7:
			return fmt.Errorf("qq gateway requested reconnect")
		case 9:
			q.mu.Lock()
			q.sessionID = ""
			q.lastSeq = 0
			q.mu.Unlock()
			return fmt.Errorf("qq gateway invalid session")
		}
	}
}

func (q *QQChannel) sendIdentifyOrResume(conn *websocket.Conn, accessToken string) error {
	q.mu.RLock()
	sessionID := q.sessionID
	lastSeq := q.lastSeq
	q.mu.RUnlock()

	if sessionID != "" && lastSeq != 0 {
		return conn.WriteJSON(map[string]interface{}{
			"op": 6,
			"d": map[string]interface{}{
				"token":      "QQBot " + accessToken,
				"session_id": sessionID,
				"seq":        lastSeq,
			},
		})
	}

	return conn.WriteJSON(map[string]interface{}{
		"op": 2,
		"d": map[string]interface{}{
			"token":   "QQBot " + accessToken,
			"intents": qqIntentGroupAndC2C,
			"shard":   []uint32{0, 1},
		},
	})
}

func (q *QQChannel) startHeartbeat(conn *websocket.Conn, interval time.Duration, stop <-chan struct{}) error {
	if interval <= 0 {
		return fmt.Errorf("qq heartbeat interval is invalid")
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				q.mu.RLock()
				lastSeq := q.lastSeq
				q.mu.RUnlock()
				_ = conn.WriteJSON(map[string]interface{}{
					"op": 1,
					"d":  lastSeq,
				})
			case <-stop:
				return
			case <-q.stopChan:
				return
			}
		}
	}()

	return nil
}

func (q *QQChannel) handleC2CMessage(event *qqC2CMessageEvent) {
	if event == nil || q.messageHandler == nil {
		return
	}

	sender := strings.TrimSpace(event.Author.UserOpenID)
	if sender == "" {
		sender = strings.TrimSpace(event.Author.ID)
	}
	if sender == "" {
		sender = strings.TrimSpace(event.Author.UnionOpenID)
	}
	if sender == "" {
		return
	}

	if !q.isAllowed(sender, event.Author.ID, event.Author.UnionOpenID) {
		return
	}

	text := strings.TrimSpace(event.Content)
	if text == "" {
		return
	}

	q.mu.Lock()
	q.lastInboundMsg[sender] = strings.TrimSpace(event.ID)
	q.mu.Unlock()

	q.messageHandler(&Message{
		ID:      strings.TrimSpace(event.ID),
		Text:    text,
		Sender:  sender,
		ChatID:  sender,
		Channel: "qq",
		Raw:     event,
	})
}

func (q *QQChannel) currentAccessToken() (string, error) {
	q.mu.RLock()
	tokenSource := q.tokenSource
	q.mu.RUnlock()
	if tokenSource == nil {
		return "", fmt.Errorf("qq token source not initialized")
	}

	token, err := tokenSource.Token()
	if err != nil {
		return "", fmt.Errorf("get qq access token: %w", err)
	}
	if strings.TrimSpace(token.AccessToken) == "" {
		return "", fmt.Errorf("qq access token is empty")
	}
	return token.AccessToken, nil
}

func (q *QQChannel) getGatewayURL(ctx context.Context, accessToken string) (string, error) {
	var info qqGatewayInfo
	if err := q.apiJSON(ctx, accessToken, http.MethodGet, "/gateway", nil, &info); err != nil {
		return "", err
	}
	if strings.TrimSpace(info.URL) == "" {
		return "", fmt.Errorf("qq gateway url is empty")
	}
	return info.URL, nil
}

func (q *QQChannel) apiJSON(ctx context.Context, accessToken, method, path string, body interface{}, out interface{}) error {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, "https://api.sgroup.qq.com"+path, reqBody)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "QQBot "+accessToken)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := q.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("qq api request failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	if out == nil || len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, out)
}

func (q *QQChannel) nextReplySeqLocked(msgID string) uint32 {
	if msgID == "" {
		return 1
	}
	q.replySeq[msgID]++
	if q.replySeq[msgID] == 0 {
		q.replySeq[msgID] = 1
	}
	return q.replySeq[msgID]
}

func (q *QQChannel) closeConn() {
	q.mu.Lock()
	conn := q.conn
	q.conn = nil
	q.mu.Unlock()

	if conn != nil {
		_ = conn.Close()
	}
}

func (q *QQChannel) isStopped() bool {
	select {
	case <-q.stopChan:
		return true
	default:
		return false
	}
}

func (q *QQChannel) isAllowed(identifiers ...string) bool {
	allow := make([]string, 0, len(q.config.AllowFrom))
	for _, entry := range q.config.AllowFrom {
		entry = strings.TrimSpace(entry)
		if entry != "" {
			allow = append(allow, entry)
		}
	}
	if len(allow) == 0 {
		return true
	}

	for _, candidate := range identifiers {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		for _, entry := range allow {
			if strings.EqualFold(entry, candidate) {
				return true
			}
		}
	}

	// Official QQBot C2C events identify users by OpenID rather than the raw QQ number
	// shown in the Tencent console. When the allowlist only contains numeric QQ IDs, we
	// cannot enforce it reliably, so treat it as legacy advisory config instead of
	// dropping the message entirely.
	if allNumericStrings(allow) {
		return true
	}

	return false
}

func allNumericStrings(values []string) bool {
	if len(values) == 0 {
		return false
	}
	for _, value := range values {
		for _, r := range value {
			if r < '0' || r > '9' {
				return false
			}
		}
	}
	return true
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
