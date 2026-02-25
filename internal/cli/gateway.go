package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Lichas/maxclaw/internal/agent"
	"github.com/Lichas/maxclaw/internal/bus"
	"github.com/Lichas/maxclaw/internal/channels"
	"github.com/Lichas/maxclaw/internal/config"
	"github.com/Lichas/maxclaw/internal/cron"
	"github.com/Lichas/maxclaw/internal/logging"
	"github.com/Lichas/maxclaw/internal/memory"
	"github.com/Lichas/maxclaw/internal/providers"
	"github.com/Lichas/maxclaw/internal/webui"
	"github.com/spf13/cobra"
)

var gatewayPort int

func init() {
	gatewayCmd.Flags().IntVarP(&gatewayPort, "port", "p", 18890, "Gateway port")
}

// gatewayCmd 网关命令
var gatewayCmd = &cobra.Command{
	Use:   "gateway",
	Short: "Start the maxclaw gateway",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		if _, err := logging.Init(config.GetDataDir()); err != nil {
			fmt.Printf("⚠ logging init error: %v\n", err)
		}

		if lg := logging.Get(); lg != nil && lg.Gateway != nil {
			lg.Gateway.Printf("gateway starting port=%d model=%s workspace=%s", gatewayPort, cfg.Agents.Defaults.Model, cfg.Agents.Defaults.Workspace)
		}

		apiKey := cfg.GetAPIKey("")
		apiBase := cfg.GetAPIBase("")
		provider, bootWarning, err := buildGatewayProvider(cfg, apiKey, apiBase)
		if err != nil {
			return err
		}

		fmt.Printf("%s Starting maxclaw gateway on port %d...\n\n", logo, gatewayPort)
		if bootWarning != "" {
			fmt.Printf("⚠ %s\n", bootWarning)
			if lg := logging.Get(); lg != nil && lg.Gateway != nil {
				lg.Gateway.Printf("startup warning: %s", bootWarning)
			}
		}

		// 创建组件
		messageBus := bus.NewMessageBus(100)

		// 创建 Cron 服务（需要先创建，传给 agent）
		storePath := filepath.Join(cfg.Agents.Defaults.Workspace, ".cron", "jobs.json")
		cronService := cron.NewService(storePath)
		cronService.SetJobHandler(func(job *cron.Job) (string, error) {
			// Deliverable jobs should go through the live gateway bus so they are sent to the real channel/chat.
			if job != nil && job.Payload.Deliver && job.Payload.Channel != "" && job.Payload.To != "" {
				return enqueueCronJob(messageBus, job)
			}
			return executeCronJob(cfg, apiKey, apiBase, cronService, job)
		})

		agentLoop := agent.NewAgentLoop(
			messageBus,
			provider,
			cfg.Agents.Defaults.Workspace,
			cfg.Agents.Defaults.Model,
			cfg.Agents.Defaults.MaxToolIterations,
			cfg.Tools.Web.Search.APIKey,
			agent.BuildWebFetchOptions(cfg),
			cfg.Tools.Exec,
			cfg.Tools.RestrictToWorkspace,
			cronService,
			cfg.Tools.MCPServers,
			cfg.Agents.Defaults.EnableGlobalSkills,
		)
		agentLoop.UpdateRuntimeExecutionMode(cfg.Agents.Defaults.ExecutionMode)
		defer agentLoop.Close()

		// 创建频道注册表
		channelRegistry := channels.NewRegistry()

		// 注册 Telegram
		if cfg.Channels.Telegram.Enabled {
			tgChannel := channels.NewTelegramChannel(&channels.TelegramConfig{
				Token:     cfg.Channels.Telegram.Token,
				Enabled:   cfg.Channels.Telegram.Enabled,
				AllowFrom: cfg.Channels.Telegram.AllowFrom,
				Proxy:     cfg.Channels.Telegram.Proxy,
			})
			tgChannel.SetMessageHandler(func(msg *channels.Message) {
				// 转发到消息总线
				inboundMsg := bus.NewInboundMessage("telegram", msg.Sender, msg.ChatID, msg.Text)
				messageBus.PublishInbound(inboundMsg)
			})
			channelRegistry.Register(tgChannel)
		}

		// 注册 Discord
		if cfg.Channels.Discord.Enabled {
			dcChannel := channels.NewDiscordChannel(&channels.DiscordConfig{
				Token:     cfg.Channels.Discord.Token,
				Enabled:   cfg.Channels.Discord.Enabled,
				AllowFrom: cfg.Channels.Discord.AllowFrom,
			})
			dcChannel.SetMessageHandler(func(msg *channels.Message) {
				inboundMsg := bus.NewInboundMessage("discord", msg.Sender, msg.ChatID, msg.Text)
				messageBus.PublishInbound(inboundMsg)
			})
			channelRegistry.Register(dcChannel)
		}

		// 注册 WhatsApp (Bridge)
		if cfg.Channels.WhatsApp.Enabled {
			waChannel := channels.NewWhatsAppChannel(&channels.WhatsAppConfig{
				Enabled:     cfg.Channels.WhatsApp.Enabled,
				BridgeURL:   cfg.Channels.WhatsApp.BridgeURL,
				BridgeToken: cfg.Channels.WhatsApp.BridgeToken,
				AllowFrom:   cfg.Channels.WhatsApp.AllowFrom,
				AllowSelf:   cfg.Channels.WhatsApp.AllowSelf,
			})
			waChannel.SetMessageHandler(func(msg *channels.Message) {
				inboundMsg := bus.NewInboundMessage("whatsapp", msg.Sender, msg.ChatID, msg.Text)
				messageBus.PublishInbound(inboundMsg)
			})
			channelRegistry.Register(waChannel)
		}

		// 注册 WebSocket
		if cfg.Channels.WebSocket.Enabled {
			wsChannel := channels.NewWebSocketChannel(&channels.WebSocketConfig{
				Enabled:      cfg.Channels.WebSocket.Enabled,
				Host:         cfg.Channels.WebSocket.Host,
				Port:         cfg.Channels.WebSocket.Port,
				Path:         cfg.Channels.WebSocket.Path,
				AllowOrigins: cfg.Channels.WebSocket.AllowOrigins,
			})
			wsChannel.SetMessageHandler(func(msg *channels.Message) {
				inboundMsg := bus.NewInboundMessage("websocket", msg.Sender, msg.ChatID, msg.Text)
				messageBus.PublishInbound(inboundMsg)
			})
			channelRegistry.Register(wsChannel)
		}

		// 注册 Slack（Socket Mode）
		if cfg.Channels.Slack.Enabled {
			slackChannel := channels.NewSlackChannel(&channels.SlackConfig{
				Enabled:   cfg.Channels.Slack.Enabled,
				BotToken:  cfg.Channels.Slack.BotToken,
				AppToken:  cfg.Channels.Slack.AppToken,
				AllowFrom: cfg.Channels.Slack.AllowFrom,
			})
			slackChannel.SetMessageHandler(func(msg *channels.Message) {
				inboundMsg := bus.NewInboundMessage("slack", msg.Sender, msg.ChatID, msg.Text)
				messageBus.PublishInbound(inboundMsg)
			})
			channelRegistry.Register(slackChannel)
		}

		// 注册 Email（IMAP/SMTP）
		if cfg.Channels.Email.Enabled {
			emailChannel := channels.NewEmailChannel(&channels.EmailConfig{
				Enabled:             cfg.Channels.Email.Enabled,
				ConsentGranted:      cfg.Channels.Email.ConsentGranted,
				IMAPHost:            cfg.Channels.Email.IMAPHost,
				IMAPPort:            cfg.Channels.Email.IMAPPort,
				IMAPUsername:        cfg.Channels.Email.IMAPUsername,
				IMAPPassword:        cfg.Channels.Email.IMAPPassword,
				IMAPMailbox:         cfg.Channels.Email.IMAPMailbox,
				IMAPUseSSL:          cfg.Channels.Email.IMAPUseSSL,
				SMTPHost:            cfg.Channels.Email.SMTPHost,
				SMTPPort:            cfg.Channels.Email.SMTPPort,
				SMTPUsername:        cfg.Channels.Email.SMTPUsername,
				SMTPPassword:        cfg.Channels.Email.SMTPPassword,
				SMTPUseTLS:          cfg.Channels.Email.SMTPUseTLS,
				SMTPUseSSL:          cfg.Channels.Email.SMTPUseSSL,
				FromAddress:         cfg.Channels.Email.FromAddress,
				AutoReplyEnabled:    cfg.Channels.Email.AutoReplyEnabled,
				PollIntervalSeconds: cfg.Channels.Email.PollIntervalSeconds,
				MarkSeen:            cfg.Channels.Email.MarkSeen,
				AllowFrom:           cfg.Channels.Email.AllowFrom,
			})
			emailChannel.SetMessageHandler(func(msg *channels.Message) {
				inboundMsg := bus.NewInboundMessage("email", msg.Sender, msg.ChatID, msg.Text)
				messageBus.PublishInbound(inboundMsg)
			})
			channelRegistry.Register(emailChannel)
		}

		// 注册 QQ（OneBot 私聊）
		if cfg.Channels.QQ.Enabled {
			qqChannel := channels.NewQQChannel(&channels.QQConfig{
				Enabled:     cfg.Channels.QQ.Enabled,
				WSURL:       cfg.Channels.QQ.WSURL,
				AccessToken: cfg.Channels.QQ.AccessToken,
				AllowFrom:   cfg.Channels.QQ.AllowFrom,
			})
			qqChannel.SetMessageHandler(func(msg *channels.Message) {
				inboundMsg := bus.NewInboundMessage("qq", msg.Sender, msg.ChatID, msg.Text)
				messageBus.PublishInbound(inboundMsg)
			})
			channelRegistry.Register(qqChannel)
		}

		// 注册 Feishu（Webhook + OpenAPI）
		if cfg.Channels.Feishu.Enabled {
			feishuChannel := channels.NewFeishuChannel(&channels.FeishuConfig{
				Enabled:           cfg.Channels.Feishu.Enabled,
				AppID:             cfg.Channels.Feishu.AppID,
				AppSecret:         cfg.Channels.Feishu.AppSecret,
				VerificationToken: cfg.Channels.Feishu.VerificationToken,
				ListenAddr:        cfg.Channels.Feishu.ListenAddr,
				WebhookPath:       cfg.Channels.Feishu.WebhookPath,
				AllowFrom:         cfg.Channels.Feishu.AllowFrom,
			})
			feishuChannel.SetMessageHandler(func(msg *channels.Message) {
				inboundMsg := bus.NewInboundMessage("feishu", msg.Sender, msg.ChatID, msg.Text)
				messageBus.PublishInbound(inboundMsg)
			})
			channelRegistry.Register(feishuChannel)
		}

		// 检查启用的频道
		enabledChannels := []string{}
		for _, ch := range channelRegistry.GetEnabled() {
			enabledChannels = append(enabledChannels, ch.Name())
		}

		if len(enabledChannels) > 0 {
			fmt.Printf("✓ Channels enabled: %v\n", enabledChannels)
			if lg := logging.Get(); lg != nil && lg.Gateway != nil {
				lg.Gateway.Printf("channels enabled: %v", enabledChannels)
			}
		} else {
			fmt.Println("⚠ Warning: No channels enabled")
			if lg := logging.Get(); lg != nil && lg.Gateway != nil {
				lg.Gateway.Printf("warning: no channels enabled")
			}
		}

		// 显示 Cron 状态
		cronStatus := cronService.Status()
		fmt.Printf("✓ Cron jobs: %d total, %d enabled\n", cronStatus["totalJobs"], cronStatus["enabledJobs"])
		if lg := logging.Get(); lg != nil && lg.Gateway != nil {
			lg.Gateway.Printf("cron jobs total=%v enabled=%v", cronStatus["totalJobs"], cronStatus["enabledJobs"])
		}

		// 启动所有服务
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// 启动 Web UI/API 服务器
		webServer := webui.NewServer(cfg, agentLoop, cronService, channelRegistry)
		go func() {
			if err := webServer.Start(ctx, cfg.Gateway.Host, gatewayPort); err != nil && err != context.Canceled {
				fmt.Printf("⚠ Web UI server error: %v\n", err)
				if lg := logging.Get(); lg != nil && lg.Web != nil {
					lg.Web.Printf("webui error: %v", err)
				}
			}
		}()

		// 设置定时任务通知处理器
		cronService.SetNotificationHandler(func(title, body string, data map[string]interface{}) {
			webServer.AddNotification(title, body, data)
		})

		fmt.Println("✓ Gateway ready")
		fmt.Println("\nPress Ctrl+C to stop")

		// 启动频道
		for _, ch := range channelRegistry.GetEnabled() {
			if err := ch.Start(ctx); err != nil {
				fmt.Printf("⚠ Failed to start %s channel: %v\n", ch.Name(), err)
				if lg := logging.Get(); lg != nil && lg.Channels != nil {
					lg.Channels.Printf("start channel=%s error=%v", ch.Name(), err)
				}
			}
		}

		// 启动 Cron 服务
		if err := cronService.Start(); err != nil {
			fmt.Printf("⚠ Failed to start cron service: %v\n", err)
			if lg := logging.Get(); lg != nil && lg.Cron != nil {
				lg.Cron.Printf("cron start error: %v", err)
			}
		}

		// 启动每日 Memory 汇总器（每小时检查一次，幂等写入 memory/MEMORY.md）
		dailySummary := memory.NewDailySummaryService(cfg.Agents.Defaults.Workspace, time.Hour)
		go dailySummary.Start(ctx)

		// 启动出站消息处理器
		go handleOutboundMessages(ctx, messageBus, channelRegistry)

		// 处理 Ctrl+C
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-sigChan
			fmt.Println("\nShutting down...")
			cancel()
		}()

		// 运行 Agent
		if err := agentLoop.Run(ctx); err != nil && err != context.Canceled {
			if lg := logging.Get(); lg != nil && lg.Gateway != nil {
				lg.Gateway.Printf("agent loop error: %v", err)
			}
			return fmt.Errorf("agent error: %w", err)
		}

		// 停止所有服务
		cronService.Stop()
		for _, ch := range channelRegistry.GetAll() {
			ch.Stop()
		}

		if lg := logging.Get(); lg != nil && lg.Gateway != nil {
			lg.Gateway.Printf("gateway shutdown")
		}

		return nil
	},
}

