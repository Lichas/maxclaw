package tools

import (
	"context"
	"testing"
	"time"

	"github.com/Lichas/maxclaw/internal/cron"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockCronService 用于测试的 mock 服务
type MockCronService struct {
	jobs      map[string]*cron.Job
	addCalled bool
	lastAdded *cron.Job
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
	m.lastAdded = job
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

	t.Run("add with at", func(t *testing.T) {
		result, err := tool.Execute(ctx, map[string]interface{}{
			"action":  "add",
			"message": "One-shot reminder",
			"at":      "2099-01-01T10:30:00",
		})
		require.NoError(t, err)
		assert.Contains(t, result, "Created job")
		assert.Contains(t, result, "at:")
		require.NotNil(t, mockService.lastAdded)
		assert.Equal(t, cron.ScheduleTypeOnce, mockService.lastAdded.Schedule.Type)
		assert.NotZero(t, mockService.lastAdded.Schedule.AtMs)
	})

	t.Run("add with at time-only", func(t *testing.T) {
		at := time.Now().Add(2 * time.Minute).Format("15:04:05")
		result, err := tool.Execute(ctx, map[string]interface{}{
			"action":  "add",
			"message": "Time only reminder",
			"at":      at,
		})
		require.NoError(t, err)
		assert.Contains(t, result, "Created job")
		require.NotNil(t, mockService.lastAdded)
		assert.Equal(t, cron.ScheduleTypeOnce, mockService.lastAdded.Schedule.Type)
		assert.Greater(t, mockService.lastAdded.Schedule.AtMs, time.Now().UnixMilli())
	})

	t.Run("invalid at", func(t *testing.T) {
		_, err := tool.Execute(ctx, map[string]interface{}{
			"action":  "add",
			"message": "Bad at",
			"at":      "tomorrow morning",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid at")
	})

	t.Run("at in the past", func(t *testing.T) {
		pastAt := time.Now().Add(-2 * time.Hour).Format("2006-01-02 15:04:05")
		_, err := tool.Execute(ctx, map[string]interface{}{
			"action":  "add",
			"message": "Past at",
			"at":      pastAt,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at must be in the future")
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
		assert.Contains(t, err.Error(), "every_seconds, cron_expr, or at is required")
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

	t.Run("list includes once schedule", func(t *testing.T) {
		_, err := mockService.AddJob("Once", cron.Schedule{
			Type: cron.ScheduleTypeOnce,
			AtMs: time.Now().Add(time.Hour).UnixMilli(),
		}, cron.Payload{Message: "Once"})
		require.NoError(t, err)

		result, err := tool.Execute(ctx, map[string]interface{}{
			"action": "list",
		})
		require.NoError(t, err)
		assert.Contains(t, result, "at:")
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
