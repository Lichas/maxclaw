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

	"github.com/Lichas/maxclaw/internal/logging"
	"github.com/bwmarrin/discordgo"
)

// DiscordConfig Discord 配置
type DiscordConfig struct {
	Token     string   `json:"token"`
	Enabled   bool     `json:"enabled"`
	AllowFrom []string `json:"allowFrom"`
}

// DiscordChannel Discord 频道
type DiscordChannel struct {
	config         *DiscordConfig
	httpClient     *http.Client
	messageHandler func(msg *Message)
	stopChan       chan struct{}
	wg             sync.WaitGroup
	enabled        bool

	session *discordgo.Session
}

// NewDiscordChannel 创建 Discord 频道
func NewDiscordChannel(config *DiscordConfig) *DiscordChannel {
	return &DiscordChannel{
		config: config,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		stopChan: make(chan struct{}),
		enabled:  config.Enabled && config.Token != "",
	}
}

// Name 返回频道名称
func (d *DiscordChannel) Name() string {
	return "discord"
}

// IsEnabled 是否启用
func (d *DiscordChannel) IsEnabled() bool {
	return d.enabled
}

// SetMessageHandler 设置消息处理器
func (d *DiscordChannel) SetMessageHandler(handler func(msg *Message)) {
	d.messageHandler = handler
}

// Start 启动 Discord 频道
func (d *DiscordChannel) Start(ctx context.Context) error {
	if !d.enabled {
		return nil
	}

	dg, err := discordgo.New("Bot " + d.config.Token)
	if err != nil {
		return err
	}

	dg.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentsDirectMessages | discordgo.IntentsMessageContent

	dg.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		d.handleMessage(s, m)
	})

	if err := dg.Open(); err != nil {
		return err
	}

	d.session = dg

	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		select {
		case <-ctx.Done():
		case <-d.stopChan:
		}
		_ = d.Stop()
	}()

	return nil
}

// Stop 停止 Discord 频道
func (d *DiscordChannel) Stop() error {
	if !d.enabled {
		return nil
	}

	select {
	case <-d.stopChan:
		// already closed
	default:
		close(d.stopChan)
	}

	if d.session != nil {
		err := d.session.Close()
		d.session = nil
		return err
	}

	d.wg.Wait()
	return nil
}

// SendMessage 发送消息到 Discord 频道
func (d *DiscordChannel) SendMessage(channelID string, text string) error {
	if !d.enabled {
		return fmt.Errorf("discord channel not enabled")
	}
	if d.session == nil {
		return fmt.Errorf("discord session not started")
	}
	_, err := d.session.ChannelMessageSend(channelID, text)
	if err != nil {
		if lg := logging.Get(); lg != nil && lg.Channels != nil {
			lg.Channels.Printf("discord send error chat=%s err=%v", channelID, err)
		}
		return err
	}
	if lg := logging.Get(); lg != nil && lg.Channels != nil {
		lg.Channels.Printf("discord send chat=%s text=%q", channelID, logging.Truncate(text, 300))
	}
	return nil
}

// SendWebhookMessage 通过 Webhook 发送消息
func (d *DiscordChannel) SendWebhookMessage(webhookURL string, text string) error {
	if webhookURL == "" {
		return fmt.Errorf("webhook URL is required")
	}

	payload := map[string]string{
		"content": text,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := d.httpClient.Post(
		webhookURL,
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("discord webhook error (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// ParseMention 解析 Discord 提及
func ParseMention(userID string) string {
	return fmt.Sprintf("<@%s>", userID)
}

// ParseChannel 解析 Discord 频道
func ParseChannel(channelID string) string {
	return fmt.Sprintf("<#%s>", channelID)
}

// EscapeMarkdown 转义 Discord Markdown
func EscapeMarkdown(text string) string {
	// 转义特殊字符
	special := []string{"*", "_", "~", "`", "|", ">"}
	result := text
	for _, char := range special {
		result = strings.ReplaceAll(result, "\\"+char, "\\\\"+char)
		result = strings.ReplaceAll(result, char, "\\"+char)
	}
	return result
}

func (d *DiscordChannel) handleMessage(_ *discordgo.Session, m *discordgo.MessageCreate) {
	if !d.enabled || d.messageHandler == nil {
		return
	}

	if m.Author == nil || m.Author.Bot {
		return
	}

	if !d.isAllowed(m.Author) {
		return
	}

	msg := &Message{
		ID:      m.ID,
		Text:    m.Content,
		Sender:  d.authorLabel(m.Author),
		ChatID:  m.ChannelID,
		Channel: "discord",
		Raw:     m,
	}
	if msg.Text == "" {
		return
	}
	d.messageHandler(msg)
	if lg := logging.Get(); lg != nil && lg.Channels != nil {
		lg.Channels.Printf("discord inbound chat=%s sender=%s text=%q", msg.ChatID, msg.Sender, logging.Truncate(msg.Text, 300))
	}
}

func (d *DiscordChannel) isAllowed(author *discordgo.User) bool {
	if len(d.config.AllowFrom) == 0 {
		return true
	}

	label := d.authorLabel(author)
	for _, allowed := range d.config.AllowFrom {
		if allowed == author.ID || allowed == author.Username || allowed == label {
			return true
		}
	}
	return false
}

func (d *DiscordChannel) authorLabel(author *discordgo.User) string {
	if author == nil {
		return ""
	}
	if author.Discriminator != "" && author.Discriminator != "0" {
		return fmt.Sprintf("%s#%s", author.Username, author.Discriminator)
	}
	return author.Username
}