func buildGatewayProvider(cfg *config.Config, apiKey, apiBase string) (providers.LLMProvider, string, error) {
	if apiKey == "" {
		return &unavailableProvider{
			model:  cfg.Agents.Defaults.Model,
			reason: "no API key configured. Set one in ~/.maxclaw/config.json (or via Web UI settings) to enable model requests",
		}, "No API key configured. Gateway started in configuration-only mode; model requests will fail until key is set.", nil
	}

	provider, err := providers.NewOpenAIProvider(
		apiKey,
		apiBase,
		cfg.Agents.Defaults.Model,
		cfg.Agents.Defaults.MaxTokens,
		cfg.Agents.Defaults.Temperature,
	)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create provider: %w", err)
	}
	return provider, "", nil
}

type unavailableProvider struct {
	model  string
	reason string
}

func (p *unavailableProvider) Chat(ctx context.Context, messages []providers.Message, tools []map[string]interface{}, model string) (*providers.Response, error) {
	return nil, fmt.Errorf("%s", p.reason)
}

func (p *unavailableProvider) ChatStream(ctx context.Context, messages []providers.Message, tools []map[string]interface{}, model string, handler providers.StreamHandler) error {
	err := fmt.Errorf("%s", p.reason)
	if handler != nil {
		handler.OnError(err)
	}
	return err
}

