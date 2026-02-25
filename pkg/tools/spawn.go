package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// SpawnRequest describes a background sub-session request.
type SpawnRequest struct {
	Task             string
	Label            string
	Model            string
	SelectedSkills   []string
	EnabledSources   []string
	SessionKey       string
	NotifyParent     bool
	Channel          string
	ChatID           string
	ParentSessionKey string
}

// SpawnResult contains sub-session execution metadata returned by callback.
type SpawnResult struct {
	SessionKey string `json:"sessionKey"`
	Message    string `json:"message,omitempty"`
}

// SpawnCallback 子代理回调函数类型
type SpawnCallback func(ctx context.Context, request SpawnRequest) (SpawnResult, error)

// SpawnTool 子代理工具 - 用于后台任务执行
type SpawnTool struct {
	BaseTool
	callback     SpawnCallback
	mu           sync.RWMutex
	channel      string
	chatID       string
	runningTasks map[string]*SpawnTask
}

// SpawnTask 表示一个正在运行的后台任务
type SpawnTask struct {
	ID         string
	Label      string
	Task       string
	Model      string
	Skills     []string
	Sources    []string
	SessionKey string
	StartTime  time.Time
	EndTime    *time.Time
	Status     string
	Result     string
	Error      string
}

// NewSpawnTool 创建子代理工具
func NewSpawnTool(callback SpawnCallback) *SpawnTool {
	return &SpawnTool{
		BaseTool: BaseTool{
			name:        "spawn",
			description: "Spawn a subagent to handle a task in the background. Use this for complex or time-consuming tasks that can run independently. The subagent will complete the task and report back when done.",
			parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"task": map[string]interface{}{
						"type":        "string",
						"description": "The task for the subagent to complete",
						"minLength":   1,
					},
					"label": map[string]interface{}{
						"type":        "string",
						"description": "Optional short label for the task (for display)",
					},
					"model": map[string]interface{}{
						"type":        "string",
						"description": "Optional model override for the sub-session",
					},
					"selected_skills": map[string]interface{}{
						"type":        "array",
						"description": "Optional list of skill names for the sub-session context",
						"items": map[string]interface{}{
							"type": "string",
						},
					},
					"enabled_sources": map[string]interface{}{
						"type":        "array",
						"description": "Optional preferred source slugs for the sub-session",
						"items": map[string]interface{}{
							"type": "string",
						},
					},
					"session_key": map[string]interface{}{
						"type":        "string",
						"description": "Optional custom session key for the spawned sub-session",
					},
					"notify_parent": map[string]interface{}{
						"type":        "boolean",
						"description": "Whether to notify parent channel/chat when sub-session starts/completes (default true)",
					},
				},
				"required": []string{"task"},
			},
		},
		callback:     callback,
		runningTasks: make(map[string]*SpawnTask),
	}
}

// SetContext 设置当前上下文
func (t *SpawnTool) SetContext(channel, chatID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.channel = channel
	t.chatID = chatID
}

// Execute 执行子代理任务
func (t *SpawnTool) Execute(ctx context.Context, params map[string]interface{}) (string, error) {
	task, _ := params["task"].(string)
	if task == "" {
		return "", fmt.Errorf("task is required")
	}

	label, _ := params["label"].(string)
	if label == "" {
		label = task[:min(30, len(task))]
	}
	model, _ := params["model"].(string)
	sessionKey, _ := params["session_key"].(string)
	notifyParent := true
	if v, ok := params["notify_parent"].(bool); ok {
		notifyParent = v
	}
	selectedSkills := toStringSlice(params["selected_skills"])
	enabledSources := toStringSlice(params["enabled_sources"])
	channel, chatID := RuntimeContextFrom(ctx)
	parentSessionKey := RuntimeSessionKeyFrom(ctx)

	// 生成任务ID
	taskID := generateTaskID()

	// 记录任务
	spawnTask := &SpawnTask{
		ID:         taskID,
		Label:      label,
		Task:       task,
		Model:      model,
		Skills:     append([]string(nil), selectedSkills...),
		Sources:    append([]string(nil), enabledSources...),
		SessionKey: sessionKey,
		StartTime:  time.Now(),
		Status:     "running",
	}

	t.mu.Lock()
	t.runningTasks[taskID] = spawnTask
	t.mu.Unlock()

	// 在后台执行
	request := SpawnRequest{
		Task:             task,
		Label:            label,
		Model:            model,
		SelectedSkills:   selectedSkills,
		EnabledSources:   enabledSources,
		SessionKey:       sessionKey,
		NotifyParent:     notifyParent,
		Channel:          channel,
		ChatID:           chatID,
		ParentSessionKey: parentSessionKey,
	}
	go t.runTask(spawnTask, request)

	metadata := map[string]interface{}{
		"taskId":       taskID,
		"label":        label,
		"task":         task,
		"model":        model,
		"sessionKey":   sessionKey,
		"skills":       selectedSkills,
		"sources":      enabledSources,
		"notifyParent": notifyParent,
	}
	payload, _ := json.Marshal(metadata)
	return fmt.Sprintf("Spawned subagent '%s' (id: %s)\n%s", label, taskID, string(payload)), nil
}

// runTask 在后台运行任务
func (t *SpawnTool) runTask(task *SpawnTask, request SpawnRequest) {
	defer func() {
		t.mu.Lock()
		delete(t.runningTasks, task.ID)
		t.mu.Unlock()
	}()
	startCtx := context.Background()
	if t.callback == nil {
		time.Sleep(100 * time.Millisecond)
		task.Status = "completed"
		task.Result = "completed with no callback handler"
		now := time.Now()
		task.EndTime = &now
		return
	}

	result, err := t.callback(startCtx, request)
	now := time.Now()
	task.EndTime = &now
	if err != nil {
		task.Status = "failed"
		task.Error = err.Error()
		return
	}
	task.Status = "completed"
	task.SessionKey = result.SessionKey
	task.Result = result.Message
}

// ListRunningTasks 列出正在运行的任务
func (t *SpawnTool) ListRunningTasks() []*SpawnTask {
	t.mu.RLock()
	defer t.mu.RUnlock()

	tasks := make([]*SpawnTask, 0, len(t.runningTasks))
	for _, task := range t.runningTasks {
		tasks = append(tasks, task)
	}
	return tasks
}

// generateTaskID 生成任务ID
func generateTaskID() string {
	return fmt.Sprintf("task_%d", time.Now().UnixNano())
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func toStringSlice(v interface{}) []string {
	list, ok := v.([]interface{})
	if !ok || len(list) == 0 {
		return nil
	}
	out := make([]string, 0, len(list))
	for _, item := range list {
		s, ok := item.(string)
		if !ok {
			continue
		}
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		out = append(out, s)
	}
	return out
}
