package agent

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Lichas/nanobot-go/internal/bus"
	"github.com/Lichas/nanobot-go/internal/config"
	"github.com/Lichas/nanobot-go/internal/cron"
	"github.com/Lichas/nanobot-go/internal/providers"
	"github.com/Lichas/nanobot-go/internal/session"
	"github.com/Lichas/nanobot-go/pkg/tools"
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
	)

	msg := bus.NewInboundMessage("telegram", "user-1", "chat-42", "hello")
	resp, err := loop.ProcessMessage(context.Background(), msg)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "Reached 2 iterations without completion.", resp.Content)
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
