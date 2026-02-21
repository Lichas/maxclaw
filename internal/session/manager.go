package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Message 会话消息
type Message struct {
	Role      string          `json:"role"`
	Content   string          `json:"content"`
	Timeline  []TimelineEntry `json:"timeline,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
}

type TimelineActivity struct {
	Type    string `json:"type"`
	Summary string `json:"summary"`
	Detail  string `json:"detail,omitempty"`
}

type TimelineEntry struct {
	Kind     string            `json:"kind"`
	Activity *TimelineActivity `json:"activity,omitempty"`
	Text     string            `json:"text,omitempty"`
}

// Session 会话
type Session struct {
	Key              string    `json:"key"`
	Messages         []Message `json:"messages"`
	LastConsolidated int       `json:"lastConsolidated,omitempty"`
}

// Manager 会话管理器
type Manager struct {
	workspace string
	sessions  map[string]*Session
	mu        sync.RWMutex
}

// NewManager 创建会话管理器
func NewManager(workspace string) *Manager {
	return &Manager{
		workspace: workspace,
		sessions:  make(map[string]*Session),
	}
}

// GetOrCreate 获取或创建会话
func (m *Manager) GetOrCreate(key string) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()

	if session, exists := m.sessions[key]; exists {
		return session
	}

	// 尝试从文件加载
	session := m.loadFromFile(key)
	if session == nil {
		session = &Session{
			Key:      key,
			Messages: make([]Message, 0),
		}
	}

	m.sessions[key] = session
	return session
}

// Save 保存会话
func (m *Manager) Save(session *Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.sessions[session.Key] = session
	return m.saveToFile(session)
}

// AddMessage 添加消息到会话
func (s *Session) AddMessage(role, content string) {
	s.AddMessageWithTimeline(role, content, nil)
}

func (s *Session) AddMessageWithTimeline(role, content string, timeline []TimelineEntry) {
	var timelineCopy []TimelineEntry
	if len(timeline) > 0 {
		timelineCopy = append([]TimelineEntry(nil), timeline...)
	}

	s.Messages = append(s.Messages, Message{
		Role:      role,
		Content:   content,
		Timeline:  timelineCopy,
		Timestamp: time.Now(),
	})
}

// GetHistory 获取历史记录
func (s *Session) GetHistory(maxMessages ...int) []Message {
	if len(maxMessages) == 0 || maxMessages[0] <= 0 {
		return s.Messages
	}
	limit := maxMessages[0]
	if len(s.Messages) <= limit {
		return s.Messages
	}
	return s.Messages[len(s.Messages)-limit:]
}

// Clear 清空会话
func (s *Session) Clear() {
	s.Messages = make([]Message, 0)
	s.LastConsolidated = 0
}

// getSessionFilePath 获取会话文件路径
func (m *Manager) getSessionFilePath(key string) string {
	// 将 key 中的特殊字符替换为安全字符
	safeKey := sanitizeFilename(key)
	return filepath.Join(m.workspace, ".sessions", safeKey+".json")
}

// loadFromFile 从文件加载会话
func (m *Manager) loadFromFile(key string) *Session {
	filePath := m.getSessionFilePath(key)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil
	}

	return &session
}

// saveToFile 保存会话到文件
func (m *Manager) saveToFile(session *Session) error {
	filePath := m.getSessionFilePath(session.Key)
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create session directory: %w", err)
	}

	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write session file: %w", err)
	}

	return nil
}

// sanitizeFilename 清理文件名
func sanitizeFilename(name string) string {
	// 简单的清理，替换不安全的字符
	result := make([]byte, 0, len(name))
	for _, c := range name {
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' {
			result = append(result, byte(c))
		} else {
			result = append(result, '_')
		}
	}
	return string(result)
}
