package agent

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/Lichas/nanobot-go/internal/bus"
	"github.com/Lichas/nanobot-go/internal/config"
	"github.com/Lichas/nanobot-go/internal/cron"
	"github.com/Lichas/nanobot-go/internal/providers"
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
