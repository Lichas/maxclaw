package cron

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Lichas/nanobot-go/internal/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewService(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "jobs.json")

	service := NewService(storePath)
	require.NotNil(t, service)

	status := service.Status()
	assert.False(t, status["running"].(bool))
	assert.Equal(t, 0, status["totalJobs"])
	assert.Equal(t, storePath, status["storePath"])
}

func TestAddJob(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "jobs.json")

	service := NewService(storePath)

	// 添加周期性任务
	schedule := Schedule{
		Type:    ScheduleTypeEvery,
		EveryMs: 1000,
	}
	payload := Payload{
		Message: "Test message",
		Channel: "test",
		Deliver: true,
	}

	job, err := service.AddJob("Test Job", schedule, payload)
	require.NoError(t, err)
	require.NotNil(t, job)

	assert.Equal(t, "Test Job", job.Name)
	assert.Equal(t, ScheduleTypeEvery, job.Schedule.Type)
	assert.Equal(t, int64(1000), job.Schedule.EveryMs)
	assert.Equal(t, "Test message", job.Payload.Message)
	assert.True(t, job.Enabled)
	assert.NotEmpty(t, job.ID)

	// 验证任务已保存
	jobs := service.ListJobs()
	assert.Len(t, jobs, 1)
}

func TestGetJob(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "jobs.json")

	service := NewService(storePath)

	schedule := Schedule{Type: ScheduleTypeEvery, EveryMs: 1000}
	payload := Payload{Message: "Test"}
	job, _ := service.AddJob("Test", schedule, payload)

	// 获取存在的任务
	found, ok := service.GetJob(job.ID)
	assert.True(t, ok)
	assert.Equal(t, job.Name, found.Name)

	// 获取不存在的任务
	_, ok = service.GetJob("non-existent")
	assert.False(t, ok)
}

func TestRemoveJob(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "jobs.json")

	service := NewService(storePath)

	schedule := Schedule{Type: ScheduleTypeEvery, EveryMs: 1000}
	payload := Payload{Message: "Test"}
	job, _ := service.AddJob("Test", schedule, payload)

	// 删除存在的任务
	assert.True(t, service.RemoveJob(job.ID))
	assert.Len(t, service.ListJobs(), 0)

	// 删除不存在的任务
	assert.False(t, service.RemoveJob("non-existent"))
}

func TestEnableJob(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "jobs.json")

	service := NewService(storePath)

	schedule := Schedule{Type: ScheduleTypeEvery, EveryMs: 1000}
	payload := Payload{Message: "Test"}
	job, _ := service.AddJob("Test", schedule, payload)
	assert.True(t, job.Enabled)

	// 禁用任务
	updated, ok := service.EnableJob(job.ID, false)
	assert.True(t, ok)
	assert.False(t, updated.Enabled)

	// 启用任务
	updated, ok = service.EnableJob(job.ID, true)
	assert.True(t, ok)
	assert.True(t, updated.Enabled)

	// 不存在的任务
	_, ok = service.EnableJob("non-existent", false)
	assert.False(t, ok)
}

func TestJobPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "jobs.json")

	// 创建服务并添加任务
	service1 := NewService(storePath)
	schedule := Schedule{Type: ScheduleTypeEvery, EveryMs: 5000}
	payload := Payload{Message: "Persistent"}
	job, err := service1.AddJob("Persistent Job", schedule, payload)
	require.NoError(t, err)

	// 创建新服务实例，验证任务已加载
	service2 := NewService(storePath)
	loaded, ok := service2.GetJob(job.ID)
	assert.True(t, ok)
	assert.Equal(t, job.Name, loaded.Name)
	assert.Equal(t, job.Schedule.EveryMs, loaded.Schedule.EveryMs)
	assert.Equal(t, job.Payload.Message, loaded.Payload.Message)
}

func TestCronJobTypes(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "jobs.json")
	service := NewService(storePath)

	// Every 类型
	everyJob, err := service.AddJob("Every Job", Schedule{
		Type:    ScheduleTypeEvery,
		EveryMs: 60000,
	}, Payload{Message: "Every"})
	require.NoError(t, err)
	assert.Equal(t, ScheduleTypeEvery, everyJob.Schedule.Type)

	// Cron 类型
	cronJob, err := service.AddJob("Cron Job", Schedule{
		Type: ScheduleTypeCron,
		Expr: "0 9 * * *",
	}, Payload{Message: "Cron"})
	require.NoError(t, err)
	assert.Equal(t, ScheduleTypeCron, cronJob.Schedule.Type)
	assert.Equal(t, "0 9 * * *", cronJob.Schedule.Expr)

	// Once 类型
	futureTime := time.Now().Add(time.Hour).UnixMilli()
	onceJob, err := service.AddJob("Once Job", Schedule{
		Type: ScheduleTypeOnce,
		AtMs: futureTime,
	}, Payload{Message: "Once"})
	require.NoError(t, err)
	assert.Equal(t, ScheduleTypeOnce, onceJob.Schedule.Type)
	assert.Equal(t, futureTime, onceJob.Schedule.AtMs)
}

