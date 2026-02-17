package channels

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

// SlackConfig Slack Socket Mode 配置
type SlackConfig struct {
	Enabled   bool
	BotToken  string
	AppToken  string
	AllowFrom []string
}

// SlackChannel Slack 频道
type SlackChannel struct {
	config         *SlackConfig
	messageHandler func(msg *Message)

	client       *slack.Client
	socketClient *socketmode.Client
	botUserID    string
	stopChan     chan struct{}
	stopOnce     sync.Once
	wg           sync.WaitGroup
	mu           sync.RWMutex
}

// NewSlackChannel 创建 Slack 频道
func NewSlackChannel(config *SlackConfig) *SlackChannel {
	if config == nil {
		config = &SlackConfig{}
	}
	return &SlackChannel{
		config:   config,
		stopChan: make(chan struct{}),
	}
}

// Name 返回频道名称
func (s *SlackChannel) Name() string {
	return "slack"
}

// IsEnabled 是否启用
func (s *SlackChannel) IsEnabled() bool {
	return s.config.Enabled && strings.TrimSpace(s.config.BotToken) != "" && strings.TrimSpace(s.config.AppToken) != ""
}

// SetMessageHandler 设置消息处理器
func (s *SlackChannel) SetMessageHandler(handler func(msg *Message)) {
	s.messageHandler = handler
}

// Start 启动 Slack Socket Mode
func (s *SlackChannel) Start(ctx context.Context) error {
	if !s.IsEnabled() {
		return nil
	}

	s.mu.Lock()
	s.client = slack.New(s.config.BotToken, slack.OptionAppLevelToken(s.config.AppToken))
	s.socketClient = socketmode.New(s.client)
	s.mu.Unlock()

	auth, err := s.client.AuthTest()
	if err == nil {
		s.botUserID = auth.UserID
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.socketClient.RunContext(ctx)
	}()

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case <-s.stopChan:
				return
			case evt, ok := <-s.socketClient.Events:
				if !ok {
					return
				}
				s.handleEvent(evt)
			}
		}
	}()

	return nil
}

// Stop 停止 Slack 连接
func (s *SlackChannel) Stop() error {
	s.stopOnce.Do(func() {
		close(s.stopChan)
	})

	s.mu.Lock()
	s.socketClient = nil
	s.mu.Unlock()

	s.wg.Wait()
	return nil
}

// SendMessage 发送消息
func (s *SlackChannel) SendMessage(chatID string, text string) error {
	if !s.IsEnabled() {
		return fmt.Errorf("slack channel not enabled")
	}
	s.mu.RLock()
	client := s.client
	s.mu.RUnlock()
	if client == nil {
		return fmt.Errorf("slack channel not started")
	}
	_, _, err := client.PostMessage(chatID, slack.MsgOptionText(text, false))
	return err
}

func (s *SlackChannel) handleEvent(evt socketmode.Event) {
	if evt.Type != socketmode.EventTypeEventsAPI {
		return
	}

	if evt.Request != nil && s.socketClient != nil {
		s.socketClient.Ack(*evt.Request)
	}

	eventsAPIEvent, ok := evt.Data.(slackevents.EventsAPIEvent)
	if !ok || eventsAPIEvent.Type != slackevents.CallbackEvent {
		return
	}

	inner := eventsAPIEvent.InnerEvent
	msgEvt, ok := inner.Data.(*slackevents.MessageEvent)
	if !ok {
		return
	}
	if msgEvt.SubType != "" || msgEvt.BotID != "" {
		return
	}
	if s.botUserID != "" && msgEvt.User == s.botUserID {
		return
	}

	if !s.isAllowed(msgEvt.User) {
		return
	}

	text := strings.TrimSpace(msgEvt.Text)
	if text == "" || s.messageHandler == nil {
		return
	}

	s.messageHandler(&Message{
		ID:      msgEvt.EventTimeStamp,
		Text:    text,
		Sender:  msgEvt.User,
		ChatID:  msgEvt.Channel,
		Channel: "slack",
		Raw:     msgEvt,
	})
}

func (s *SlackChannel) isAllowed(sender string) bool {
	if len(s.config.AllowFrom) == 0 {
		return true
	}
	for _, v := range s.config.AllowFrom {
		if strings.TrimSpace(v) == sender {
			return true
		}
	}
	return false
}
