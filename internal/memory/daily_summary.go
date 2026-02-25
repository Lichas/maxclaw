package memory

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Lichas/maxclaw/internal/logging"
	"github.com/Lichas/maxclaw/internal/session"
)

const (
	dailySummaryHeader = "## Daily Summaries"
	defaultMemoryBody  = "# Long-term Memory\n\nThis file stores important information that should persist across sessions.\n"
)

type DailySummaryService struct {
	workspace string
	interval  time.Duration
}

type summaryData struct {
	sessionKeys map[string]struct{}
	userMsgs    []string
	assistMsgs  []string
	totalMsgs   int
}

func NewDailySummaryService(workspace string, interval time.Duration) *DailySummaryService {
	if interval <= 0 {
		interval = time.Hour
	}
	return &DailySummaryService{
		workspace: workspace,
		interval:  interval,
	}
}

func (s *DailySummaryService) Start(ctx context.Context) {
	s.run(time.Now())

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			s.run(now)
		}
	}
}

func (s *DailySummaryService) RunOnce(now time.Time) (bool, error) {
	return SummarizePreviousDay(s.workspace, now)
}

func (s *DailySummaryService) run(now time.Time) {
	updated, err := s.RunOnce(now)
	if err != nil {
		if lg := logging.Get(); lg != nil && lg.Cron != nil {
			lg.Cron.Printf("daily memory summary error: %v", err)
		}
		return
	}
	if updated {
		if lg := logging.Get(); lg != nil && lg.Cron != nil {
			lg.Cron.Printf("daily memory summary updated for %s", now.AddDate(0, 0, -1).Format("2006-01-02"))
		}
	}
}

// SummarizePreviousDay appends yesterday's summary to memory/MEMORY.md if not summarized yet.
func SummarizePreviousDay(workspace string, now time.Time) (bool, error) {
	if workspace == "" {
		return false, fmt.Errorf("workspace is empty")
	}
	if now.IsZero() {
		now = time.Now()
	}
	loc := now.Location()
	day := now.In(loc).AddDate(0, 0, -1)
	return SummarizeDay(workspace, day)
}

// SummarizeDay appends one day's summary to memory/MEMORY.md (idempotent by date heading).
func SummarizeDay(workspace string, day time.Time) (bool, error) {
	if workspace == "" {
		return false, fmt.Errorf("workspace is empty")
	}
	if day.IsZero() {
		return false, fmt.Errorf("summary day is zero")
	}

	dayKey := day.In(day.Location()).Format("2006-01-02")
	summary, err := buildSummaryForDay(workspace, day)
	if err != nil {
		return false, err
	}
	if summary == "" {
		return false, nil
	}

	memoryPath := filepath.Join(workspace, "memory", "MEMORY.md")
	content, err := loadOrInitMemory(memoryPath)
	if err != nil {
		return false, err
	}

	if alreadySummarized(content, dayKey) {
		return false, nil
	}

	if !strings.Contains(content, dailySummaryHeader) {
		content = strings.TrimRight(content, "\n") + "\n\n" + dailySummaryHeader + "\n"
	}

	content = strings.TrimRight(content, "\n") + "\n\n" + summary + "\n"
	if err := os.WriteFile(memoryPath, []byte(content), 0644); err != nil {
		return false, fmt.Errorf("write memory file: %w", err)
	}
	return true, nil
}

func loadOrInitMemory(path string) (string, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return "", fmt.Errorf("create memory dir: %w", err)
	}

	data, err := os.ReadFile(path)
	if err == nil {
		return string(data), nil
	}
	if !os.IsNotExist(err) {
		return "", fmt.Errorf("read memory file: %w", err)
	}

	if err := os.WriteFile(path, []byte(defaultMemoryBody), 0644); err != nil {
		return "", fmt.Errorf("init memory file: %w", err)
	}
	return defaultMemoryBody, nil
}

func alreadySummarized(memoryBody, dayKey string) bool {
	needle := "### " + dayKey
	return strings.Contains(memoryBody, needle)
}

