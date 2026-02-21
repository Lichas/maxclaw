package bus

// MediaAttachment 媒体附件
type MediaAttachment struct {
	Type     string `json:"type"` // image, audio, video, document
	URL      string `json:"url,omitempty"`
	FileID   string `json:"fileId,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
}

// InboundMessage 入站消息
type InboundMessage struct {
	Channel        string           `json:"channel"`                  // telegram, discord, whatsapp, cli
	SenderID       string           `json:"senderId"`                 // 发送者 ID
	ChatID         string           `json:"chatId"`                   // 会话 ID
	Content        string           `json:"content"`                  // 消息内容
	SelectedSkills []string         `json:"selectedSkills,omitempty"` // optional explicit skill filters
	Media          *MediaAttachment `json:"media,omitempty"`
	SessionKey     string           `json:"sessionKey"` // channel:chatId
}

// NewInboundMessage 创建入站消息
func NewInboundMessage(channel, senderID, chatID, content string) *InboundMessage {
	return &InboundMessage{
		Channel:    channel,
		SenderID:   senderID,
		ChatID:     chatID,
		Content:    content,
		SessionKey: channel + ":" + chatID,
	}
}

// OutboundMessage 出站消息
type OutboundMessage struct {
	Channel string `json:"channel"`
	ChatID  string `json:"chatId"`
	Content string `json:"content"`
}

// NewOutboundMessage 创建出站消息
func NewOutboundMessage(channel, chatID, content string) *OutboundMessage {
	return &OutboundMessage{
		Channel: channel,
		ChatID:  chatID,
		Content: content,
	}
}
