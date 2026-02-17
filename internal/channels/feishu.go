package channels

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// FeishuConfig Feishu/Lark 配置
type FeishuConfig struct {
	Enabled           bool
	AppID             string
	AppSecret         string
	VerificationToken string
	ListenAddr        string
	WebhookPath       string
	AllowFrom         []string
}

// FeishuChannel Feishu 频道
type FeishuChannel struct {
	config         *FeishuConfig
	messageHandler func(msg *Message)

	httpClient *http.Client
	server     *http.Server

	stopChan    chan struct{}
	stopOnce    sync.Once
	wg          sync.WaitGroup
	mu          sync.RWMutex
	token       string
	tokenExpire time.Time
}

// NewFeishuChannel 创建 Feishu 频道
func NewFeishuChannel(config *FeishuConfig) *FeishuChannel {
	if config == nil {
		config = &FeishuConfig{}
	}
	if strings.TrimSpace(config.ListenAddr) == "" {
		config.ListenAddr = "0.0.0.0:18792"
	}
	if strings.TrimSpace(config.WebhookPath) == "" {
		config.WebhookPath = "/feishu/events"
	}

	return &FeishuChannel{
		config: config,
		httpClient: &http.Client{
			Timeout: 20 * time.Second,
		},
		stopChan: make(chan struct{}),
	}
}

// Name 返回频道名称
func (f *FeishuChannel) Name() string {
	return "feishu"
}

// IsEnabled 是否启用
func (f *FeishuChannel) IsEnabled() bool {
	return f.config.Enabled &&
		strings.TrimSpace(f.config.AppID) != "" &&
		strings.TrimSpace(f.config.AppSecret) != ""
}

// SetMessageHandler 设置消息处理器
func (f *FeishuChannel) SetMessageHandler(handler func(msg *Message)) {
	f.messageHandler = handler
}

// Start 启动 Feishu webhook 监听
func (f *FeishuChannel) Start(ctx context.Context) error {
	if !f.IsEnabled() {
		return nil
	}

	mux := http.NewServeMux()
	mux.HandleFunc(f.config.WebhookPath, f.handleWebhook)

	srv := &http.Server{
		Addr:    f.config.ListenAddr,
		Handler: mux,
	}

	f.mu.Lock()
	f.server = srv
	f.mu.Unlock()

	f.wg.Add(1)
	go func() {
		defer f.wg.Done()
		err := srv.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			// keep silent: gateway logs startup errors in caller
		}
	}()

	f.wg.Add(1)
	go func() {
		defer f.wg.Done()
		select {
		case <-ctx.Done():
		case <-f.stopChan:
		}
		_ = f.shutdownServer()
	}()

	return nil
}

// Stop 停止 webhook 服务
func (f *FeishuChannel) Stop() error {
	f.stopOnce.Do(func() {
		close(f.stopChan)
	})

	err := f.shutdownServer()
	f.wg.Wait()
	return err
}

func (f *FeishuChannel) shutdownServer() error {
	f.mu.Lock()
	srv := f.server
	f.server = nil
	f.mu.Unlock()

	if srv == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return srv.Shutdown(ctx)
}

// SendMessage 发送 Feishu 文本消息
func (f *FeishuChannel) SendMessage(chatID string, text string) error {
	if !f.IsEnabled() {
		return fmt.Errorf("feishu channel not enabled")
	}

	token, err := f.getTenantToken(context.Background())
	if err != nil {
		return err
	}

	contentBytes, _ := json.Marshal(map[string]string{"text": text})
	reqBody := map[string]string{
		"receive_id": chatID,
		"msg_type":   "text",
		"content":    string(contentBytes),
	}
	data, _ := json.Marshal(reqBody)

	req, err := http.NewRequest(http.MethodPost, "https://open.feishu.cn/open-apis/im/v1/messages?receive_id_type=open_id", bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("feishu send failed: status=%d body=%s", resp.StatusCode, string(body))
	}
	return nil
}

func (f *FeishuChannel) handleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	raw, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var evt feishuEventEnvelope
	if err := json.Unmarshal(raw, &evt); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// URL verification challenge
	if evt.Challenge != "" {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"challenge": evt.Challenge})
		return
	}

	if token := strings.TrimSpace(f.config.VerificationToken); token != "" {
		if strings.TrimSpace(evt.Header.Token) != token {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
	}

	if evt.Header.EventType != "im.message.receive_v1" {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
		return
	}

	sender := strings.TrimSpace(evt.Event.Sender.SenderID.OpenID)
	if sender == "" || !f.allowedSender(sender) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
		return
	}

	text := strings.TrimSpace(parseFeishuText(evt.Event.Message.MessageType, evt.Event.Message.Content))
	if text != "" && f.messageHandler != nil {
		f.messageHandler(&Message{
			ID:      evt.Event.Message.MessageID,
			Text:    text,
			Sender:  sender,
			ChatID:  sender,
			Channel: "feishu",
			Raw:     evt,
		})
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"ok":true}`))
}

func (f *FeishuChannel) allowedSender(sender string) bool {
	if len(f.config.AllowFrom) == 0 {
		return true
	}
	for _, v := range f.config.AllowFrom {
		if strings.TrimSpace(v) == sender {
			return true
		}
	}
	return false
}

func parseFeishuText(messageType, rawContent string) string {
	if strings.TrimSpace(rawContent) == "" {
		return ""
	}
	if messageType != "text" {
		return "[" + messageType + "]"
	}

	var content struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(rawContent), &content); err != nil {
		return rawContent
	}
	return content.Text
}

func (f *FeishuChannel) getTenantToken(ctx context.Context) (string, error) {
	f.mu.RLock()
	if f.token != "" && time.Now().Before(f.tokenExpire) {
		token := f.token
		f.mu.RUnlock()
		return token, nil
	}
	f.mu.RUnlock()

	reqBody := map[string]string{
		"app_id":     f.config.AppID,
		"app_secret": f.config.AppSecret,
	}
	data, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal", bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("get feishu token failed: status=%d body=%s", resp.StatusCode, string(body))
	}

	var out struct {
		Code              int    `json:"code"`
		Msg               string `json:"msg"`
		TenantAccessToken string `json:"tenant_access_token"`
		Expire            int64  `json:"expire"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", err
	}
	if out.Code != 0 || out.TenantAccessToken == "" {
		return "", fmt.Errorf("get feishu token failed: code=%d msg=%s", out.Code, out.Msg)
	}

	expireIn := out.Expire
	if expireIn <= 60 {
		expireIn = 120
	}

	f.mu.Lock()
	f.token = out.TenantAccessToken
	f.tokenExpire = time.Now().Add(time.Duration(expireIn-60) * time.Second)
	f.mu.Unlock()

	return out.TenantAccessToken, nil
}

type feishuEventEnvelope struct {
	Challenge string `json:"challenge"`
	Header    struct {
		EventType string `json:"event_type"`
		Token     string `json:"token"`
	} `json:"header"`
	Event struct {
		Sender struct {
			SenderID struct {
				OpenID string `json:"open_id"`
			} `json:"sender_id"`
		} `json:"sender"`
		Message struct {
			MessageID   string `json:"message_id"`
			MessageType string `json:"message_type"`
			Content     string `json:"content"`
		} `json:"message"`
	} `json:"event"`
}
