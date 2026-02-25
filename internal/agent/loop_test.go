package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/Lichas/maxclaw/internal/bus"
	"github.com/Lichas/maxclaw/internal/config"
	"github.com/Lichas/maxclaw/internal/cron"
	"github.com/Lichas/maxclaw/internal/providers"
	"github.com/Lichas/maxclaw/internal/session"
	"github.com/Lichas/maxclaw/pkg/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testProvider struct {
	callCount int
}

func (p *testProvider) Chat(ctx context.Context, messages []providers.Message, defs []map[string]interface{}, model string) (*providers.Response, error) {
	return nil, nil
}

func (p *testProvider) ChatStream(ctx context.Context, messages []providers.Message, defs []map[string]interface{}, model string, handler providers.StreamHandler) error {
	if p.callCount == 0 {
		handler.OnToolCallStart("tool_1", "cron")
		handler.OnToolCallDelta("tool_1", `{"action":"add","message":"Ping me","every_seconds":60}`)
		handler.OnToolCallEnd("tool_1")
		handler.OnComplete()
		p.callCount++
		return nil
	}

	handler.OnContent("ok")
	handler.OnComplete()
	p.callCount++
	return nil
}

func (p *testProvider) GetDefaultModel() string {
	return "test-model"
}

type staticProvider struct{}

func (p *staticProvider) Chat(ctx context.Context, messages []providers.Message, defs []map[string]interface{}, model string) (*providers.Response, error) {
	return nil, nil
}

func (p *staticProvider) ChatStream(ctx context.Context, messages []providers.Message, defs []map[string]interface{}, model string, handler providers.StreamHandler) error {
	handler.OnContent("ok")
	handler.OnComplete()
	return nil
}

func (p *staticProvider) GetDefaultModel() string {
	return "test-model"
}

type endlessToolProvider struct{}

func (p *endlessToolProvider) Chat(ctx context.Context, messages []providers.Message, defs []map[string]interface{}, model string) (*providers.Response, error) {
	return nil, nil
}

func (p *endlessToolProvider) ChatStream(ctx context.Context, messages []providers.Message, defs []map[string]interface{}, model string, handler providers.StreamHandler) error {
	handler.OnToolCallStart("call_1", "does_not_exist")
	handler.OnToolCallDelta("call_1", `{}`)
	handler.OnToolCallEnd("call_1")
	handler.OnComplete()
	return nil
}

func (p *endlessToolProvider) GetDefaultModel() string {
	return "test-model"
}

type streamEventProvider struct {
	callCount int
}

func (p *streamEventProvider) Chat(ctx context.Context, messages []providers.Message, defs []map[string]interface{}, model string) (*providers.Response, error) {
	return nil, nil
}

func (p *streamEventProvider) ChatStream(ctx context.Context, messages []providers.Message, defs []map[string]interface{}, model string, handler providers.StreamHandler) error {
	if p.callCount == 0 {
		handler.OnToolCallStart("tool_1", "list_dir")
		handler.OnToolCallDelta("tool_1", `{"path":"."}`)
		handler.OnToolCallEnd("tool_1")
		handler.OnComplete()
		p.callCount++
		return nil
	}

	handler.OnContent("event stream ok")
	handler.OnComplete()
	p.callCount++
	return nil
}

func (p *streamEventProvider) GetDefaultModel() string {
	return "test-model"
}

type captureSkillsProvider struct {
	systemPrompt string
}

func (p *captureSkillsProvider) Chat(ctx context.Context, messages []providers.Message, defs []map[string]interface{}, model string) (*providers.Response, error) {
	return nil, nil
}

func (p *captureSkillsProvider) ChatStream(ctx context.Context, messages []providers.Message, defs []map[string]interface{}, model string, handler providers.StreamHandler) error {
	if len(messages) > 0 && messages[0].Role == "system" {
		p.systemPrompt = messages[0].Content
	}
	handler.OnContent("ok")
	handler.OnComplete()
	return nil
}

func (p *captureSkillsProvider) GetDefaultModel() string {
	return "test-model"
}

func TestAgentLoopProcessMessageInjectsRuntimeContextForCron(t *testing.T) {
	workspace := t.TempDir()
	messageBus := bus.NewMessageBus(10)
	provider := &testProvider{}
	cronSvc := cron.NewService(filepath.Join(workspace, ".cron", "jobs.json"))

	loop := NewAgentLoop(
		messageBus,
		provider,
		workspace,
		"test-model",
		3,
		"",
		tools.WebFetchOptions{},
		config.ExecToolConfig{Timeout: 5},
		false,
		cronSvc,
		nil,
		false,
	)

	msg := bus.NewInboundMessage("telegram", "user-1", "chat-42", "set a reminder")
	resp, err := loop.ProcessMessage(context.Background(), msg)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "ok", resp.Content)

	jobs := cronSvc.ListJobs()
	require.Len(t, jobs, 1)
	assert.Equal(t, "telegram", jobs[0].Payload.Channel)
	assert.Equal(t, "chat-42", jobs[0].Payload.To)
	assert.Equal(t, "Ping me", jobs[0].Payload.Message)
}

