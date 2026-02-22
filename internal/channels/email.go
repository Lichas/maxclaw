package channels

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/emersion/go-imap"
	imapclient "github.com/emersion/go-imap/client"
)

// EmailConfig Email(IMAP/SMTP) 配置
type EmailConfig struct {
	Enabled             bool
	ConsentGranted      bool
	IMAPHost            string
	IMAPPort            int
	IMAPUsername        string
	IMAPPassword        string
	IMAPMailbox         string
	IMAPUseSSL          bool
	SMTPHost            string
	SMTPPort            int
	SMTPUsername        string
	SMTPPassword        string
	SMTPUseTLS          bool
	SMTPUseSSL          bool
	FromAddress         string
	AutoReplyEnabled    bool
	PollIntervalSeconds int
	MarkSeen            bool
	AllowFrom           []string
}

// EmailChannel Email 频道
type EmailChannel struct {
	config         *EmailConfig
	messageHandler func(msg *Message)
	stopChan       chan struct{}
	stopOnce       sync.Once
	wg             sync.WaitGroup
}

// NewEmailChannel 创建 Email 频道
func NewEmailChannel(config *EmailConfig) *EmailChannel {
	if config == nil {
		config = &EmailConfig{}
	}
	if config.IMAPPort == 0 {
		config.IMAPPort = 993
	}
	if config.IMAPMailbox == "" {
		config.IMAPMailbox = "INBOX"
	}
	if config.SMTPPort == 0 {
		config.SMTPPort = 587
	}
	if config.PollIntervalSeconds <= 0 {
		config.PollIntervalSeconds = 30
	}
	return &EmailChannel{
		config:   config,
		stopChan: make(chan struct{}),
	}
}

// Name 返回频道名称
func (e *EmailChannel) Name() string {
	return "email"
}

// IsEnabled 是否启用
func (e *EmailChannel) IsEnabled() bool {
	return e.config.Enabled &&
		e.config.ConsentGranted &&
		strings.TrimSpace(e.config.IMAPHost) != "" &&
		strings.TrimSpace(e.config.IMAPUsername) != "" &&
		strings.TrimSpace(e.config.IMAPPassword) != "" &&
		strings.TrimSpace(e.config.SMTPHost) != ""
}

// SetMessageHandler 设置消息处理器
func (e *EmailChannel) SetMessageHandler(handler func(msg *Message)) {
	e.messageHandler = handler
}

// Start 启动轮询
func (e *EmailChannel) Start(ctx context.Context) error {
	if !e.IsEnabled() {
		return nil
	}

	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		ticker := time.NewTicker(time.Duration(e.config.PollIntervalSeconds) * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-e.stopChan:
				return
			case <-ticker.C:
				_ = e.pollOnce()
			}
		}
	}()

	return nil
}

// Stop 停止（轮询基于 ctx 自动停止）
func (e *EmailChannel) Stop() error {
	e.stopOnce.Do(func() {
		close(e.stopChan)
	})
	e.wg.Wait()
	return nil
}

// SendMessage 通过 SMTP 发信
func (e *EmailChannel) SendMessage(chatID string, text string) error {
	if !e.IsEnabled() {
		return fmt.Errorf("email channel not enabled")
	}
	if !e.config.AutoReplyEnabled {
		return nil
	}

	from := strings.TrimSpace(e.config.FromAddress)
	if from == "" {
		from = strings.TrimSpace(e.config.SMTPUsername)
	}
	if from == "" {
		from = strings.TrimSpace(e.config.IMAPUsername)
	}

	msg := buildEmailMessage(from, chatID, "Re: maxclaw reply", text)
	return sendSMTP(e.config, from, chatID, msg)
}

func (e *EmailChannel) pollOnce() error {
	c, err := e.dialIMAP()
	if err != nil {
		return err
	}
	defer c.Logout()

	if _, err := c.Select(e.config.IMAPMailbox, false); err != nil {
		return err
	}

	criteria := imap.NewSearchCriteria()
	criteria.WithoutFlags = []string{imap.SeenFlag}
	ids, err := c.Search(criteria)
	if err != nil || len(ids) == 0 {
		return err
	}

	for _, id := range ids {
		if err := e.fetchAndHandleOne(c, id); err != nil {
			continue
		}
	}
	return nil
}

