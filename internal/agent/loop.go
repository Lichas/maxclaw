package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Lichas/maxclaw/internal/bus"
	"github.com/Lichas/maxclaw/internal/config"
	"github.com/Lichas/maxclaw/internal/cron"
	"github.com/Lichas/maxclaw/internal/logging"
	"github.com/Lichas/maxclaw/internal/memory"
	"github.com/Lichas/maxclaw/internal/providers"
	"github.com/Lichas/maxclaw/internal/session"
	"github.com/Lichas/maxclaw/internal/skills"
	"github.com/Lichas/maxclaw/pkg/tools"
)

const (
	sessionContextWindow         = 500
	sessionConsolidateThreshold  = 120
	sessionConsolidateKeepRecent = 40
)

// AgentLoop Agent 循环
type AgentLoop struct {
	Bus                 *bus.MessageBus
	Provider            providers.LLMProvider
	Workspace           string
	Model               string
	MaxIterations       int
	BraveAPIKey         string
	WebFetchOptions     tools.WebFetchOptions
	ExecConfig          config.ExecToolConfig
	RestrictToWorkspace bool
	CronService         *cron.Service
	MCPServers          map[string]config.MCPServerConfig

	context  *ContextBuilder
	sessions *session.Manager
	tools    *tools.Registry

	mcpConnector   *tools.MCPConnector
	mcpConnectOnce sync.Once
}

// StreamEvent is a structured event for UI streaming consumers.
type StreamEvent struct {
	Type       string `json:"type"`
	Iteration  int    `json:"iteration,omitempty"`
	Message    string `json:"message,omitempty"`
	Delta      string `json:"delta,omitempty"`
	ToolID     string `json:"toolId,omitempty"`
	ToolName   string `json:"toolName,omitempty"`
	ToolArgs   string `json:"toolArgs,omitempty"`
	Summary    string `json:"summary,omitempty"`
	ToolResult string `json:"toolResult,omitempty"`
	Response   string `json:"response,omitempty"`
	Done       bool   `json:"done,omitempty"`
}

// NewAgentLoop 创建 Agent 循环
func NewAgentLoop(
	bus *bus.MessageBus,
	provider providers.LLMProvider,
	workspace string,
	model string,
	maxIterations int,
	braveAPIKey string,
	webFetch tools.WebFetchOptions,
	execConfig config.ExecToolConfig,
	restrictToWorkspace bool,
	cronService *cron.Service,
	mcpServers map[string]config.MCPServerConfig,
) *AgentLoop {
	if maxIterations <= 0 {
		maxIterations = 20
	}

	// 设置工具允许的目录
	if restrictToWorkspace {
		tools.SetAllowedDir(workspace)
	}

	loop := &AgentLoop{
		Bus:                 bus,
		Provider:            provider,
		Workspace:           workspace,
		Model:               model,
		MaxIterations:       maxIterations,
		BraveAPIKey:         braveAPIKey,
		WebFetchOptions:     webFetch,
		ExecConfig:          execConfig,
		RestrictToWorkspace: restrictToWorkspace,
		CronService:         cronService,
		MCPServers:          cloneMCPServerConfigs(mcpServers),
		context:             NewContextBuilder(workspace),
		sessions:            session.NewManager(workspace),
		tools:               tools.NewRegistry(),
	}

	if len(loop.MCPServers) > 0 {
		loop.mcpConnector = tools.NewMCPConnector(convertMCPServers(loop.MCPServers))
	}

	loop.registerDefaultTools()
	return loop
}

// registerDefaultTools 注册默认工具
func (a *AgentLoop) registerDefaultTools() {
	// 文件工具
	a.tools.Register(tools.NewReadFileTool())
	a.tools.Register(tools.NewWriteFileTool())
	a.tools.Register(tools.NewEditFileTool())
	a.tools.Register(tools.NewListDirTool())

	// Shell 工具
	a.tools.Register(tools.NewExecTool(a.Workspace, a.ExecConfig.Timeout, a.RestrictToWorkspace))

	// Web 工具
	a.tools.Register(tools.NewWebSearchTool(a.BraveAPIKey, 5))
	a.tools.Register(tools.NewWebFetchTool(a.WebFetchOptions))
	a.tools.Register(tools.NewBrowserTool(tools.BrowserOptionsFromWebFetch(a.WebFetchOptions)))

	// 消息工具
	a.tools.Register(tools.NewMessageTool(func(channel, chatID, content string) error {
		return a.Bus.PublishOutbound(bus.NewOutboundMessage(channel, chatID, content))
	}))

	// 子代理工具
	spawnTool := tools.NewSpawnTool(func(task string) error {
		fmt.Printf("[Spawn] %s\n", task)
		return nil
	})
	a.tools.Register(spawnTool)

	// 定时任务工具
	if a.CronService != nil {
		cronTool := tools.NewCronTool(a.CronService)
		a.tools.Register(cronTool)
	}
}

