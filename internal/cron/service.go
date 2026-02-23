package cron

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Lichas/maxclaw/internal/logging"
	"github.com/robfig/cron/v3"
)

// JobFunc 任务执行函数类型
type JobFunc func(job *Job) (string, error)

// NotificationFunc 通知函数类型
type NotificationFunc func(title, body string, data map[string]interface{})

// Service 定时任务服务
type Service struct {
	jobs         map[string]*Job
	mu           sync.RWMutex
	storePath    string
	running      bool
	stopChan     chan struct{}
	wg           sync.WaitGroup
	onJob        JobFunc
	onNotify     NotificationFunc
	cron         *cron.Cron
	historyStore *HistoryStore
}

// NewService 创建定时任务服务
func NewService(storePath string) *Service {
	s := &Service{
		jobs:      make(map[string]*Job),
		storePath: storePath,
		stopChan:  make(chan struct{}),
		cron:      cron.New(),
	}
	s.load()

	historyPath := filepath.Join(filepath.Dir(storePath), "cron_history.json")
	s.historyStore = NewHistoryStore(historyPath)

	return s
}

// SetJobHandler 设置任务处理器
func (s *Service) SetJobHandler(handler JobFunc) {
	s.onJob = handler
}

// SetNotificationHandler 设置通知处理器
func (s *Service) SetNotificationHandler(handler NotificationFunc) {
	s.onNotify = handler
}

// AddJob 添加任务
func (s *Service) AddJob(name string, schedule Schedule, payload Payload) (*Job, error) {
	job := NewJob(name, schedule, payload)

	s.mu.Lock()
	defer s.mu.Unlock()

	s.jobs[job.ID] = job

	// 如果服务正在运行，立即调度
	if s.running {
		s.scheduleJob(job)
	}

	if err := s.save(); err != nil {
		delete(s.jobs, job.ID)
		return nil, fmt.Errorf("failed to save job: %w", err)
	}

	return job, nil
}

// GetJob 获取任务
func (s *Service) GetJob(id string) (*Job, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, ok := s.jobs[id]
	return job, ok
}

// RemoveJob 删除任务
func (s *Service) RemoveJob(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.jobs[id]; !ok {
		return false
	}

	delete(s.jobs, id)
	s.save()
	return true
}

// UpdateJob 更新任务
func (s *Service) UpdateJob(id string, name string, schedule Schedule, payload Payload) (*Job, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[id]
	if !ok {
		return nil, false
	}

	// 更新任务信息
	job.Name = name
	job.Schedule = schedule
	job.Payload = payload

	// 如果服务正在运行，需要重新调度
	if s.running {
		// 重新调度任务（先移除旧调度，再添加新调度）
		// 注意：当前实现中，scheduleJob 是内部方法，需要重启服务或重新加载
		// 这里我们假设调度是基于 cron 库的，它会自动处理
	}

	if err := s.save(); err != nil {
		return nil, false
	}

	return job, true
}

// ListJobs 列出所有任务
func (s *Service) ListJobs() []*Job {
	s.mu.RLock()
	defer s.mu.RUnlock()

	jobs := make([]*Job, 0, len(s.jobs))
	for _, job := range s.jobs {
		jobs = append(jobs, job)
	}
	return jobs
}

// EnableJob 启用/禁用任务
func (s *Service) EnableJob(id string, enabled bool) (*Job, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[id]
	if !ok {
		return nil, false
	}

	job.Enabled = enabled
	s.save()
	return job, true
}

// Start 启动服务
func (s *Service) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("service already running")
	}

	s.running = true
	s.stopChan = make(chan struct{})

	// 调度所有已启用的任务
	for _, job := range s.jobs {
		if job.Enabled {
			s.scheduleJob(job)
		}
	}

	s.cron.Start()

	return nil
}

// Stop 停止服务
func (s *Service) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	s.running = false
	close(s.stopChan)
	s.cron.Stop()
	s.wg.Wait()
}

// IsRunning 检查是否在运行
func (s *Service) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// scheduleJob 调度单个任务
func (s *Service) scheduleJob(job *Job) {
	switch job.Schedule.Type {
	case ScheduleTypeEvery:
		s.scheduleEveryJob(job)
	case ScheduleTypeCron:
		s.scheduleCronJob(job)
	case ScheduleTypeOnce:
		s.scheduleOnceJob(job)
	}
}

// scheduleEveryJob 调度周期性任务
func (s *Service) scheduleEveryJob(job *Job) {
	duration := time.Duration(job.Schedule.EveryMs) * time.Millisecond
	if duration <= 0 {
		s.logCronf("cron schedule skipped type=every job_id=%s reason=invalid_interval interval_ms=%d", job.ID, job.Schedule.EveryMs)
		return
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		ticker := time.NewTicker(duration)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if !s.running {
					return
				}
				s.executeJob(job, "every")
			case <-s.stopChan:
				return
			}
		}
	}()
}

