package channels

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/tencent-connect/botgo"
	"github.com/tencent-connect/botgo/dto"
	"github.com/tencent-connect/botgo/event"
	qqwebhook "github.com/tencent-connect/botgo/interaction/webhook"
	qqtoken "github.com/tencent-connect/botgo/token"
)

const (
	defaultQQListenAddr  = "0.0.0.0:18793"
	defaultQQWebhookPath = "/qq/events"
)

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

type qqSenderFunc func(ctx context.Context, userID string, msg dto.APIMessage) error

// QQChannel QQ 机器人频道
type QQChannel struct {
	config         *QQConfig
	messageHandler func(msg *Message)
	httpClient     *http.Client

	server   *http.Server
	postC2C  qqSenderFunc
	stopChan chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup

	mu             sync.RWMutex
	lastInboundMsg map[string]string
}

// NewQQChannel 创建 QQ 频道
func NewQQChannel(config *QQConfig) *QQChannel {
	if config == nil {
		config = &QQConfig{}
	}
	if strings.TrimSpace(config.ListenAddr) == "" {
		config.ListenAddr = defaultQQListenAddr
	}
	if strings.TrimSpace(config.WebhookPath) == "" {
		config.WebhookPath = defaultQQWebhookPath
	}
	if !strings.HasPrefix(config.WebhookPath, "/") {
		config.WebhookPath = "/" + config.WebhookPath
	}

	return &QQChannel{
		config:         config,
		httpClient:     &http.Client{Timeout: 20 * time.Second},
		stopChan:       make(chan struct{}),
		lastInboundMsg: make(map[string]string),
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

// Start 启动 QQBot webhook 监听
func (q *QQChannel) Start(ctx context.Context) error {
	if !q.IsEnabled() {
		return nil
	}

	appID, appSecret := ResolveQQBotCredentials(q.config.AppID, q.config.AppSecret, q.config.AccessToken)
	credentials := &qqtoken.QQBotCredentials{
		AppID:     appID,
		AppSecret: appSecret,
	}

	tokenSource := qqtoken.NewQQBotTokenSource(credentials)
	if err := qqtoken.StartRefreshAccessToken(ctx, tokenSource); err != nil {
		return fmt.Errorf("start qq token refresh: %w", err)
	}

	api := botgo.NewOpenAPI(appID, tokenSource).WithTimeout(10 * time.Second)

	q.mu.Lock()
	q.postC2C = func(ctx context.Context, userID string, msg dto.APIMessage) error {
		_, err := api.PostC2CMessage(ctx, userID, msg)
		return err
	}
	q.mu.Unlock()

	event.RegisterHandlers(q.handleOfficialC2CMessage())

	mux := http.NewServeMux()
	mux.HandleFunc(q.config.WebhookPath, func(w http.ResponseWriter, r *http.Request) {
		qqwebhook.HTTPHandler(w, r, credentials)
	})

	srv := &http.Server{
		Addr:    q.config.ListenAddr,
		Handler: mux,
	}

	q.mu.Lock()
	q.server = srv
	q.mu.Unlock()

	q.wg.Add(1)
	go func() {
		defer q.wg.Done()
		err := srv.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			// Keep silent here; gateway startup already reports channel start errors.
		}
	}()

	q.wg.Add(1)
	go func() {
		defer q.wg.Done()
		select {
		case <-ctx.Done():
		case <-q.stopChan:
		}
		_ = q.shutdownServer()
	}()

	return nil
}

// Stop 停止 QQBot webhook 服务
func (q *QQChannel) Stop() error {
	q.stopOnce.Do(func() {
		close(q.stopChan)
	})

	err := q.shutdownServer()
	q.wg.Wait()
	return err
}

func (q *QQChannel) shutdownServer() error {
	q.mu.Lock()
	srv := q.server
	q.server = nil
	q.postC2C = nil
	q.mu.Unlock()

	if srv == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return srv.Shutdown(ctx)
}

// SendMessage 发送 QQ 私聊消息
func (q *QQChannel) SendMessage(chatID string, text string) error {
	if !q.IsEnabled() {
		return fmt.Errorf("qq channel not enabled")
	}

	q.mu.RLock()
	send := q.postC2C
	msgID := q.lastInboundMsg[strings.TrimSpace(chatID)]
	q.mu.RUnlock()

	if send == nil {
		return fmt.Errorf("qq channel not connected")
	}

	msg := &dto.MessageToCreate{
		Content: strings.TrimSpace(text),
		MsgType: dto.TextMsg,
		MsgID:   msgID,
	}
	if msg.Content == "" {
		return fmt.Errorf("qq message content is empty")
	}

	return send(context.Background(), strings.TrimSpace(chatID), msg)
}

func (q *QQChannel) handleOfficialC2CMessage() event.C2CMessageEventHandler {
	return func(_ *dto.WSPayload, data *dto.WSC2CMessageData) error {
		if data == nil {
			return nil
		}

		sender := ""
		if data.Author != nil {
			sender = strings.TrimSpace(data.Author.ID)
		}
		if sender == "" || !q.isAllowed(sender) {
			return nil
		}

		text := strings.TrimSpace(data.Content)
		if text == "" || q.messageHandler == nil {
			return nil
		}

		q.mu.Lock()
		q.lastInboundMsg[sender] = strings.TrimSpace(data.ID)
		q.mu.Unlock()

		q.messageHandler(&Message{
			ID:      strings.TrimSpace(data.ID),
			Text:    text,
			Sender:  sender,
			ChatID:  sender,
			Channel: "qq",
			Raw:     data,
		})
		return nil
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