// Run 运行 Agent 循环
func (a *AgentLoop) Run(ctx context.Context) error {
	a.ensureMCPConnected(ctx)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// 消费入站消息
		msg, err := a.Bus.ConsumeInbound(ctx)
		if err != nil {
			if err == context.Canceled || err == context.DeadlineExceeded {
				return nil
			}
			continue
		}

		// 处理消息
		response, err := a.ProcessMessage(ctx, msg)
		if err != nil {
			// 发送错误响应
			a.Bus.PublishOutbound(bus.NewOutboundMessage(
				msg.Channel,
				msg.ChatID,
				fmt.Sprintf("Error: %v", err),
			))
			continue
		}

		if response != nil {
			a.Bus.PublishOutbound(response)
		}
	}
}

// streamHandler 流式响应处理器
type streamHandler struct {
	channel           string
	chatID            string
	bus               *bus.MessageBus
	content           strings.Builder
	toolCalls         []providers.ToolCall
	accumulatingCalls map[string]*providers.ToolCall
	onDelta           func(string)
}

func newStreamHandler(channel, chatID string, msgBus *bus.MessageBus, onDelta func(string)) *streamHandler {
	return &streamHandler{
		channel:           channel,
		chatID:            chatID,
		bus:               msgBus,
		accumulatingCalls: make(map[string]*providers.ToolCall),
		onDelta:           onDelta,
	}
}

func (h *streamHandler) OnContent(token string) {
	h.content.WriteString(token)
	if h.onDelta != nil {
		h.onDelta(token)
	}
}

func (h *streamHandler) OnToolCallStart(id, name string) {
	h.accumulatingCalls[id] = &providers.ToolCall{
		ID:       id,
		Type:     "function",
		Function: providers.ToolCallFunction{Name: name, Arguments: ""},
	}
}

func (h *streamHandler) OnToolCallDelta(id, delta string) {
	if tc, ok := h.accumulatingCalls[id]; ok {
		tc.Function.Arguments += delta
	}
}

func (h *streamHandler) OnToolCallEnd(id string) {
	if tc, ok := h.accumulatingCalls[id]; ok {
		h.toolCalls = append(h.toolCalls, *tc)
		delete(h.accumulatingCalls, id)
	}
}

func (h *streamHandler) OnComplete() {}

func (h *streamHandler) OnError(err error) {
	fmt.Printf("[Stream Error] %v\n", err)
}

func (h *streamHandler) GetContent() string {
	return h.content.String()
}

func (h *streamHandler) GetToolCalls() []providers.ToolCall {
	return h.toolCalls
}

// ProcessMessage 处理单个消息（流式版本）
func (a *AgentLoop) ProcessMessage(ctx context.Context, msg *bus.InboundMessage) (*bus.OutboundMessage, error) {
	return a.processMessageWithCallbacks(ctx, msg, nil, nil)
}

func (a *AgentLoop) processMessageWithDelta(ctx context.Context, msg *bus.InboundMessage, onDelta func(string)) (*bus.OutboundMessage, error) {
	return a.processMessageWithCallbacks(ctx, msg, onDelta, nil)
}

