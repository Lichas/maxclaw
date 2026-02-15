package tools

import (
	"context"
	"testing"

	"github.com/Lichas/nanobot-go/internal/cron"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockCronService 用于测试的 mock 服务
type MockCronService struct {
	jobs      map[string]*cron.Job
	addCalled bool
}

func NewMockCronService() *MockCronService {
	return &MockCronService{
		jobs: make(map[string]*cron.Job),
	}
}

func (m *MockCronService) AddJob(name string, schedule cron.Schedule, payload cron.Payload) (*cron.Job, error) {
	m.addCalled = true
	job := cron.NewJob(name, schedule, payload)
	m.jobs[job.ID] = job
	return job, nil
}

func (m *MockCronService) RemoveJob(id string) bool {
	if _, ok := m.jobs[id]; ok {
		delete(m.jobs, id)
		return true
	}
	return false
}

func (m *MockCronService) ListJobs() []*cron.Job {
	jobs := make([]*cron.Job, 0, len(m.jobs))
	for _, job := range m.jobs {
		jobs = append(jobs, job)
	}
	return jobs
}

func TestCronToolAdd(t *testing.T) {
	mockService := NewMockCronService()
	tool := NewCronTool(mockService)
	tool.SetContext("telegram", "123456")
	ctx := context.Background()

	t.Run("add with every_seconds", func(t *testing.T) {
		result, err := tool.Execute(ctx, map[string]interface{}{
			"action":        "add",
			"message":       "Take a break",
			"every_seconds": 3600,
		})
		require.NoError(t, err)
		assert.Contains(t, result, "Created job")
		assert.Contains(t, result, "Take a break")
		assert.Contains(t, result, "id:")
	})

	t.Run("add with cron_expr", func(t *testing.T) {
		result, err := tool.Execute(ctx, map[string]interface{}{
			"action":    "add",
			"message":   "Daily report",
			"cron_expr": "0 9 * * *",
		})
		require.NoError(t, err)
		assert.Contains(t, result, "Created job")
		assert.Contains(t, result, "Daily report")
		assert.Contains(t, result, "id:")
	})

	t.Run("missing message", func(t *testing.T) {
		_, err := tool.Execute(ctx, map[string]interface{}{
			"action":        "add",
			"every_seconds": 3600,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "message is required")
	})

	t.Run("missing schedule", func(t *testing.T) {
		_, err := tool.Execute(ctx, map[string]interface{}{
			"action":  "add",
			"message": "No schedule",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "every_seconds or cron_expr is required")
	})

	t.Run("no context", func(t *testing.T) {
		toolNoContext := NewCronTool(mockService)
		_, err := toolNoContext.Execute(ctx, map[string]interface{}{
			"action":        "add",
			"message":       "Test",
			"every_seconds": 3600,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no session context")
	})

	t.Run("use runtime context", func(t *testing.T) {
		toolNoContext := NewCronTool(mockService)
		runtimeCtx := WithRuntimeContext(ctx, "whatsapp", "998877")
		_, err := toolNoContext.Execute(runtimeCtx, map[string]interface{}{
			"action":        "add",
			"message":       "Runtime context add",
			"every_seconds": 1800,
		})
		require.NoError(t, err)
	})
}

func TestCronToolList(t *testing.T) {
	mockService := NewMockCronService()
	tool := NewCronTool(mockService)
	ctx := context.Background()

	t.Run("list empty", func(t *testing.T) {
		result, err := tool.Execute(ctx, map[string]interface{}{
			"action": "list",
		})
		require.NoError(t, err)
		assert.Equal(t, "No scheduled jobs.", result)
	})
}

func TestCronToolRemove(t *testing.T) {
	mockService := NewMockCronService()
	tool := NewCronTool(mockService)
	ctx := context.Background()

	t.Run("remove without job_id", func(t *testing.T) {
		_, err := tool.Execute(ctx, map[string]interface{}{
			"action": "remove",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "job_id is required")
	})

	t.Run("remove nonexistent", func(t *testing.T) {
		_, err := tool.Execute(ctx, map[string]interface{}{
			"action": "remove",
			"job_id": "nonexistent",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestCronToolInvalidAction(t *testing.T) {
	mockService := NewMockCronService()
	tool := NewCronTool(mockService)
	ctx := context.Background()

	_, err := tool.Execute(ctx, map[string]interface{}{
		"action": "invalid",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown action")
}