func (e *EmailChannel) fetchAndHandleOne(c *imapclient.Client, id uint32) error {
	seqset := new(imap.SeqSet)
	seqset.AddNum(id)
	section := &imap.BodySectionName{}
	items := []imap.FetchItem{imap.FetchEnvelope, section.FetchItem()}

	msgCh := make(chan *imap.Message, 1)
	done := make(chan error, 1)
	go func() {
		done <- c.Fetch(seqset, items, msgCh)
	}()

	var fetched *imap.Message
	for msg := range msgCh {
		fetched = msg
	}
	if err := <-done; err != nil {
		return err
	}
	if fetched == nil {
		return nil
	}

	sender := extractEmailSender(fetched.Envelope)
	if sender == "" || !e.allowedSender(sender) {
		return nil
	}

	body := ""
	if r := fetched.GetBody(section); r != nil {
		raw, _ := io.ReadAll(r)
		body = extractEmailBody(raw)
	}

	subject := ""
	if fetched.Envelope != nil {
		subject = fetched.Envelope.Subject
	}
	content := strings.TrimSpace(body)
	if content == "" {
		content = subject
	}
	if content == "" || e.messageHandler == nil {
		return nil
	}

	e.messageHandler(&Message{
		ID:      "email-" + strconv.FormatUint(uint64(id), 10),
		Text:    "Email received\nFrom: " + sender + "\nSubject: " + subject + "\n\n" + content,
		Sender:  sender,
		ChatID:  sender,
		Channel: "email",
		Raw:     fetched,
	})

	if e.config.MarkSeen {
		_ = c.Store(seqset, imap.FormatFlagsOp(imap.AddFlags, true), []interface{}{imap.SeenFlag}, nil)
	}
	return nil
}

func (e *EmailChannel) dialIMAP() (*imapclient.Client, error) {
	addr := fmt.Sprintf("%s:%d", e.config.IMAPHost, e.config.IMAPPort)
	var c *imapclient.Client
	var err error
	if e.config.IMAPUseSSL {
		c, err = imapclient.DialTLS(addr, &tls.Config{MinVersion: tls.VersionTLS12})
	} else {
		c, err = imapclient.Dial(addr)
	}
	if err != nil {
		return nil, err
	}
	if err := c.Login(e.config.IMAPUsername, e.config.IMAPPassword); err != nil {
		_ = c.Logout()
		return nil, err
	}
	return c, nil
}

func (e *EmailChannel) allowedSender(sender string) bool {
	if len(e.config.AllowFrom) == 0 {
		return true
	}
	sender = strings.ToLower(strings.TrimSpace(sender))
	for _, v := range e.config.AllowFrom {
		if strings.ToLower(strings.TrimSpace(v)) == sender {
			return true
		}
	}
	return false
}

func extractEmailSender(env *imap.Envelope) string {
	if env == nil || len(env.From) == 0 {
		return ""
	}
	addr := env.From[0]
	mailbox := strings.TrimSpace(addr.MailboxName)
	host := strings.TrimSpace(addr.HostName)
	if mailbox == "" || host == "" {
		return ""
	}
	return strings.ToLower(mailbox + "@" + host)
}

func extractEmailBody(raw []byte) string {
	msg, err := mail.ReadMessage(bufio.NewReader(strings.NewReader(string(raw))))
	if err != nil {
		return strings.TrimSpace(string(raw))
	}
	body, err := io.ReadAll(msg.Body)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(body))
}

func buildEmailMessage(from, to, subject, body string) []byte {
	headers := []string{
		"From: " + from,
		"To: " + to,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"",
		body,
	}
	return []byte(strings.Join(headers, "\r\n"))
}

func sendSMTP(cfg *EmailConfig, from, to string, msg []byte) error {
	addr := fmt.Sprintf("%s:%d", cfg.SMTPHost, cfg.SMTPPort)
	host := cfg.SMTPHost
	auth := smtp.PlainAuth("", cfg.SMTPUsername, cfg.SMTPPassword, host)

	if cfg.SMTPUseSSL {
		conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12})
		if err != nil {
			return err
		}
		defer conn.Close()

		client, err := smtp.NewClient(conn, host)
		if err != nil {
			return err
		}
		defer client.Quit()

		if err := client.Auth(auth); err != nil {
			return err
		}
		if err := client.Mail(from); err != nil {
			return err
		}
		if err := client.Rcpt(to); err != nil {
			return err
		}
		w, err := client.Data()
		if err != nil {
			return err
		}
		if _, err := w.Write(msg); err != nil {
			return err
		}
		return w.Close()
	}

	if !cfg.SMTPUseTLS {
		return smtp.SendMail(addr, auth, from, []string{to}, msg)
	}

	client, err := smtp.Dial(addr)
	if err != nil {
		return err
	}
	defer client.Quit()

	if err := client.StartTLS(&tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}); err != nil {
		return err
	}
	if err := client.Auth(auth); err != nil {
		return err
	}
	if err := client.Mail(from); err != nil {
		return err
	}
	if err := client.Rcpt(to); err != nil {
		return err
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		return err
	}
	return w.Close()
}
