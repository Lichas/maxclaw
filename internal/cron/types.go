package cron

import (
	"encoding/json"
	"fmt"
	"time"
)

// ExecutionRecord 任务执行记录
type ExecutionRecord struct {
	ID        string     `json:"id"`
	JobID     string     `json:"jobId"`
	JobTitle  string     `json:"jobTitle"`
	StartedAt time.Time  `json:"startedAt"`
	EndedAt   *time.Time `json:"endedAt,omitempty"`
	Status    string     `json:"status"` // running, success, failed
	Output    string     `json:"output"`
	Error     string     `json:"error,omitempty"`
	Duration  int64      `json:"durationMs"` // milliseconds
}

// ScheduleType 调度类型
type ScheduleType string

const (
	// ScheduleTypeEvery 每隔一段时间执行
	ScheduleTypeEvery ScheduleType = "every"
	// ScheduleTypeCron Cron 表达式
	ScheduleTypeCron ScheduleType = "cron"
	// ScheduleTypeOnce 一次性任务
	ScheduleTypeOnce ScheduleType = "once"
)

// Schedule 任务调度配置
type Schedule struct {
	Type    ScheduleType `json:"type"`
	EveryMs int64        `json:"everyMs,omitempty"` // 每隔多少毫秒（ScheduleTypeEvery）
	Expr    string       `json:"expr,omitempty"`    // Cron 表达式（ScheduleTypeCron）
	AtMs    int64        `json:"atMs,omitempty"`    // 执行时间戳（ScheduleTypeOnce）
}

// Payload 任务负载
type Payload struct {
	Message string `json:"message"`           // 发送给 Agent 的消息
	Channel string `json:"channel,omitempty"` // 输出频道（可选）
	To      string `json:"to,omitempty"`      // 接收者（可选）
	Deliver bool   `json:"deliver"`           // 是否发送结果到频道
}

// Job 定时任务
type Job struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Schedule Schedule `json:"schedule"`
	Payload  Payload  `json:"payload"`
	Enabled  bool     `json:"enabled"`
	Created  int64    `json:"created"`
}

// NewJob 创建新任务
func NewJob(name string, schedule Schedule, payload Payload) *Job {
	return &Job{
		ID:       generateJobID(),
		Name:     name,
		Schedule: schedule,
		Payload:  payload,
		Enabled:  true,
		Created:  time.Now().UnixMilli(),
	}
}

// generateJobID 生成任务 ID
func generateJobID() string {
	return fmt.Sprintf("job_%d", time.Now().UnixNano())
}

// GetNextRun 获取下次执行时间
func (j *Job) GetNextRun() (time.Time, bool) {
	if !j.Enabled {
		return time.Time{}, false
	}

	switch j.Schedule.Type {
	case ScheduleTypeEvery:
		if j.Schedule.EveryMs <= 0 {
			return time.Time{}, false
		}
		return time.Now().Add(time.Duration(j.Schedule.EveryMs) * time.Millisecond), true

	case ScheduleTypeCron:
		// 简单的 cron 解析，实际需要 cron 库
		return time.Now().Add(time.Minute), true

	case ScheduleTypeOnce:
		at := time.UnixMilli(j.Schedule.AtMs)
		if at.Before(time.Now()) {
			return time.Time{}, false
		}
		return at, true

	default:
		return time.Time{}, false
	}
}

// ShouldRun 检查是否应该执行
func (j *Job) ShouldRun() bool {
	next, ok := j.GetNextRun()
	if !ok {
		return false
	}
	return time.Now().After(next) || time.Now().Equal(next)
}

// MarshalJSON 序列化
func (j *Job) MarshalJSON() ([]byte, error) {
	type Alias Job
	return json.Marshal(&struct {
		*Alias
	}{
		Alias: (*Alias)(j),
	})
}
