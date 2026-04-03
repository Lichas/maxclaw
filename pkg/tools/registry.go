package tools

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// Registry 工具注册表
type Registry struct {
	tools map[string]Tool
	mu    sync.RWMutex
}

// NewRegistry 创建工具注册表
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

// Register 注册工具
func (r *Registry) Register(tool Tool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if tool == nil {
		return fmt.Errorf("tool cannot be nil")
	}

	name := tool.Name()
	if name == "" {
		return fmt.Errorf("tool name cannot be empty")
	}

	r.tools[name] = tool
	return nil
}

// Get 获取工具
func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tool, exists := r.tools[name]
	return tool, exists
}

// Execute 执行工具
func (r *Registry) Execute(ctx context.Context, name string, params map[string]interface{}) (string, error) {
	tool, exists := r.Get(name)
	if !exists {
		return "", fmt.Errorf("tool not found: %s", name)
	}

	// 验证参数
	if err := ValidateParams(tool.Parameters(), params); err != nil {
		return fmt.Sprintf("Invalid parameters: %s", err.Error()), nil
	}

	return tool.Execute(ctx, params)
}

// schemaTool 能够生成 OpenAI Schema 的工具接口
type schemaTool interface {
	Tool
	ToOpenAISchema() map[string]interface{}
}

// GetDefinitions 获取所有工具定义（OpenAI 格式）
func (r *Registry) GetDefinitions() []map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	definitions := make([]map[string]interface{}, 0, len(r.tools))
	for _, tool := range r.tools {
		if st, ok := tool.(schemaTool); ok {
			definitions = append(definitions, st.ToOpenAISchema())
		}
	}
	return definitions
}

// List 列出所有工具名称
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}

// RemoveByPrefix removes all tools whose names start with the given prefix.
func (r *Registry) RemoveByPrefix(prefix string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for name := range r.tools {
		if prefix == "" || strings.HasPrefix(name, prefix) {
			delete(r.tools, name)
		}
	}
}

// Count 返回工具数量
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.tools)
}