func TestAgentLoopProcessMessageMCPFailureDoesNotBreakMainFlow(t *testing.T) {
	workspace := t.TempDir()
	messageBus := bus.NewMessageBus(10)
	provider := &staticProvider{}
	cronSvc := cron.NewService(filepath.Join(workspace, ".cron", "jobs.json"))

	loop := NewAgentLoop(
		messageBus,
		provider,
		workspace,
		"test-model",
		3,
		"",
		tools.WebFetchOptions{},
		config.ExecToolConfig{Timeout: 5},
		false,
		cronSvc,
		map[string]config.MCPServerConfig{
			"broken": {Command: "/__nonexistent_mcp_server__"},
		},
		false,
	)

	msg := bus.NewInboundMessage("telegram", "user-1", "chat-42", "hello")
	resp, err := loop.ProcessMessage(context.Background(), msg)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "ok", resp.Content)
}

func TestAgentLoopProcessDirectUsesProvidedSessionKey(t *testing.T) {
	workspace := t.TempDir()
	messageBus := bus.NewMessageBus(10)
	provider := &staticProvider{}
	cronSvc := cron.NewService(filepath.Join(workspace, ".cron", "jobs.json"))

	loop := NewAgentLoop(
		messageBus,
		provider,
		workspace,
		"test-model",
		3,
		"",
		tools.WebFetchOptions{},
		config.ExecToolConfig{Timeout: 5},
		false,
		cronSvc,
		nil,
		false,
	)

	_, err := loop.ProcessDirect(context.Background(), "hello", "cli:custom", "cli", "direct")
	require.NoError(t, err)

	mgr := session.NewManager(workspace)
	custom := mgr.GetOrCreate("cli:custom")
	require.Len(t, custom.Messages, 2)
	assert.Equal(t, "hello", custom.Messages[0].Content)
	assert.Equal(t, "ok", custom.Messages[1].Content)

	defaultSession := mgr.GetOrCreate("cli:direct")
	assert.Len(t, defaultSession.Messages, 0)
}

func TestAgentLoopProcessMessageSlashHelp(t *testing.T) {
	workspace := t.TempDir()
	messageBus := bus.NewMessageBus(10)
	provider := &staticProvider{}
	cronSvc := cron.NewService(filepath.Join(workspace, ".cron", "jobs.json"))

	loop := NewAgentLoop(
		messageBus,
		provider,
		workspace,
		"test-model",
		3,
		"",
		tools.WebFetchOptions{},
		config.ExecToolConfig{Timeout: 5},
		false,
		cronSvc,
		nil,
		false,
	)

	msg := bus.NewInboundMessage("telegram", "user-1", "chat-42", "/help")
	resp, err := loop.ProcessMessage(context.Background(), msg)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Contains(t, resp.Content, "/new")
	assert.Contains(t, resp.Content, "/help")

	sess := loop.sessions.GetOrCreate("telegram:chat-42")
	assert.Len(t, sess.Messages, 0)
}

func TestAgentLoopProcessMessageSlashNewClearsSession(t *testing.T) {
	workspace := t.TempDir()
	messageBus := bus.NewMessageBus(10)
	provider := &staticProvider{}
	cronSvc := cron.NewService(filepath.Join(workspace, ".cron", "jobs.json"))

	loop := NewAgentLoop(
		messageBus,
		provider,
		workspace,
		"test-model",
		3,
		"",
		tools.WebFetchOptions{},
		config.ExecToolConfig{Timeout: 5},
		false,
		cronSvc,
		nil,
		false,
	)

	sess := loop.sessions.GetOrCreate("telegram:chat-42")
	sess.AddMessage("user", "old")
	sess.AddMessage("assistant", "old-reply")
	require.NoError(t, loop.sessions.Save(sess))

	msg := bus.NewInboundMessage("telegram", "user-1", "chat-42", "/new")
	resp, err := loop.ProcessMessage(context.Background(), msg)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "New session started.", resp.Content)

	cleared := loop.sessions.GetOrCreate("telegram:chat-42")
	assert.Len(t, cleared.Messages, 0)

	historyPath := filepath.Join(workspace, "memory", "HISTORY.md")
	body, readErr := os.ReadFile(historyPath)
	require.NoError(t, readErr)
	assert.Contains(t, string(body), "session: telegram:chat-42")
	assert.Contains(t, string(body), "old")
}

