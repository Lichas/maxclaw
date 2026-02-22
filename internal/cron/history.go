package cron

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// HistoryStore 执行历史存储
type HistoryStore struct {
	records   []ExecutionRecord
	mu        sync.RWMutex
	storePath string
	maxSize   int
}

// NewHistoryStore 创建历史存储
func NewHistoryStore(storePath string) *HistoryStore {
	h := &HistoryStore{
		storePath: storePath,
		maxSize:   1000, // Keep last 1000 records
		records:   make([]ExecutionRecord, 0),
	}
	h.load()
	return h
}

// AddRecord 添加执行记录
func (h *HistoryStore) AddRecord(record ExecutionRecord) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Assign ID if not set
	if record.ID == "" {
		record.ID = fmt.Sprintf("exec_%d", time.Now().UnixNano())
	}

	h.records = append(h.records, record)

	// Trim old records
	if len(h.records) > h.maxSize {
		h.records = h.records[len(h.records)-h.maxSize:]
	}

	h.save()
}

// UpdateRecord 更新执行记录
func (h *HistoryStore) UpdateRecord(id string, updates func(*ExecutionRecord)) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for i := range h.records {
		if h.records[i].ID == id {
			updates(&h.records[i])
			h.save()
			return
		}
	}
}

// GetRecords 获取执行记录
func (h *HistoryStore) GetRecords(jobID string, limit int) []ExecutionRecord {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var result []ExecutionRecord
	for i := len(h.records) - 1; i >= 0; i-- {
		if jobID == "" || h.records[i].JobID == jobID {
			result = append(result, h.records[i])
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}
	return result
}

// GetRecord 获取单条记录
func (h *HistoryStore) GetRecord(id string) (*ExecutionRecord, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, r := range h.records {
		if r.ID == id {
			return &r, true
		}
	}
	return nil, false
}

func (h *HistoryStore) load() {
	data, err := os.ReadFile(h.storePath)
	if err != nil {
		return
	}
	json.Unmarshal(data, &h.records)
}

func (h *HistoryStore) save() error {
	dir := filepath.Dir(h.storePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(h.records, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(h.storePath, data, 0644)
}