func (a *AgentLoop) processMessageWithCallbacks(
	ctx context.Context,
	msg *bus.InboundMessage,
	onDelta func(string),
	onEvent func(StreamEvent),
) (*bus.OutboundMessage, error) {
	a.ensureMCPConnected(ctx)
	timeline := make([]session.TimelineEntry, 0, 64)
	emitEvent := func(event StreamEvent) {
		timeline = appendTimelineFromEvent(timeline, event)
		if onEvent != nil {
			onEvent(event)
		}
	}

	if lg := logging.Get(); lg != nil && lg.Session != nil {
		lg.Session.Printf("inbound channel=%s chat=%s sender=%s content=%q", msg.Channel, msg.ChatID, msg.SenderID, logging.Truncate(msg.Content, 400))
	}

	// 获取或创建会话
	sess := a.sessions.GetOrCreate(msg.SessionKey)

	// 统一 slash 命令
	cmd := strings.TrimSpace(strings.ToLower(msg.Content))
	switch cmd {
	case "/new":
		if _, err := memory.ArchiveSessionAll(a.Workspace, sess); err != nil {
			if lg := logging.Get(); lg != nil && lg.Session != nil {
				lg.Session.Printf("archive session on /new failed: %v", err)
			}
		}
		sess.Clear()
		_ = a.sessions.Save(sess)
		return bus.NewOutboundMessage(msg.Channel, msg.ChatID, "New session started."), nil
	case "/help":
		return bus.NewOutboundMessage(
			msg.Channel,
			msg.ChatID,
			"maxclaw commands:\n/new - Start a new conversation\n/help - Show available commands",
		), nil
	}

	// 获取历史记录并转换为 providers.Message
	history := a.convertSessionMessages(sess.GetHistory(sessionContextWindow))

	// 构建消息
	selectedSkillRefs := normalizeSkillRefs(msg.SelectedSkills)
	messages := a.context.BuildMessagesWithSkillRefs(history, msg.Content, selectedSkillRefs, msg.Media, msg.Channel, msg.ChatID)

	// Agent 循环
	var finalContent string
	maxIterationReached := true
	toolDefs := a.tools.GetDefinitions()

	for i := 0; i < a.MaxIterations; i++ {
		iteration := i + 1
		emitEvent(StreamEvent{
			Type:      "status",
			Iteration: iteration,
			Message:   fmt.Sprintf("Iteration %d", iteration),
		})

		deltaCallback := onDelta
		if deltaCallback == nil && msg.Channel == "cli" {
			deltaCallback = func(delta string) {
				fmt.Print(delta)
			}
		}
		streamCallback := func(delta string) {
			if deltaCallback != nil {
				deltaCallback(delta)
			}
			emitEvent(StreamEvent{
				Type:      "content_delta",
				Delta:     delta,
				Iteration: iteration,
			})
		}

		// 流式调用 LLM
		handler := newStreamHandler(msg.Channel, msg.ChatID, a.Bus, streamCallback)

		err := a.Provider.ChatStream(ctx, messages, toolDefs, a.Model, handler)
		if err != nil {
			return nil, fmt.Errorf("LLM stream error: %w", err)
		}

		// CLI 换行
		if msg.Channel == "cli" && onDelta == nil && onEvent == nil {
			fmt.Println()
		}

		content := handler.GetContent()
		toolCalls := handler.GetToolCalls()

		// 处理工具调用
		if len(toolCalls) > 0 {
			emitEvent(StreamEvent{
				Type:      "status",
				Iteration: iteration,
				Message:   "Executing tools",
			})

			// 添加助手消息（带工具调用）
			messages = a.context.AddAssistantMessage(messages, content, toolCalls)

			// 执行工具调用并显示结果
			for _, tc := range toolCalls {
				emitEvent(StreamEvent{
					Type:      "tool_start",
					Iteration: iteration,
					ToolID:    tc.ID,
					ToolName:  tc.Function.Name,
					ToolArgs:  truncateEventText(tc.Function.Arguments, 600),
					Summary:   summarizeToolStart(tc.Function.Name, tc.Function.Arguments),
				})

				var args map[string]interface{}
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
					args = map[string]interface{}{}
				}

				toolCtx := tools.WithRuntimeContext(ctx, msg.Channel, msg.ChatID)
				result, execErr := a.tools.Execute(toolCtx, tc.Function.Name, args)
				if execErr != nil {
					result = fmt.Sprintf("Error: %v", execErr)
				}

				if lg := logging.Get(); lg != nil && lg.Tools != nil {
					lg.Tools.Printf("tool name=%s args=%q result_len=%d", tc.Function.Name, logging.Truncate(tc.Function.Arguments, 300), len(result))
				}

				// 显示工具执行结果
				if msg.Channel == "cli" {
					fmt.Printf("[Result: %s]\n%s\n\n", tc.Function.Name, result)
				}

				emitEvent(StreamEvent{
					Type:       "tool_result",
					Iteration:  iteration,
					ToolID:     tc.ID,
					ToolName:   tc.Function.Name,
					ToolResult: truncateEventText(result, 2000),
					Summary:    summarizeToolResult(tc.Function.Name, result, execErr),
				})

				messages = a.context.AddToolResult(messages, tc.ID, tc.Function.Name, result)
			}
		} else {
			// 没有工具调用，结束循环
			finalContent = content
			maxIterationReached = false
			emitEvent(StreamEvent{
				Type:      "status",
				Iteration: iteration,
				Message:   "Preparing final response",
			})
			break
		}
	}

	if finalContent == "" {
		if maxIterationReached {
			finalContent = fmt.Sprintf("Reached %d iterations without completion.", a.MaxIterations)
		} else {
			finalContent = "I've completed processing but have no response to give."
		}
	}

	if lg := logging.Get(); lg != nil && lg.Session != nil {
		lg.Session.Printf("outbound channel=%s chat=%s content=%q", msg.Channel, msg.ChatID, logging.Truncate(finalContent, 400))
	}

	// 保存到会话
	sess.AddMessage("user", msg.Content)
	if len(timeline) > 0 {
		sess.AddMessageWithTimeline("assistant", finalContent, timeline)
	} else {
		sess.AddMessage("assistant", finalContent)
	}

	if len(sess.Messages) > sessionConsolidateThreshold {
		if _, err := memory.ConsolidateSession(a.Workspace, sess, sessionConsolidateKeepRecent); err != nil {
			if lg := logging.Get(); lg != nil && lg.Session != nil {
				lg.Session.Printf("memory consolidation failed: %v", err)
			}
		}
	}
	a.sessions.Save(sess)

	return bus.NewOutboundMessage(msg.Channel, msg.ChatID, finalContent), nil
}