func (p *unavailableProvider) GetDefaultModel() string {
	if p.model != "" {
		return p.model
	}
	return "gpt-4"
}

// handleOutboundMessages 处理出站消息
func handleOutboundMessages(ctx context.Context, bus *bus.MessageBus, registry *channels.Registry) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		msg, err := bus.ConsumeOutbound(ctx)
		if err != nil {
			if err == context.Canceled {
				return
			}
			continue
		}

		if msg == nil {
			continue
		}
		if msg.Channel == "" || msg.ChatID == "" {
			if lg := logging.Get(); lg != nil && lg.Gateway != nil {
				lg.Gateway.Printf("drop outbound: channel=%q chat=%q content=%q", msg.Channel, msg.ChatID, logging.Truncate(msg.Content, 200))
			}
			continue
		}

		ch, ok := registry.Get(msg.Channel)
		if !ok {
			if lg := logging.Get(); lg != nil && lg.Gateway != nil {
				lg.Gateway.Printf("drop outbound: channel %q not registered", msg.Channel)
			}
			continue
		}

		if err := ch.SendMessage(msg.ChatID, msg.Content); err != nil {
			if lg := logging.Get(); lg != nil && lg.Channels != nil {
				lg.Channels.Printf("send failed channel=%s chat=%s err=%v", msg.Channel, msg.ChatID, err)
			}
		}
	}
}