func buildSummaryForDay(workspace string, day time.Time) (string, error) {
	data := summaryData{
		sessionKeys: map[string]struct{}{},
	}
	dayKey := day.In(day.Location()).Format("2006-01-02")

	if err := collectSummaryFromSessionFiles(workspace, day, &data); err != nil {
		return "", err
	}
	if err := collectSummaryFromHistory(workspace, dayKey, &data); err != nil {
		return "", err
	}

	if data.totalMsgs == 0 {
		return "", nil
	}

	userHighlights := uniqueTopN(data.userMsgs, 5)
	assistantHighlights := uniqueTopN(data.assistMsgs, 3)
	dayDisplay := day.In(day.Location()).Format("2006-01-02")

	var sb strings.Builder
	sb.WriteString("### " + dayDisplay + "\n")
	sb.WriteString(fmt.Sprintf("- Sessions active: %d\n", len(data.sessionKeys)))
	sb.WriteString(fmt.Sprintf("- Message count: %d\n", data.totalMsgs))

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

	return strings.TrimRight(sb.String(), "\n"), nil
}

func collectSummaryFromSessionFiles(workspace string, day time.Time, data *summaryData) error {
	sessionsDir := filepath.Join(workspace, ".sessions")
	err := filepath.WalkDir(sessionsDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".json") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		var sess session.Session
		if err := json.Unmarshal(content, &sess); err != nil {
			return nil
		}

		dayKey := day.In(day.Location()).Format("2006-01-02")
		for _, msg := range sess.Messages {
			if msg.Timestamp.IsZero() {
				continue
			}
			if msg.Timestamp.In(day.Location()).Format("2006-01-02") != dayKey {
				continue
			}

			data.sessionKeys[sess.Key] = struct{}{}
			data.totalMsgs++

			clean := cleanMessage(msg.Content)
			if clean == "" {
				continue
			}
			switch msg.Role {
			case "user":
				data.userMsgs = append(data.userMsgs, clean)
			case "assistant":
				data.assistMsgs = append(data.assistMsgs, clean)
			}
		}

		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("walk sessions dir: %w", err)
	}
	return nil
}

func collectSummaryFromHistory(workspace, dayKey string, data *summaryData) error {
	historyPath := filepath.Join(workspace, "memory", "HISTORY.md")
	file, err := os.Open(historyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("open history file: %w", err)
	}
	defer file.Close()

	var inTargetDayEntry bool
	var inUserHighlights bool
	var inAssistantHighlights bool

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		raw := scanner.Text()
		line := strings.TrimSpace(raw)

		if strings.HasPrefix(line, "### [") {
			inTargetDayEntry = false
			inUserHighlights = false
			inAssistantHighlights = false

			if !strings.Contains(line, "["+dayKey+" ") {
				continue
			}
			inTargetDayEntry = true

			if sessionKey := extractHistorySessionKey(line); sessionKey != "" {
				data.sessionKeys[sessionKey] = struct{}{}
			}
			continue
		}

		if !inTargetDayEntry {
			continue
		}

		if strings.HasPrefix(line, "- Messages consolidated:") {
			var n int
			if _, err := fmt.Sscanf(line, "- Messages consolidated: %d", &n); err == nil && n > 0 {
				data.totalMsgs += n
			}
			continue
		}
		if line == "- User highlights:" {
			inUserHighlights = true
			inAssistantHighlights = false
			continue
		}
		if line == "- Assistant highlights:" {
			inUserHighlights = false
			inAssistantHighlights = true
			continue
		}
		if strings.HasPrefix(line, "- ") && !strings.HasPrefix(raw, "  - ") {
			inUserHighlights = false
			inAssistantHighlights = false
			continue
		}

		if !strings.HasPrefix(raw, "  - ") {
			continue
		}

		item := strings.TrimSpace(strings.TrimPrefix(raw, "  - "))
		if item == "" {
			continue
		}

		if inUserHighlights {
			data.userMsgs = append(data.userMsgs, item)
			continue
		}
		if inAssistantHighlights {
			data.assistMsgs = append(data.assistMsgs, item)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan history file: %w", err)
	}
	return nil
}

func extractHistorySessionKey(heading string) string {
	const marker = "] session: "
	idx := strings.Index(heading, marker)
	if idx < 0 {
		return ""
	}
	return strings.TrimSpace(heading[idx+len(marker):])
}

func cleanMessage(s string) string {
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

func uniqueTopN(items []string, n int) []string {
	if n <= 0 {
		return nil
	}
	out := make([]string, 0, n)
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
		if len(out) >= n {
			break
		}
	}
	return out
}
