package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Lichas/nanobot-go/internal/session"
)

const (
	defaultMemoryTemplate  = "# Long-term Memory\n\nThis file stores important information that should persist across sessions.\n"
	defaultHistoryTemplate = "# Conversation History\n\nAppend-only summaries for grep-based recall.\n"
)

type Store struct {
	workspace   string
	memoryDir   string
	memoryPath  string
	historyPath string
}

func NewStore(workspace string) *Store {
	memoryDir := filepath.Join(workspace, "memory")
	return &Store{
		workspace:   workspace,
		memoryDir:   memoryDir,
		memoryPath:  filepath.Join(memoryDir, "MEMORY.md"),
		historyPath: filepath.Join(memoryDir, "HISTORY.md"),
	}
}

func (s *Store) EnsureFiles() error {
	if s.workspace == "" {
		return fmt.Errorf("workspace is empty")
	}
	if err := os.MkdirAll(s.memoryDir, 0755); err != nil {
		return fmt.Errorf("create memory dir: %w", err)
	}
	if err := ensureFileWithDefault(s.memoryPath, defaultMemoryTemplate); err != nil {
		return err
	}
	if err := ensureFileWithDefault(s.historyPath, defaultHistoryTemplate); err != nil {
		return err
	}
	return nil
}

func (s *Store) ReadLongTerm() (string, error) {
	if err := s.EnsureFiles(); err != nil {
		return "", err
	}
	body, err := os.ReadFile(s.memoryPath)
	if err != nil {
		return "", fmt.Errorf("read memory file: %w", err)
	}
	return string(body), nil
}

func (s *Store) WriteLongTerm(content string) error {
	if err := s.EnsureFiles(); err != nil {
		return err
	}
	return os.WriteFile(s.memoryPath, []byte(content), 0644)
}

func (s *Store) AppendHistory(entry string) error {
	if strings.TrimSpace(entry) == "" {
		return nil
	}
	if err := s.EnsureFiles(); err != nil {
		return err
	}
	f, err := os.OpenFile(s.historyPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open history file: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString(strings.TrimSpace(entry) + "\n\n"); err != nil {
		return fmt.Errorf("append history: %w", err)
	}
	return nil
}

func (s *Store) HistoryPath() string {
	return s.historyPath
}

func ensureFileWithDefault(path, content string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat file %s: %w", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("write default file %s: %w", path, err)
	}
	return nil
}

// ConsolidateSession appends a compact session summary to HISTORY.md and updates LastConsolidated.
// keepRecent controls how many most-recent messages stay unconsolidated.
func ConsolidateSession(workspace string, sess *session.Session, keepRecent int) (bool, error) {
	if sess == nil {
		return false, fmt.Errorf("session is nil")
	}
	if keepRecent < 0 {
		keepRecent = 0
	}
	end := len(sess.Messages) - keepRecent
	if end <= 0 || end <= sess.LastConsolidated {
		return false, nil
	}

	entry := buildHistoryEntry(sess.Key, sess.Messages[sess.LastConsolidated:end], time.Now())
	sess.LastConsolidated = end
	if strings.TrimSpace(entry) == "" {
		return false, nil
	}

	store := NewStore(workspace)
	if err := store.AppendHistory(entry); err != nil {
		return false, err
	}
	return true, nil
}

// ArchiveSessionAll consolidates all not-yet-consolidated messages.
func ArchiveSessionAll(workspace string, sess *session.Session) (bool, error) {
	return ConsolidateSession(workspace, sess, 0)
}

func buildHistoryEntry(sessionKey string, msgs []session.Message, now time.Time) string {
	if len(msgs) == 0 {
		return ""
	}

	var userHighlights []string
	var assistantHighlights []string
	totalNonEmpty := 0

	for _, msg := range msgs {
		clean := cleanHistoryLine(msg.Content)
		if clean == "" {
			continue
		}
		totalNonEmpty++
		switch msg.Role {
		case "user":
			userHighlights = append(userHighlights, clean)
		case "assistant":
			assistantHighlights = append(assistantHighlights, clean)
		}
	}

	if totalNonEmpty == 0 {
		return ""
	}

	userHighlights = uniqueTopN(userHighlights, 4)
	assistantHighlights = uniqueTopN(assistantHighlights, 2)

	var sb strings.Builder
	sb.WriteString("### [")
	sb.WriteString(now.Format("2006-01-02 15:04"))
	sb.WriteString("] session: ")
	sb.WriteString(sessionKey)
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("- Messages consolidated: %d\n", len(msgs)))

	if len(userHighlights) > 0 {
		sb.WriteString("- User highlights:\n")
		for _, item := range userHighlights {
			sb.WriteString("  - " + item + "\n")
		}
	}
	if len(assistantHighlights) > 0 {
		sb.WriteString("- Assistant highlights:\n")
		for _, item := range assistantHighlights {
			sb.WriteString("  - " + item + "\n")
		}
	}

	return strings.TrimRight(sb.String(), "\n")
}

func cleanHistoryLine(s string) string {
	fields := strings.Fields(strings.TrimSpace(s))
	if len(fields) == 0 {
		return ""
	}
	s = strings.Join(fields, " ")
	if len(s) > 180 {
		return s[:180] + "..."
	}
	return s
}
