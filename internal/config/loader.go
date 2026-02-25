package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	maxclawConfigDirEnv = "MAXCLAW_HOME"
	legacyConfigDirEnv  = "NANOBOT_HOME"
	maxclawConfigDir    = ".maxclaw"
	legacyConfigDir     = ".nanobot"
)

// GetConfigDir 返回配置目录
func GetConfigDir() string {
	if envDir := strings.TrimSpace(os.Getenv(maxclawConfigDirEnv)); envDir != "" {
		return expandPath(envDir)
	}
	if envDir := strings.TrimSpace(os.Getenv(legacyConfigDirEnv)); envDir != "" {
		return expandPath(envDir)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return maxclawConfigDir
	}

	maxclawDirPath := filepath.Join(homeDir, maxclawConfigDir)
	legacyDirPath := filepath.Join(homeDir, legacyConfigDir)

	if dirExists(maxclawDirPath) {
		return maxclawDirPath
	}
	if dirExists(legacyDirPath) {
		return legacyDirPath
	}

	return maxclawDirPath
}

// GetConfigPath 返回配置文件路径
func GetConfigPath() string {
	return filepath.Join(GetConfigDir(), "config.json")
}

// GetDataDir 返回数据目录
func GetDataDir() string {
	return GetConfigDir()
}

// GetLogsDir 返回日志目录
func GetLogsDir() string {
	return filepath.Join(GetConfigDir(), "logs")
}

// GetWorkspacePath 返回工作空间路径
func GetWorkspacePath() string {
	return filepath.Join(GetConfigDir(), "workspace")
}

