package channels

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Lichas/maxclaw/internal/logging"
)

// TelegramConfig Telegram 配置
type TelegramConfig struct {
	Token     string   `json:"token"`
	Enabled   bool     `json:"enabled"`
	AllowFrom []string `json:"allowFrom,omitempty"`
	Proxy     string   `json:"proxy,omitempty"`
}

// TelegramChannel Telegram 频道
type TelegramChannel struct {
	config         *TelegramConfig
	httpClient     *http.Client
	messageHandler func(msg *Message)
	stopChan       chan struct{}
	wg             sync.WaitGroup
	offset         int64
	enabled        bool
	mu             sync.RWMutex
	status         string
	botUsername    string
	botName        string
	lastError      string
}

// NewTelegramChannel 创建 Telegram 频道
func NewTelegramChannel(config *TelegramConfig) *TelegramChannel {
	transport := &http.Transport{Proxy: http.ProxyFromEnvironment}
	if proxy := strings.TrimSpace(config.Proxy); proxy != "" {
		if parsed, err := url.Parse(proxy); err == nil {
			transport.Proxy = http.ProxyURL(parsed)
		}
	}

	return &TelegramChannel{
		config: config,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
		},
		stopChan: make(chan struct{}),
		offset:   0,
		enabled:  config.Enabled && config.Token != "",
	}
}

// Name 返回频道名称
func (t *TelegramChannel) Name() string {
	return "telegram"
}

// IsEnabled 是否启用
func (t *TelegramChannel) IsEnabled() bool {
	return t.enabled
}

// SetMessageHandler 设置消息处理器
func (t *TelegramChannel) SetMessageHandler(handler func(msg *Message)) {
	t.messageHandler = handler
}

// Start 启动 Telegram 频道
func (t *TelegramChannel) Start(ctx context.Context) error {
	if !t.enabled {
		return nil
	}

	t.refreshBotInfo()

	t.wg.Add(1)
	go t.pollUpdates(ctx)

	return nil
}

// Stop 停止 Telegram 频道
func (t *TelegramChannel) Stop() error {
	if !t.enabled {
		return nil
	}

	close(t.stopChan)
	t.wg.Wait()
	return nil
}

// pollUpdates 轮询更新
func (t *TelegramChannel) pollUpdates(ctx context.Context) {
	defer t.wg.Done()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.stopChan:
			return
		case <-ticker.C:
			t.fetchUpdates()
		}
	}
}

