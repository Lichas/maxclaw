package agent

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Lichas/nanobot-go/internal/bus"
	"github.com/Lichas/nanobot-go/internal/providers"
)

//go:embed prompts/system_prompt.md
var systemPromptTemplate string

//go:embed prompts/environment.md
var environmentTemplate string

// ContextBuilder 上下文构建器
type ContextBuilder struct {
	workspace string
}

// NewContextBuilder 创建上下文构建器
func NewContextBuilder(workspace string) *ContextBuilder {
	return &ContextBuilder{workspace: workspace}
}

// BuildMessages 构建消息列表
func (b *ContextBuilder) BuildMessages(history []providers.Message, currentMessage string, media *bus.MediaAttachment, channel, chatID string) []providers.Message {
	messages := make([]providers.Message, 0)

	// 系统提示
	systemPrompt := b.buildSystemPrompt(channel, chatID, currentMessage)
	messages = append(messages, providers.Message{
		Role:    "system",
		Content: systemPrompt,
	})

	// 历史消息
	messages = append(messages, history...)

	// 当前消息
	content := currentMessage
	if media != nil {
		content = fmt.Sprintf("[Media: %s] %s", media.Type, content)
	}
	messages = append(messages, providers.Message{
		Role:    "user",
		Content: content,
	})

	return messages
}

// AddAssistantMessage 添加助手消息
func (b *ContextBuilder) AddAssistantMessage(messages []providers.Message, content string, toolCalls []providers.ToolCall) []providers.Message {
	msg := providers.Message{
		Role:    "assistant",
		Content: content,
	}
	// 如果有工具调用，正确设置
	if len(toolCalls) > 0 {
		msg.ToolCalls = toolCalls
	}
	messages = append(messages, msg)
	return messages
}

// AddToolResult 添加工具结果
func (b *ContextBuilder) AddToolResult(messages []providers.Message, toolCallID, name, result string) []providers.Message {
	messages = append(messages, providers.Message{
		Role:       "tool",
		Content:    result,
		ToolCallID: toolCallID,
	})
	return messages
}

// buildSystemPrompt 构建系统提示
func (b *ContextBuilder) buildSystemPrompt(channel, chatID, currentMessage string) string {
	var parts []string

	// 1. 嵌入的基础系统提示
	parts = append(parts, systemPromptTemplate)

	// 2. 读取 AGENTS.md
	agentsPath := filepath.Join(b.workspace, "AGENTS.md")
	if content, err := os.ReadFile(agentsPath); err == nil {
		parts = append(parts, "## Agent Instructions\n"+string(content))
	}

	// 3. 读取 SOUL.md
	soulPath := filepath.Join(b.workspace, "SOUL.md")
	if content, err := os.ReadFile(soulPath); err == nil {
		parts = append(parts, "## Personality\n"+string(content))
	}

	// 4. 读取 USER.md
	userPath := filepath.Join(b.workspace, "USER.md")
	if content, err := os.ReadFile(userPath); err == nil {
		parts = append(parts, "## User Information\n"+string(content))
	}

	// 5. 读取 MEMORY.md
	memoryPath := filepath.Join(b.workspace, "memory", "MEMORY.md")
	if content, err := os.ReadFile(memoryPath); err == nil {
		parts = append(parts, "## Long-term Memory\n"+string(content))
	}

	// 6. 读取 heartbeat.md（OpenClaw 风格：短周期状态/优先级）
	// 优先读取 memory/heartbeat.md，兼容根目录 heartbeat.md
	if hb := b.loadHeartbeat(); hb != "" {
		parts = append(parts, "## Heartbeat\n"+hb)
	}

	// 7. Skills
	if skillsSection := b.buildSkillsSection(currentMessage); skillsSection != "" {
		parts = append(parts, skillsSection)
	}

	// 8. 动态环境信息
	envSection := b.buildEnvironmentSection(channel, chatID)
	parts = append(parts, envSection)

	return strings.Join(parts, "\n\n")
}

func (b *ContextBuilder) loadHeartbeat() string {
	candidates := []string{
		filepath.Join(b.workspace, "memory", "heartbeat.md"),
		filepath.Join(b.workspace, "heartbeat.md"),
	}

	for _, path := range candidates {
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		text := strings.TrimSpace(string(content))
		if text != "" {
			return text
		}
	}
	return ""
}

// buildEnvironmentSection 构建环境信息部分
func (b *ContextBuilder) buildEnvironmentSection(channel, chatID string) string {
	now := time.Now()
	year, month, day := now.Date()
	hour, min, _ := now.Clock()
	weekday := now.Weekday().String()

	// 替换模板变量
	result := environmentTemplate
	result = strings.ReplaceAll(result, "{{CURRENT_DATE}}", now.Format("2006-01-02 15:04:05 MST"))
	result = strings.ReplaceAll(result, "{{CURRENT_DATE_SHORT}}", now.Format("2006-01-02"))
	result = strings.ReplaceAll(result, "{{YEAR}}", fmt.Sprintf("%d", year))
	result = strings.ReplaceAll(result, "{{MONTH}}", fmt.Sprintf("%d (%s)", int(month), month))
	result = strings.ReplaceAll(result, "{{DAY}}", fmt.Sprintf("%d (%s)", day, weekday))
	result = strings.ReplaceAll(result, "{{WEEKDAY}}", weekday)
	result = strings.ReplaceAll(result, "{{TIME}}", fmt.Sprintf("%02d:%02d", hour, min))
	result = strings.ReplaceAll(result, "{{CHANNEL}}", channel)
	result = strings.ReplaceAll(result, "{{CHAT_ID}}", chatID)
	result = strings.ReplaceAll(result, "{{WORKSPACE}}", b.workspace)
	result = strings.ReplaceAll(result, "{{SKILLS_DIR}}", filepath.Join(b.workspace, "skills"))

	return result
}