func TestJobGetNextRun(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		job       *Job
		wantOk    bool
		wantAfter time.Time
	}{
		{
			name: "every job",
			job: &Job{
				Enabled:  true,
				Schedule: Schedule{Type: ScheduleTypeEvery, EveryMs: 60000},
			},
			wantOk: true,
		},
		{
			name: "disabled job",
			job: &Job{
				Enabled:  false,
				Schedule: Schedule{Type: ScheduleTypeEvery, EveryMs: 60000},
			},
			wantOk: false,
		},
		{
			name: "once job in future",
			job: &Job{
				Enabled:  true,
				Schedule: Schedule{Type: ScheduleTypeOnce, AtMs: now.Add(time.Hour).UnixMilli()},
			},
			wantOk: true,
		},
		{
			name: "once job in past",
			job: &Job{
				Enabled:  true,
				Schedule: Schedule{Type: ScheduleTypeOnce, AtMs: now.Add(-time.Hour).UnixMilli()},
			},
			wantOk: false,
		},
		{
			name: "invalid every",
			job: &Job{
				Enabled:  true,
				Schedule: Schedule{Type: ScheduleTypeEvery, EveryMs: 0},
			},
			wantOk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			next, ok := tt.job.GetNextRun()
			assert.Equal(t, tt.wantOk, ok)
			if tt.wantOk {
				assert.False(t, next.IsZero())
			}
		})
	}
}

func TestServiceStartStop(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "jobs.json")

	service := NewService(storePath)

	// 添加测试任务
	schedule := Schedule{Type: ScheduleTypeEvery, EveryMs: 100}
	payload := Payload{Message: "Test"}
	service.AddJob("Test", schedule, payload)

	// 设置处理器
	executed := make(chan bool, 1)
	service.SetJobHandler(func(job *Job) (string, error) {
		executed <- true
		return "done", nil
	})

	// 启动服务
	err := service.Start()
	require.NoError(t, err)
	assert.True(t, service.IsRunning())

	// 停止服务
	service.Stop()
	assert.False(t, service.IsRunning())
}

func TestServiceWithEmptyStorePath(t *testing.T) {
	service := NewService("")

	// 添加任务（不应该保存到文件）
	schedule := Schedule{Type: ScheduleTypeEvery, EveryMs: 1000}
	payload := Payload{Message: "Test"}
	job, err := service.AddJob("Test", schedule, payload)
	require.NoError(t, err)
	assert.NotNil(t, job)

	// 验证任务在内存中
	assert.Len(t, service.ListJobs(), 1)

	// 创建新服务实例（同样没有 storePath）
	service2 := NewService("")
	// 不应该加载之前的任务
	assert.Len(t, service2.ListJobs(), 0)
}

func TestLoadCorruptedFile(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "jobs.json")

	// 写入损坏的 JSON
	err := os.WriteFile(storePath, []byte("not valid json"), 0644)
	require.NoError(t, err)

	// 创建服务应该返回错误但不应崩溃
	service := NewService(storePath)
	assert.NotNil(t, service)
	assert.Len(t, service.ListJobs(), 0)
}

func TestExecuteJobLogsAttemptAndCompletion(t *testing.T) {
	lg, err := logging.Init(t.TempDir())
	require.NoError(t, err)
	require.NotNil(t, lg)
	require.NotNil(t, lg.Cron)

	var buf bytes.Buffer
	lg.Cron.SetOutput(&buf)

	service := NewService("")
	service.SetJobHandler(func(job *Job) (string, error) {
		return "ok", nil
	})

	job := NewJob("log-test", Schedule{Type: ScheduleTypeEvery, EveryMs: 1000}, Payload{Message: "m"})
	service.executeJob(job, "every")

	logText := buf.String()
	assert.Contains(t, logText, "cron attempt trigger=every")
	assert.Contains(t, logText, "cron execute trigger=every")
	assert.Contains(t, logText, "cron completed trigger=every")
	assert.Contains(t, logText, job.ID)
}

func TestExecuteJobLogsSkipReasons(t *testing.T) {
	lg, err := logging.Init(t.TempDir())
	require.NoError(t, err)
	require.NotNil(t, lg)
	require.NotNil(t, lg.Cron)

	var buf bytes.Buffer
	lg.Cron.SetOutput(&buf)

	service := NewService("")

	disabledJob := NewJob("disabled", Schedule{Type: ScheduleTypeEvery, EveryMs: 1000}, Payload{})
	disabledJob.Enabled = false
	service.executeJob(disabledJob, "cron")

	noHandlerJob := NewJob("no-handler", Schedule{Type: ScheduleTypeEvery, EveryMs: 1000}, Payload{})
	service.executeJob(noHandlerJob, "cron")

	logText := buf.String()
	assert.Contains(t, logText, "cron attempt trigger=cron")
	assert.True(t, strings.Contains(logText, "reason=disabled"))
	assert.True(t, strings.Contains(logText, "reason=no_handler"))
}