// fetchUpdates 获取更新
func (t *TelegramChannel) fetchUpdates() {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates", t.config.Token)

	params := url.Values{}
	params.Set("offset", strconv.FormatInt(t.offset+1, 10))
	params.Set("limit", "100")

	resp, err := t.httpClient.Get(apiURL + "?" + params.Encode())
	if err != nil {
		st := t.Status()
		t.setStatus("error", st.Username, st.Name, err.Error())
		if lg := logging.Get(); lg != nil && lg.Channels != nil {
			lg.Channels.Printf("telegram getUpdates error: %v", err)
		}
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		st := t.Status()
		t.setStatus("error", st.Username, st.Name, err.Error())
		if lg := logging.Get(); lg != nil && lg.Channels != nil {
			lg.Channels.Printf("telegram readUpdates error: %v", err)
		}
		return
	}

	var result struct {
		OK     bool `json:"ok"`
		Result []struct {
			UpdateID int64 `json:"update_id"`
			Message  struct {
				MessageID int64 `json:"message_id"`
				From      struct {
					ID       int64  `json:"id"`
					Username string `json:"username"`
				} `json:"from"`
				Chat struct {
					ID   int64  `json:"id"`
					Type string `json:"type"`
				} `json:"chat"`
				Text string `json:"text"`
				Date int64  `json:"date"`
			} `json:"message"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		st := t.Status()
		t.setStatus("error", st.Username, st.Name, err.Error())
		if lg := logging.Get(); lg != nil && lg.Channels != nil {
			lg.Channels.Printf("telegram parseUpdates error: %v", err)
		}
		return
	}

	if !result.OK {
		st := t.Status()
		t.setStatus("error", st.Username, st.Name, "getUpdates returned not ok")
		if lg := logging.Get(); lg != nil && lg.Channels != nil {
			lg.Channels.Printf("telegram getUpdates not ok")
		}
		return
	}

	st := t.Status()
	if st.Status != "ready" {
		t.setStatus("ready", st.Username, st.Name, "")
	}

	for _, update := range result.Result {
		if update.UpdateID > t.offset {
			t.offset = update.UpdateID
		}

		if update.Message.Text != "" && t.messageHandler != nil {
			if !t.isAllowed(update.Message.From.ID, update.Message.From.Username) {
				continue
			}
			sender := update.Message.From.Username
			if strings.TrimSpace(sender) == "" {
				sender = strconv.FormatInt(update.Message.From.ID, 10)
			}
			msg := &Message{
				ID:      strconv.FormatInt(update.Message.MessageID, 10),
				Text:    update.Message.Text,
				Sender:  sender,
				ChatID:  strconv.FormatInt(update.Message.Chat.ID, 10),
				Channel: "telegram",
				Raw:     update,
			}
			t.messageHandler(msg)
			if lg := logging.Get(); lg != nil && lg.Channels != nil {
				lg.Channels.Printf("telegram inbound chat=%s sender=%s text=%q", msg.ChatID, msg.Sender, logging.Truncate(msg.Text, 300))
			}
		}
	}
}

func (t *TelegramChannel) isAllowed(userID int64, username string) bool {
	if len(t.config.AllowFrom) == 0 {
		return true
	}

	idStr := strconv.FormatInt(userID, 10)
	username = strings.TrimPrefix(strings.TrimSpace(username), "@")

	for _, allowed := range t.config.AllowFrom {
		allowed = strings.TrimSpace(allowed)
		if allowed == "" {
			continue
		}
		if allowed == idStr {
			return true
		}
		if strings.TrimPrefix(allowed, "@") == username && username != "" {
			return true
		}
	}
	return false
}

// SendMessage 发送消息
func (t *TelegramChannel) SendMessage(chatID string, text string) error {
	if !t.enabled {
		return fmt.Errorf("telegram channel not enabled")
	}

	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.config.Token)

	params := url.Values{}
	params.Set("chat_id", chatID)
	params.Set("text", text)
	params.Set("parse_mode", "HTML")

	resp, err := t.httpClient.Post(
		apiURL,
		"application/x-www-form-urlencoded",
		strings.NewReader(params.Encode()),
	)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram API error: %s", string(body))
	}

	if lg := logging.Get(); lg != nil && lg.Channels != nil {
		lg.Channels.Printf("telegram send chat=%s text=%q", chatID, logging.Truncate(text, 300))
	}
	return nil
}

// TelegramStatus 当前状态快照
type TelegramStatus struct {
	Enabled  bool   `json:"enabled"`
	Status   string `json:"status,omitempty"`
	Username string `json:"username,omitempty"`
	Name     string `json:"name,omitempty"`
	Link     string `json:"link,omitempty"`
	Error    string `json:"error,omitempty"`
}

// Status 返回 Telegram 频道状态（供 Web UI 使用）
func (t *TelegramChannel) Status() TelegramStatus {
	t.mu.RLock()
	defer t.mu.RUnlock()

	link := ""
	if t.botUsername != "" {
		link = fmt.Sprintf("https://t.me/%s", t.botUsername)
	}

	return TelegramStatus{
		Enabled:  t.enabled,
		Status:   t.status,
		Username: t.botUsername,
		Name:     t.botName,
		Link:     link,
		Error:    t.lastError,
	}
}

func (t *TelegramChannel) refreshBotInfo() {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/getMe", t.config.Token)

	resp, err := t.httpClient.Get(apiURL)
	if err != nil {
		t.setStatus("error", "", "", err.Error())
		return
	}
	defer resp.Body.Close()

	var result struct {
		OK     bool `json:"ok"`
		Result struct {
			ID        int64  `json:"id"`
			Username  string `json:"username"`
			FirstName string `json:"first_name"`
		} `json:"result"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.setStatus("error", "", "", err.Error())
		return
	}
	if !result.OK {
		if result.Description == "" {
			result.Description = "invalid token"
		}
		t.setStatus("error", "", "", result.Description)
		return
	}

	t.setStatus("ready", result.Result.Username, result.Result.FirstName, "")
}

func (t *TelegramChannel) setStatus(status, username, name, errMsg string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.status = status
	t.botUsername = username
	t.botName = name
	t.lastError = errMsg
}