func TestAgentLoopProcessMessageMaxIterationFallback(t *testing.T) {
	workspace := t.TempDir()
	messageBus := bus.NewMessageBus(10)
	provider := &endlessToolProvider{}
	cronSvc := cron.NewService(filepath.Join(workspace, ".cron", "jobs.json"))

	loop := NewAgentLoop(
		messageBus,
		provider,
		workspace,
		"test-model",
		2,
		"",
		tools.WebFetchOptions{},
		config.ExecToolConfig{Timeout: 5},
		false,
		cronSvc,
		nil,
		false,
	)

	msg := bus.NewInboundMessage("telegram", "user-1", "chat-42", "hello")
	resp, err := loop.ProcessMessage(context.Background(), msg)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Contains(t, resp.Content, "Reached 2 iterations without completion.")
}

func TestAgentLoopProcessMessageMaxIterationAutoModeUsesExpandedBudget(t *testing.T) {
	workspace := t.TempDir()
	messageBus := bus.NewMessageBus(10)
	provider := &endlessToolProvider{}
	cronSvc := cron.NewService(filepath.Join(workspace, ".cron", "jobs.json"))

	loop := NewAgentLoop(
		messageBus,
		provider,
		workspace,
		"test-model",
		2,
		"",
		tools.WebFetchOptions{},
		config.ExecToolConfig{Timeout: 5},
		false,
		cronSvc,
		nil,
		false,
	)
	loop.UpdateRuntimeExecutionMode(config.ExecutionModeAuto)

	msg := bus.NewInboundMessage("telegram", "user-1", "chat-42", "hello")
	resp, err := loop.ProcessMessage(context.Background(), msg)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Contains(t, resp.Content, "Reached 10 iterations without completion.")
	assert.NotContains(t, resp.Content, "输入'继续'以恢复执行")
}

func TestAgentLoopAutoModeResumesPausedPlanWithoutContinue(t *testing.T) {
	workspace := t.TempDir()
	messageBus := bus.NewMessageBus(10)
	provider := &staticProvider{}
	cronSvc := cron.NewService(filepath.Join(workspace, ".cron", "jobs.json"))

	loop := NewAgentLoop(
		messageBus,
		provider,
		workspace,
		"test-model",
		2,
		"",
		tools.WebFetchOptions{},
		config.ExecToolConfig{Timeout: 5},
		false,
		cronSvc,
		nil,
		false,
	)
	loop.UpdateRuntimeExecutionMode(config.ExecutionModeAuto)

	plan := CreatePlan("auto resume test")
	plan.AddStep("step 1")
	plan.Steps[0].Status = StepStatusRunning
	plan.Status = PlanStatusPaused
	require.NoError(t, loop.PlanManager.Save("telegram:chat-42", plan))

	msg := bus.NewInboundMessage("telegram", "user-1", "chat-42", "继续处理")
	resp, err := loop.ProcessMessage(context.Background(), msg)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "ok", resp.Content)

	updatedPlan, err := loop.PlanManager.Load("telegram:chat-42")
	require.NoError(t, err)
	require.NotNil(t, updatedPlan)
	assert.Equal(t, PlanStatusCompleted, updatedPlan.Status)
}

func TestAgentLoopProcessMessageAutoConsolidatesWhenSessionLarge(t *testing.T) {
	workspace := t.TempDir()
	messageBus := bus.NewMessageBus(10)
	provider := &staticProvider{}
	cronSvc := cron.NewService(filepath.Join(workspace, ".cron", "jobs.json"))

	loop := NewAgentLoop(
		messageBus,
		provider,
		workspace,
		"test-model",
		2,
		"",
		tools.WebFetchOptions{},
		config.ExecToolConfig{Timeout: 5},
		false,
		cronSvc,
		nil,
		false,
	)

	sess := loop.sessions.GetOrCreate("telegram:chat-42")
	for i := 0; i < sessionConsolidateThreshold+5; i++ {
		sess.AddMessage("user", "context")
	}
	require.NoError(t, loop.sessions.Save(sess))

	msg := bus.NewInboundMessage("telegram", "user-1", "chat-42", "hello")
	resp, err := loop.ProcessMessage(context.Background(), msg)
	require.NoError(t, err)
	require.NotNil(t, resp)

	updated := loop.sessions.GetOrCreate("telegram:chat-42")
	assert.Greater(t, updated.LastConsolidated, 0)

	historyPath := filepath.Join(workspace, "memory", "HISTORY.md")
	body, readErr := os.ReadFile(historyPath)
	require.NoError(t, readErr)
	assert.Contains(t, string(body), "session: telegram:chat-42")
}

