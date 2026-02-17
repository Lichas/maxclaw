package agent

import (
	_ "embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Lichas/nanobot-go/internal/bus"
	"github.com/Lichas/nanobot-go/internal/providers"
)

//go:embed prompts/system_prompt.md
var systemPromptTemplate string

//go:embed prompts/environment.md
var environmentTemplate string

const nanobotSourceMarkerFile = ".nanobot-source-root"
const nanobotSourceSearchRootsEnv = "NANOBOT_SOURCE_SEARCH_ROOTS"
const nanobotSourceSearchMaxDepth = 5

var errNanobotSourceMarkerFound = errors.New("nanobot source marker found")

// ContextBuilder 上下文构建器
type ContextBuilder struct {
	workspace string

	sourceOnce        sync.Once
	sourceDir         string
	sourceMarkerPath  string
	sourceMarkerFound bool
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

	// 9. 两层内存提示（HISTORY.md 不自动注入上下文，按需 grep）
	parts = append(parts, b.buildMemoryHintsSection())

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
	sourceDir, markerPath, markerFound := b.resolveNanobotSource()

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
	result = strings.ReplaceAll(result, "{{NANOBOT_SOURCE_MARKER_FILE}}", nanobotSourceMarkerFile)
	result = strings.ReplaceAll(result, "{{NANOBOT_SOURCE_MARKER_PATH}}", markerPath)
	result = strings.ReplaceAll(result, "{{NANOBOT_SOURCE_DIR}}", sourceDir)
	result = strings.ReplaceAll(result, "{{NANOBOT_SOURCE_MARKER_FOUND}}", boolYesNo(markerFound))

	return result
}

func (b *ContextBuilder) resolveNanobotSource() (sourceDir, markerPath string, markerFound bool) {
	b.sourceOnce.Do(func() {
		b.sourceDir, b.sourceMarkerPath, b.sourceMarkerFound = b.resolveNanobotSourceUncached()
	})
	return b.sourceDir, b.sourceMarkerPath, b.sourceMarkerFound
}

func (b *ContextBuilder) resolveNanobotSourceUncached() (sourceDir, markerPath string, markerFound bool) {
	if envSource := strings.TrimSpace(os.Getenv("NANOBOT_SOURCE_DIR")); envSource != "" {
		sourceDir = envSource
		if abs, err := filepath.Abs(sourceDir); err == nil {
			sourceDir = abs
		}
		markerPath = filepath.Join(sourceDir, nanobotSourceMarkerFile)
		if info, err := os.Stat(markerPath); err == nil && !info.IsDir() {
			return sourceDir, markerPath, true
		}
		return sourceDir, markerPath, false
	}

	start := b.workspace
	if start == "" {
		start = "."
	}
	absStart, err := filepath.Abs(start)
	if err != nil {
		absStart = start
	}

	dir := absStart
	for {
		candidate := filepath.Join(dir, nanobotSourceMarkerFile)
		if info, statErr := os.Stat(candidate); statErr == nil && !info.IsDir() {
			return dir, candidate, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	for _, root := range b.sourceSearchRoots(absStart) {
		if foundDir, foundMarker, found := findSourceMarkerUnder(root, nanobotSourceSearchMaxDepth); found {
			return foundDir, foundMarker, true
		}
	}

	return absStart, filepath.Join(absStart, nanobotSourceMarkerFile), false
}

func boolYesNo(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}

func (b *ContextBuilder) buildMemoryHintsSection() string {
	memoryPath := filepath.Join(b.workspace, "memory", "MEMORY.md")
	historyPath := filepath.Join(b.workspace, "memory", "HISTORY.md")
	return strings.Join([]string{
		"## Memory System",
		fmt.Sprintf("- Long-term memory: %s (always loaded)", memoryPath),
		fmt.Sprintf("- History log: %s (append-only, grep-searchable, not auto-loaded)", historyPath),
		fmt.Sprintf("- To recall past events, use exec with grep, for example: grep -i \"keyword\" %s", historyPath),
	}, "\n")
}

func (b *ContextBuilder) sourceSearchRoots(absWorkspace string) []string {
	var roots []string
	seen := make(map[string]struct{})

	addRoot := func(candidate string) {
		candidate = expandSimplePath(candidate)
		if candidate == "" {
			return
		}

		abs, err := filepath.Abs(candidate)
		if err != nil {
			return
		}
		abs = filepath.Clean(abs)

		if _, ok := seen[abs]; ok {
			return
		}

		info, err := os.Stat(abs)
		if err != nil || !info.IsDir() {
			return
		}

		seen[abs] = struct{}{}
		roots = append(roots, abs)
	}

	for _, raw := range parseSourceSearchRoots(os.Getenv(nanobotSourceSearchRootsEnv)) {
		addRoot(raw)
	}

	if home, err := os.UserHomeDir(); err == nil {
		home = filepath.Clean(home)
		if filepath.Clean(absWorkspace) == filepath.Join(home, ".nanobot", "workspace") {
			addRoot(filepath.Join(home, "git"))
			addRoot(filepath.Join(home, "src"))
		}
	}

	return roots
}

func parseSourceSearchRoots(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}

	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == os.PathListSeparator || r == ',' || r == '\n'
	})

	roots := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			roots = append(roots, trimmed)
		}
	}
	return roots
}

func expandSimplePath(path string) string {
	path = strings.TrimSpace(os.ExpandEnv(path))
	if path == "" {
		return ""
	}
	if path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
		return path
	}
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

func findSourceMarkerUnder(root string, maxDepth int) (sourceDir, markerPath string, markerFound bool) {
	root = filepath.Clean(root)
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return "", "", false
	}

	directMarker := filepath.Join(root, nanobotSourceMarkerFile)
	if markerInfo, markerErr := os.Stat(directMarker); markerErr == nil && !markerInfo.IsDir() {
		return root, directMarker, true
	}

	var found string
	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if d.IsDir() {
			if path != root {
				if sourceSearchSkipDir(d.Name()) {
					return filepath.SkipDir
				}
			}
			if pathDepth(root, path) > maxDepth {
				return filepath.SkipDir
			}
			return nil
		}

		if d.Name() == nanobotSourceMarkerFile {
			found = path
			return errNanobotSourceMarkerFound
		}
		return nil
	})

	if errors.Is(walkErr, errNanobotSourceMarkerFound) {
		return filepath.Dir(found), found, true
	}
	return "", "", false
}

func pathDepth(root, path string) int {
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "." {
		return 0
	}
	depth := 0
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		if part != "" && part != "." {
			depth++
		}
	}
	return depth
}

func sourceSearchSkipDir(name string) bool {
	switch name {
	case ".git", ".hg", ".svn", "node_modules", ".idea", ".vscode", "__pycache__":
		return true
	default:
		return false
	}
}
