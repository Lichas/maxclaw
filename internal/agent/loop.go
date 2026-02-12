package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Lichas/nanobot-go/internal/bus"
	"github.com/Lichas/nanobot-go/internal/config"
	"github.com/Lichas/nanobot-go/internal/cron"
	"github.com/Lichas/nanobot-go/internal/logging"
	"github.com/Lichas/nanobot-go/internal/providers"
	"github.com/Lichas/nanobot-go/internal/session"
	"github.com/Lichas/nanobot-go/internal/skills"
	"github.com/Lichas/nanobot-go/pkg/tools"
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

	context  *ContextBuilder
	sessions *session.Manager
	tools    *tools.Registry
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
		context:             NewContextBuilder(workspace),
		sessions:            session.NewManager(workspace),
		tools:               tools.NewRegistry(),
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
	if h.onDelta != nil {
		h.onDelta(fmt.Sprintf("\n[Tool: %s]\n", name))
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
	if lg := logging.Get(); lg != nil && lg.Session != nil {
		lg.Session.Printf("inbound channel=%s chat=%s sender=%s content=%q", msg.Channel, msg.ChatID, msg.SenderID, logging.Truncate(msg.Content, 400))
	}

	// 获取或创建会话
	sess := a.sessions.GetOrCreate(msg.SessionKey)

	// 获取历史记录并转换为 providers.Message
	history := a.convertSessionMessages(sess.GetHistory())

	// 构建消息
	messages := a.context.BuildMessages(history, msg.Content, msg.Media, msg.Channel, msg.ChatID)

	// Agent 循环
	var finalContent string
	toolDefs := a.tools.GetDefinitions()

	for i := 0; i < a.MaxIterations; i++ {
		// 流式调用 LLM
		handler := newStreamHandler(msg.Channel, msg.ChatID, a.Bus, func(delta string) {
			// 实时发送流式内容（可选）
			if msg.Channel == "cli" {
				fmt.Print(delta)
			}
		})

		err := a.Provider.ChatStream(ctx, messages, toolDefs, a.Model, handler)
		if err != nil {
			return nil, fmt.Errorf("LLM stream error: %w", err)
		}

		// CLI 换行
		if msg.Channel == "cli" {
			fmt.Println()
		}

		content := handler.GetContent()
		toolCalls := handler.GetToolCalls()

		// 处理工具调用
		if len(toolCalls) > 0 {
			// 添加助手消息（带工具调用）
			messages = a.context.AddAssistantMessage(messages, content, toolCalls)

			// 执行工具调用并显示结果
			for _, tc := range toolCalls {
				var args map[string]interface{}
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
					args = map[string]interface{}{}
				}

				result, err := a.tools.Execute(ctx, tc.Function.Name, args)
				if err != nil {
					result = fmt.Sprintf("Error: %v", err)
				}

				if lg := logging.Get(); lg != nil && lg.Tools != nil {
					lg.Tools.Printf("tool name=%s args=%q result_len=%d", tc.Function.Name, logging.Truncate(tc.Function.Arguments, 300), len(result))
				}

				// 显示工具执行结果
				if msg.Channel == "cli" {
					fmt.Printf("[Result: %s]\n%s\n\n", tc.Function.Name, result)
				}

				messages = a.context.AddToolResult(messages, tc.ID, tc.Function.Name, result)
			}
		} else {
			// 没有工具调用，结束循环
			finalContent = content
			break
		}
	}

	if finalContent == "" {
		finalContent = "I've completed processing but have no response to give."
	}

	if lg := logging.Get(); lg != nil && lg.Session != nil {
		lg.Session.Printf("outbound channel=%s chat=%s content=%q", msg.Channel, msg.ChatID, logging.Truncate(finalContent, 400))
	}

	// 保存到会话
	sess.AddMessage("user", msg.Content)
	sess.AddMessage("assistant", finalContent)
	a.sessions.Save(sess)

	return bus.NewOutboundMessage(msg.Channel, msg.ChatID, finalContent), nil
}

// ProcessDirect 直接处理消息（用于 CLI）
func (a *AgentLoop) ProcessDirect(ctx context.Context, content, sessionKey, channel, chatID string) (string, error) {
	msg := bus.NewInboundMessage(channel, "user", chatID, content)
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
