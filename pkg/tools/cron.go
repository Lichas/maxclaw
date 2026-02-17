package tools

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Lichas/nanobot-go/internal/cron"
)

// CronService 定时任务服务接口
type CronService interface {
	AddJob(name string, schedule cron.Schedule, payload cron.Payload) (*cron.Job, error)
	RemoveJob(id string) bool
	ListJobs() []*cron.Job
}

// CronTool 定时任务工具
type CronTool struct {
	BaseTool
	service CronService
	mu      sync.RWMutex
	channel string
	chatID  string
}

// NewCronTool 创建定时任务工具
func NewCronTool(service CronService) *CronTool {
	return &CronTool{
		BaseTool: BaseTool{
			name:        "cron",
			description: "Schedule reminders and recurring tasks. Actions: add, list, remove. Use for setting up reminders, periodic checks, or scheduled notifications.",
			parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"action": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"add", "list", "remove"},
						"description": "Action to perform",
					},
					"message": map[string]interface{}{
						"type":        "string",
						"description": "Reminder message (required for add)",
					},
					"every_seconds": map[string]interface{}{
						"type":        "integer",
						"description": "Interval in seconds (for recurring tasks, e.g., 3600 for hourly)",
						"minimum":     1,
					},
					"cron_expr": map[string]interface{}{
						"type":        "string",
						"description": "Cron expression like '0 9 * * *' for daily at 9am",
					},
					"at": map[string]interface{}{
						"type":        "string",
						"description": "ISO datetime for one-time execution (e.g. '2026-02-12T10:30:00')",
					},
					"job_id": map[string]interface{}{
						"type":        "string",
						"description": "Job ID (required for remove)",
					},
				},
				"required": []string{"action"},
			},
		},
		service: service,
	}
}

// SetContext 设置当前上下文
func (t *CronTool) SetContext(channel, chatID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.channel = channel
	t.chatID = chatID
}

// Execute 执行定时任务操作
func (t *CronTool) Execute(ctx context.Context, params map[string]interface{}) (string, error) {
	action, _ := params["action"].(string)
	if action == "" {
		return "", fmt.Errorf("action is required")
	}

	switch action {
	case "add":
		return t.addJob(ctx, params)
	case "list":
		return t.listJobs()
	case "remove":
		return t.removeJob(params)
	default:
		return "", fmt.Errorf("unknown action: %s", action)
	}
}

// addJob 添加定时任务
func (t *CronTool) addJob(ctx context.Context, params map[string]interface{}) (string, error) {
	message, _ := params["message"].(string)
	if message == "" {
		return "", fmt.Errorf("message is required for add action")
	}

	t.mu.RLock()
	channel := t.channel
	chatID := t.chatID
	t.mu.RUnlock()

	// 优先使用当前请求上下文，避免并发请求时的上下文串线
	if ctxChannel, ctxChatID := RuntimeContextFrom(ctx); ctxChannel != "" || ctxChatID != "" {
		if channel == "" {
			channel = ctxChannel
		}
		if chatID == "" {
			chatID = ctxChatID
		}
	}

	if channel == "" || chatID == "" {
		return "", fmt.Errorf("no session context (channel/chat_id)")
	}

	if t.service == nil {
		return "", fmt.Errorf("cron service not available")
	}

	// 解析调度配置
	var schedule cron.Schedule

	if v, ok := params["every_seconds"]; ok {
		// 周期性任务
		var everySec int
		switch val := v.(type) {
		case float64:
			everySec = int(val)
		case int:
			everySec = val
		case string:
			parsed, err := strconv.Atoi(val)
			if err != nil {
				return "", fmt.Errorf("invalid every_seconds value")
			}
			everySec = parsed
		default:
			return "", fmt.Errorf("invalid every_seconds type")
		}

		if everySec <= 0 {
			return "", fmt.Errorf("every_seconds must be positive")
		}

		schedule = cron.Schedule{
			Type:    cron.ScheduleTypeEvery,
			EveryMs: int64(everySec) * 1000,
		}
	} else if v, ok := params["cron_expr"]; ok {
		// Cron 表达式任务
		expr, ok := v.(string)
		if !ok || expr == "" {
			return "", fmt.Errorf("invalid cron_expr")
		}
		schedule = cron.Schedule{
			Type: cron.ScheduleTypeCron,
			Expr: expr,
		}
	} else if v, ok := params["at"]; ok {
		raw, ok := v.(string)
		if !ok {
			return "", fmt.Errorf("invalid at")
		}
		runAt, err := parseCronAt(raw)
		if err != nil {
			return "", fmt.Errorf("invalid at, expected ISO datetime")
		}
		schedule = cron.Schedule{
			Type: cron.ScheduleTypeOnce,
			AtMs: runAt.UnixMilli(),
		}
	} else {
		return "", fmt.Errorf("either every_seconds, cron_expr, or at is required")
	}

	// 构建任务名称
	name := message
	if len(name) > 30 {
		name = name[:30] + "..."
	}

	// 构建负载
	payload := cron.Payload{
		Message: message,
		Channel: channel,
		To:      chatID,
		Deliver: true,
	}

	// 调用服务添加任务
	job, err := t.service.AddJob(name, schedule, payload)
	if err != nil {
		return "", fmt.Errorf("failed to add job: %w", err)
	}

	return fmt.Sprintf("Created job '%s' (id: %s)", job.Name, job.ID), nil
}

// listJobs 列出所有定时任务
func (t *CronTool) listJobs() (string, error) {
	if t.service == nil {
		return "No scheduled jobs.", nil
	}

	jobs := t.service.ListJobs()
	if len(jobs) == 0 {
		return "No scheduled jobs.", nil
	}

	result := "Scheduled jobs:\n"
	for i, job := range jobs {
		status := "enabled"
		if !job.Enabled {
			status = "disabled"
		}
		schedule := ""
		switch job.Schedule.Type {
		case cron.ScheduleTypeEvery:
			schedule = fmt.Sprintf("every %d seconds", job.Schedule.EveryMs/1000)
		case cron.ScheduleTypeCron:
			schedule = fmt.Sprintf("cron: %s", job.Schedule.Expr)
		case cron.ScheduleTypeOnce:
			schedule = fmt.Sprintf("at: %s", time.UnixMilli(job.Schedule.AtMs).Format(time.RFC3339))
		}
		result += fmt.Sprintf("%d. %s (id: %s, %s, %s)\n", i+1, job.Name, job.ID, schedule, status)
	}
	return result, nil
}

func parseCronAt(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, fmt.Errorf("empty at")
	}

	zoneLayouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
	}
	for _, layout := range zoneLayouts {
		if t, err := time.Parse(layout, raw); err == nil {
			return t, nil
		}
	}

	localLayouts := []string{
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04",
	}
	for _, layout := range localLayouts {
		if t, err := time.ParseInLocation(layout, raw, time.Local); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("invalid at format")
}

// removeJob 删除定时任务
func (t *CronTool) removeJob(params map[string]interface{}) (string, error) {
	jobID, _ := params["job_id"].(string)
	if jobID == "" {
		return "", fmt.Errorf("job_id is required for remove action")
	}

	if t.service == nil {
		return "", fmt.Errorf("cron service not available")
	}

	if t.service.RemoveJob(jobID) {
		return fmt.Sprintf("Removed job %s", jobID), nil
	}
	return "", fmt.Errorf("job %s not found", jobID)
}