// ProcessDirect 直接处理消息（用于 CLI）
func (a *AgentLoop) ProcessDirect(ctx context.Context, content, sessionKey, channel, chatID string) (string, error) {
	return a.ProcessDirectWithSkills(ctx, content, sessionKey, channel, chatID, nil)
}

func (a *AgentLoop) ProcessDirectWithSkills(
	ctx context.Context,
	content, sessionKey, channel, chatID string,
	selectedSkills []string,
) (string, error) {
	msg := bus.NewInboundMessage(channel, "user", chatID, content)
	if sessionKey != "" {
		msg.SessionKey = sessionKey
	}
	msg.SelectedSkills = normalizeSkillRefs(selectedSkills)

	resp, err := a.ProcessMessage(ctx, msg)
	if err != nil {
		return "", err
	}
	if resp == nil {
		return "", nil
	}
	// CLI 模式下流式输出已实时打印，返回空字符串避免重复输出
	if channel == "cli" {
		return "", nil
	}
	return resp.Content, nil
}

// ProcessDirectStream 直接处理消息并按 delta 回调流式输出。
func (a *AgentLoop) ProcessDirectStream(
	ctx context.Context,
	content, sessionKey, channel, chatID string,
	onDelta func(string),
) (string, error) {
	msg := bus.NewInboundMessage(channel, "user", chatID, content)
	if sessionKey != "" {
		msg.SessionKey = sessionKey
	}

	resp, err := a.processMessageWithDelta(ctx, msg, onDelta)
	if err != nil {
		return "", err
	}
	if resp == nil {
		return "", nil
	}
	return resp.Content, nil
}

// ProcessDirectEventStream streams structured events for UI clients.
func (a *AgentLoop) ProcessDirectEventStream(
	ctx context.Context,
	content, sessionKey, channel, chatID string,
	onEvent func(StreamEvent),
) (string, error) {
	return a.ProcessDirectEventStreamWithSkills(ctx, content, sessionKey, channel, chatID, nil, onEvent)
}

func (a *AgentLoop) ProcessDirectEventStreamWithSkills(
	ctx context.Context,
	content, sessionKey, channel, chatID string,
	selectedSkills []string,
	onEvent func(StreamEvent),
) (string, error) {
	msg := bus.NewInboundMessage(channel, "user", chatID, content)
	if sessionKey != "" {
		msg.SessionKey = sessionKey
	}
	msg.SelectedSkills = normalizeSkillRefs(selectedSkills)

	resp, err := a.processMessageWithCallbacks(ctx, msg, nil, onEvent)
	if err != nil {
		return "", err
	}
	if resp == nil {
		return "", nil
	}
	return resp.Content, nil
}

func summarizeToolStart(name, args string) string {
	argPreview := strings.TrimSpace(args)
	if argPreview == "" {
		return fmt.Sprintf("%s started", name)
	}
	return fmt.Sprintf("%s %s", name, truncateEventText(argPreview, 100))
}

func summarizeToolResult(name, result string, err error) string {
	if err != nil {
		return fmt.Sprintf("%s failed: %v", name, err)
	}
	trimmed := strings.TrimSpace(result)
	if trimmed == "" {
		return fmt.Sprintf("%s completed", name)
	}
	firstLine := strings.SplitN(trimmed, "\n", 2)[0]
	return fmt.Sprintf("%s -> %s", name, truncateEventText(firstLine, 140))
}

func truncateEventText(input string, max int) string {
	if max <= 0 {
		return input
	}
	runes := []rune(input)
	if len(runes) <= max {
		return input
	}
	return string(runes[:max]) + "..."
}

