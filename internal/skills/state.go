package skills

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// StateManager 管理技能的启用/禁用状态
type StateManager struct {
	mu        sync.RWMutex
	states    map[string]bool // skill name -> enabled
	storePath string
}

// NewStateManager 创建状态管理器
func NewStateManager(storePath string) *StateManager {
	sm := &StateManager{
		states:    make(map[string]bool),
		storePath: storePath,
	}
	sm.load()
	return sm
}

// IsEnabled 检查技能是否启用（默认启用）
func (sm *StateManager) IsEnabled(skillName string) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	// 如果没有记录，默认启用
	enabled, exists := sm.states[skillName]
	if !exists {
		return true
	}
	return enabled
}

// SetEnabled 设置技能启用状态
func (sm *StateManager) SetEnabled(skillName string, enabled bool) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.states[skillName] = enabled
	return sm.save()
}

// GetAllStates 获取所有技能状态
func (sm *StateManager) GetAllStates() map[string]bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	result := make(map[string]bool, len(sm.states))
	for k, v := range sm.states {
		result[k] = v
	}
	return result
}

// FilterEnabled 过滤出启用的技能
func (sm *StateManager) FilterEnabled(entries []Entry) []Entry {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var filtered []Entry
	for _, entry := range entries {
		// 默认启用，除非明确禁用
		if enabled, exists := sm.states[entry.Name]; !exists || enabled {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

// load 从文件加载状态
func (sm *StateManager) load() error {
	if sm.storePath == "" {
		return nil
	}

	data, err := os.ReadFile(sm.storePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	return json.Unmarshal(data, &sm.states)
}

// save 保存状态到文件
func (sm *StateManager) save() error {
	if sm.storePath == "" {
		return nil
	}

	dir := filepath.Dir(sm.storePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(sm.states, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(sm.storePath, data, 0644)
}