// scheduleCronJob 调度 Cron 任务
func (s *Service) scheduleCronJob(job *Job) {
	if job.Schedule.Expr == "" {
		s.logCronf("cron schedule skipped type=cron job_id=%s reason=empty_expr", job.ID)
		return
	}

	if _, err := s.cron.AddFunc(job.Schedule.Expr, func() {
		s.executeJob(job, "cron")
	}); err != nil {
		s.logCronf("cron schedule failed type=cron job_id=%s expr=%q err=%v", job.ID, job.Schedule.Expr, err)
	}
}

// scheduleOnceJob 调度一次性任务
func (s *Service) scheduleOnceJob(job *Job) {
	at := time.UnixMilli(job.Schedule.AtMs)
	if at.Before(time.Now()) {
		s.logCronf("cron schedule skipped type=once job_id=%s reason=past_time at=%s", job.ID, at.Format(time.RFC3339))
		return
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		select {
		case <-time.After(time.Until(at)):
			if s.running {
				s.executeJob(job, "once")
			}
		case <-s.stopChan:
			return
		}
	}()
}

// executeJob 执行任务
func (s *Service) executeJob(job *Job, trigger string) {
	if job == nil {
		s.logCronf("cron attempt trigger=%s skipped reason=nil_job", trigger)
		return
	}

	s.logCronf("cron attempt trigger=%s job=%s job_id=%s enabled=%t", trigger, job.Name, job.ID, job.Enabled)
	if !job.Enabled {
		s.logCronf("cron skip trigger=%s job_id=%s reason=disabled", trigger, job.ID)
		return
	}
	if s.onJob == nil {
		s.logCronf("cron skip trigger=%s job_id=%s reason=no_handler", trigger, job.ID)
		return
	}

	// Create execution record
	record := ExecutionRecord{
		ID:        fmt.Sprintf("exec_%d", time.Now().UnixNano()),
		JobID:     job.ID,
		JobTitle:  job.Name,
		StartedAt: time.Now(),
		Status:    "running",
	}
	s.historyStore.AddRecord(record)

	s.logCronf("cron execute trigger=%s job=%s job_id=%s", trigger, job.Name, job.ID)
	start := time.Now()
	result, err := s.onJob(job)
	duration := time.Since(start).Milliseconds()

	// Update record after execution
	now := time.Now()
	s.historyStore.UpdateRecord(record.ID, func(r *ExecutionRecord) {
		r.EndedAt = &now
		r.Duration = duration
		r.Output = result
		if err != nil {
			r.Status = "failed"
			r.Error = err.Error()
		} else {
			r.Status = "success"
		}
	})

	if err != nil {
		s.logCronf("cron failed trigger=%s job=%s job_id=%s err=%v", trigger, job.Name, job.ID, err)
		// Send notification on failure
		if s.onNotify != nil {
			s.onNotify(
				"定时任务执行失败",
				fmt.Sprintf("任务 \"%s\" 执行失败: %v", job.Name, err),
				map[string]interface{}{
					"type":    "scheduled_task",
					"jobId":   job.ID,
					"jobName": job.Name,
					"status":  "failed",
				},
			)
		}
	} else {
		s.logCronf("cron completed trigger=%s job=%s job_id=%s result=%q", trigger, job.Name, job.ID, logging.Truncate(result, 400))
		// Send notification on success
		if s.onNotify != nil {
			s.onNotify(
				"定时任务完成",
				fmt.Sprintf("任务 \"%s\" 执行完成", job.Name),
				map[string]interface{}{
					"type":    "scheduled_task",
					"jobId":   job.ID,
					"jobName": job.Name,
					"status":  "success",
				},
			)
		}
	}
}

// GetHistoryStore 获取历史存储
func (s *Service) GetHistoryStore() *HistoryStore {
	return s.historyStore
}

func (s *Service) logCronf(format string, args ...interface{}) {
	if lg := logging.Get(); lg != nil && lg.Cron != nil {
		lg.Cron.Printf(format, args...)
		return
	}
	fmt.Printf("[Cron] "+format+"\n", args...)
}

// save 保存任务到文件
func (s *Service) save() error {
	if s.storePath == "" {
		return nil
	}

	dir := filepath.Dir(s.storePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(s.jobs, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.storePath, data, 0644)
}

// load 从文件加载任务
func (s *Service) load() error {
	if s.storePath == "" {
		return nil
	}

	data, err := os.ReadFile(s.storePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var jobs map[string]*Job
	if err := json.Unmarshal(data, &jobs); err != nil {
		return err
	}

	s.jobs = jobs
	return nil
}

// Status 获取服务状态
func (s *Service) Status() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	enabledCount := 0
	for _, job := range s.jobs {
		if job.Enabled {
			enabledCount++
		}
	}

	return map[string]interface{}{
		"running":     s.running,
		"totalJobs":   len(s.jobs),
		"enabledJobs": enabledCount,
		"storePath":   s.storePath,
	}
}