func normalizeSkillRefs(selectedSkills []string) []string {
	if len(selectedSkills) == 0 {
		return nil
	}

	out := make([]string, 0, len(selectedSkills))
	seen := make(map[string]struct{}, len(selectedSkills))
	for _, raw := range selectedSkills {
		ref := sanitizeSkillRef(raw)
		if ref == "" {
			continue
		}
		if _, ok := seen[ref]; ok {
			continue
		}
		seen[ref] = struct{}{}
		out = append(out, ref)
	}
	return out
}

func sanitizeSkillRef(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range raw {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '.' || r == '-' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func appendTimelineFromEvent(timeline []session.TimelineEntry, event StreamEvent) []session.TimelineEntry {
	switch event.Type {
	case "content_delta":
		if event.Delta == "" {
			return timeline
		}
		if len(timeline) > 0 && timeline[len(timeline)-1].Kind == "text" {
			last := timeline[len(timeline)-1]
			last.Text += event.Delta
			timeline[len(timeline)-1] = last
			return timeline
		}
		return append(timeline, session.TimelineEntry{
			Kind: "text",
			Text: event.Delta,
		})
	case "status", "tool_start", "tool_result", "error":
		summary := strings.TrimSpace(event.Summary)
		if summary == "" {
			summary = strings.TrimSpace(event.Message)
		}
		detail := ""
		switch event.Type {
		case "tool_start":
			detail = strings.TrimSpace(event.ToolArgs)
		case "tool_result":
			detail = strings.TrimSpace(event.ToolResult)
		}

		if summary == "" && detail == "" {
			return timeline
		}

		activity := session.TimelineActivity{
			Type:    event.Type,
			Summary: summary,
			Detail:  detail,
		}

		if len(timeline) > 0 {
			last := timeline[len(timeline)-1]
			if last.Kind == "activity" && last.Activity != nil &&
				last.Activity.Type == activity.Type &&
				last.Activity.Summary == activity.Summary &&
				last.Activity.Detail == activity.Detail {
				return timeline
			}
		}

		return append(timeline, session.TimelineEntry{
			Kind:     "activity",
			Activity: &activity,
		})
	default:
		return timeline
	}
}

// convertSessionMessages 转换会话消息
func (a *AgentLoop) convertSessionMessages(msgs []session.Message) []providers.Message {
	result := make([]providers.Message, len(msgs))
	for i, msg := range msgs {
		result[i] = providers.Message{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}
	return result
}

// LoadSkills 加载技能文件
func (a *AgentLoop) LoadSkills() error {
	skillsDir := filepath.Join(a.Workspace, "skills")
	_, err := skills.Discover(skillsDir)
	return err
}

// Close 释放 AgentLoop 资源（主要是 MCP 连接）。
func (a *AgentLoop) Close() error {
	if a.mcpConnector == nil {
		return nil
	}
	return a.mcpConnector.Close()
}

func (a *AgentLoop) ensureMCPConnected(ctx context.Context) {
	if a.mcpConnector == nil {
		return
	}
	a.mcpConnectOnce.Do(func() {
		if err := a.mcpConnector.Connect(ctx, a.tools); err != nil {
			if lg := logging.Get(); lg != nil && lg.Tools != nil {
				lg.Tools.Printf("mcp connect warning: %v", err)
			}
		} else if lg := logging.Get(); lg != nil && lg.Tools != nil {
			registered := a.mcpConnector.RegisteredTools()
			if len(registered) > 0 {
				lg.Tools.Printf("mcp connected tools=%v", registered)
			}
		}
	})
}

func cloneMCPServerConfigs(in map[string]config.MCPServerConfig) map[string]config.MCPServerConfig {
	if len(in) == 0 {
		return map[string]config.MCPServerConfig{}
	}
	out := make(map[string]config.MCPServerConfig, len(in))
	for name, server := range in {
		s := server
		if s.Args == nil {
			s.Args = []string{}
		}
		if s.Env == nil {
			s.Env = map[string]string{}
		}
		out[name] = s
	}
	return out
}

func convertMCPServers(in map[string]config.MCPServerConfig) map[string]tools.MCPServerOptions {
	if len(in) == 0 {
		return map[string]tools.MCPServerOptions{}
	}
	out := make(map[string]tools.MCPServerOptions, len(in))
	for name, server := range in {
		out[name] = tools.MCPServerOptions{
			Name:    name,
			Command: server.Command,
			Args:    append([]string(nil), server.Args...),
			Env:     cloneStringMap(server.Env),
			URL:     server.URL,
		}
	}
	return out
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