// LoadConfig 从文件加载配置
func LoadConfig() (*Config, error) {
	configPath := GetConfigPath()

	// 如果配置文件不存在，返回默认配置
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return DefaultConfig(), nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	config := DefaultConfig()
	if err := json.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// 兼容 Claude Desktop / Cursor 风格的顶层 mcpServers 配置
	// 若 tools.mcpServers 已显式设置，则其优先级更高
	var topLevel struct {
		MCPServers map[string]MCPServerConfig `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &topLevel); err == nil && len(topLevel.MCPServers) > 0 {
		if config.Tools.MCPServers == nil {
			config.Tools.MCPServers = map[string]MCPServerConfig{}
		}
		for name, server := range topLevel.MCPServers {
			if _, exists := config.Tools.MCPServers[name]; !exists {
				config.Tools.MCPServers[name] = server
			}
		}
	}

	// Expand workspace path (supports ~ and $HOME)
	config.Agents.Defaults.Workspace = expandPath(config.Agents.Defaults.Workspace)
	config.Agents.Defaults.ExecutionMode = NormalizeExecutionMode(config.Agents.Defaults.ExecutionMode)

	return config, nil
}

func expandPath(path string) string {
	if path == "" {
		return path
	}

	path = os.ExpandEnv(path)

	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			if path == "~" {
				return home
			}
			if strings.HasPrefix(path, "~/") {
				return filepath.Join(home, path[2:])
			}
		}
	}

	return path
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// SaveConfig 保存配置到文件
func SaveConfig(config *Config) error {
	configDir := GetConfigDir()
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	configPath := GetConfigPath()
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// EnsureWorkspace 确保工作空间目录存在
func EnsureWorkspace() error {
	workspace := GetWorkspacePath()
	if err := os.MkdirAll(workspace, 0755); err != nil {
		return fmt.Errorf("failed to create workspace: %w", err)
	}
	return nil
}

// CreateWorkspaceTemplates 创建工作空间模板文件
func CreateWorkspaceTemplates() error {
	workspace := GetWorkspacePath()

	templates := map[string]string{
		"AGENTS.md": `# Agent Instructions

You are a helpful AI assistant. Be concise, accurate, and friendly.

## Guidelines

- Always explain what you're doing before taking actions
- Ask for clarification when the request is ambiguous
- Use tools to help accomplish tasks
- Remember important information in your memory files
`,
		"SOUL.md": `# Soul

I am maxclaw, a lightweight AI assistant.

## Personality

- Helpful and friendly
- Concise and to the point
- Curious and eager to learn

## Values

- Accuracy over speed
- User privacy and safety
- Transparency in actions
`,
		"USER.md": `# User

Information about the user goes here.

## Preferences

- Communication style: (casual/formal)
- Timezone: (your timezone)
- Language: (your preferred language)
`,
	}

	for filename, content := range templates {
		filePath := filepath.Join(workspace, filename)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
				return fmt.Errorf("failed to create %s: %w", filename, err)
			}
		}
	}

	// 创建 skills 目录与示例
	skillsDir := filepath.Join(workspace, "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		return fmt.Errorf("failed to create skills directory: %w", err)
	}

	skillsReadme := filepath.Join(skillsDir, "README.md")
	if _, err := os.Stat(skillsReadme); os.IsNotExist(err) {
		skillsContent := `# Skills

Skills 是一组可复用的本地指令文件。将技能放在本目录下的 .md 文件中：

- skills/<name>.md
- skills/<name>/SKILL.md

触发方式：
- @skill:<name> 只加载指定技能
- $<name> 只加载指定技能
- @skill:all / $all 加载全部技能
- @skill:none / $none 禁用技能加载
`
		if err := os.WriteFile(skillsReadme, []byte(skillsContent), 0644); err != nil {
			return fmt.Errorf("failed to create skills README: %w", err)
		}
	}

	exampleDir := filepath.Join(skillsDir, "example")
	if err := os.MkdirAll(exampleDir, 0755); err != nil {
		return fmt.Errorf("failed to create example skill directory: %w", err)
	}

	exampleSkill := filepath.Join(exampleDir, "SKILL.md")
	if _, err := os.Stat(exampleSkill); os.IsNotExist(err) {
		exampleContent := `# Example Skill

When writing responses:
- Be concise
- Provide steps
- Call tools when needed
`
		if err := os.WriteFile(exampleSkill, []byte(exampleContent), 0644); err != nil {
			return fmt.Errorf("failed to create example skill: %w", err)
		}
	}

	// 创建 memory 目录
	memoryDir := filepath.Join(workspace, "memory")
	if err := os.MkdirAll(memoryDir, 0755); err != nil {
		return fmt.Errorf("failed to create memory directory: %w", err)
	}

	// 创建 MEMORY.md
	memoryPath := filepath.Join(memoryDir, "MEMORY.md")
	if _, err := os.Stat(memoryPath); os.IsNotExist(err) {
		memoryContent := `# Long-term Memory

This file stores important information that should persist across sessions.

## User Information

(Important facts about the user)

## Preferences

(User preferences learned over time)

## Important Notes

(Things to remember)
`
		if err := os.WriteFile(memoryPath, []byte(memoryContent), 0644); err != nil {
			return fmt.Errorf("failed to create MEMORY.md: %w", err)
		}
	}

	// 创建 HISTORY.md（两层内存系统：grep 可检索历史日志）
	historyPath := filepath.Join(memoryDir, "HISTORY.md")
	if _, err := os.Stat(historyPath); os.IsNotExist(err) {
		historyContent := `# Conversation History

Append-only summaries for grep-based recall.
`
		if err := os.WriteFile(historyPath, []byte(historyContent), 0644); err != nil {
			return fmt.Errorf("failed to create HISTORY.md: %w", err)
		}
	}

	// 创建 heartbeat.md（短周期工作状态）
	heartbeatPath := filepath.Join(memoryDir, "heartbeat.md")
	if _, err := os.Stat(heartbeatPath); os.IsNotExist(err) {
		heartbeatContent := `# Heartbeat

Last Updated: (fill automatically or manually)

## Focus Now

- Current top priorities

## Blockers

- What is blocked and why

## Next Checkpoint

- What should happen next
`
		if err := os.WriteFile(heartbeatPath, []byte(heartbeatContent), 0644); err != nil {
			return fmt.Errorf("failed to create heartbeat.md: %w", err)
		}
	}

	return nil
}