func TestAgentLoopProcessDirectEventStreamEmitsStructuredEvents(t *testing.T) {
	workspace := t.TempDir()
	messageBus := bus.NewMessageBus(10)
	provider := &streamEventProvider{}

	loop := NewAgentLoop(
		messageBus,
		provider,
		workspace,
		"test-model",
		3,
		"",
		tools.WebFetchOptions{},
		config.ExecToolConfig{Timeout: 5},
		false,
		nil,
		nil,
		false,
	)

	var events []StreamEvent
	resp, err := loop.ProcessDirectEventStream(
		context.Background(),
		"hello",
		"desktop:test",
		"desktop",
		"chat-1",
		func(event StreamEvent) {
			events = append(events, event)
		},
	)
	require.NoError(t, err)
	assert.Equal(t, "event stream ok", resp)
	require.NotEmpty(t, events)

	var hasStatus bool
	var hasToolStart bool
	var hasToolResult bool
	var hasDelta bool
	for _, event := range events {
		switch event.Type {
		case "status":
			hasStatus = true
		case "tool_start":
			hasToolStart = true
		case "tool_result":
			hasToolResult = true
		case "content_delta":
			hasDelta = true
		}
	}

	assert.True(t, hasStatus)
	assert.True(t, hasToolStart)
	assert.True(t, hasToolResult)
	assert.True(t, hasDelta)

	mgr := session.NewManager(workspace)
	sess := mgr.GetOrCreate("desktop:test")
	require.Len(t, sess.Messages, 2)
	require.NotEmpty(t, sess.Messages[1].Timeline)

	var timelineHasActivity bool
	var timelineHasText bool
	for _, entry := range sess.Messages[1].Timeline {
		if entry.Kind == "activity" {
			timelineHasActivity = true
		}
		if entry.Kind == "text" && entry.Text != "" {
			timelineHasText = true
		}
	}

	assert.True(t, timelineHasActivity)
	assert.True(t, timelineHasText)
}

func TestTruncateEventTextPreservesUTF8Boundaries(t *testing.T) {
	input := "从零开始理解🌟AI入门课程"
	truncated := truncateEventText(input, 7)

	assert.True(t, utf8.ValidString(truncated))
	assert.NotContains(t, truncated, "�")
	assert.Equal(t, "从零开始理解🌟...", truncated)
}

func TestProcessDirectWithSkillsUsesOnlySelectedSkills(t *testing.T) {
	workspace := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(workspace, "skills"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(workspace, "skills", "alpha.md"), []byte("# Alpha\nAlpha content"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(workspace, "skills", "beta.md"), []byte("# Beta\nBeta content"), 0644))

	provider := &captureSkillsProvider{}
	loop := NewAgentLoop(
		bus.NewMessageBus(10),
		provider,
		workspace,
		"test-model",
		2,
		"",
		tools.WebFetchOptions{},
		config.ExecToolConfig{Timeout: 5},
		false,
		nil,
		nil,
		false,
	)

	resp, err := loop.ProcessDirectWithSkills(context.Background(), "hello", "desktop:test", "desktop", "chat-1", []string{"alpha"})
	require.NoError(t, err)
	assert.Equal(t, "ok", resp)

	assert.Contains(t, provider.systemPrompt, "### Alpha")
	assert.NotContains(t, provider.systemPrompt, "### Beta")

	sessionMgr := session.NewManager(workspace)
	sess := sessionMgr.GetOrCreate("desktop:test")
	require.Len(t, sess.Messages, 2)
	assert.Equal(t, "hello", strings.TrimSpace(sess.Messages[0].Content))
	assert.NotContains(t, sess.Messages[0].Content, "@skill:")
}

func TestAgentLoop_PlanManagerIntegration(t *testing.T) {
	tmpDir := t.TempDir()
	pm := NewPlanManager(tmpDir)
	sessionKey := "test:session"

	// Test plan creation and saving
	plan := CreatePlan("test goal")
	plan.AddStep("step 1")
	plan.Steps[0].Status = StepStatusRunning
	now := time.Now()
	plan.Steps[0].StartedAt = &now

	err := pm.Save(sessionKey, plan)
	if err != nil {
		t.Fatalf("failed to save plan: %v", err)
	}

	// Verify plan can be loaded
	loaded, err := pm.Load(sessionKey)
	if err != nil {
		t.Fatalf("failed to load plan: %v", err)
	}

	if loaded.Goal != "test goal" {
		t.Errorf("expected goal 'test goal', got %s", loaded.Goal)
	}

	// Test Exists
	if !pm.Exists(sessionKey) {
		t.Error("expected plan to exist")
	}

	// Test Delete
	err = pm.Delete(sessionKey)
	if err != nil {
		t.Fatalf("failed to delete plan: %v", err)
	}

	if pm.Exists(sessionKey) {
		t.Error("expected plan to not exist after delete")
	}
}
