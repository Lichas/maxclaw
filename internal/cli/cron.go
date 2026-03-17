package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Lichas/maxclaw/internal/agent"
	"github.com/Lichas/maxclaw/internal/bus"
	"github.com/Lichas/maxclaw/internal/channels"
	"github.com/Lichas/maxclaw/internal/config"
	"github.com/Lichas/maxclaw/internal/cron"
	"github.com/Lichas/maxclaw/internal/logging"
	"github.com/Lichas/maxclaw/internal/providers"
	"github.com/spf13/cobra"
)

var (
	cronName     string
	cronSchedule string
	cronMessage  string
	cronChannel  string
	cronType     string
	cronEvery    int64
	cronAt       string
	cronDeliver  bool
)

func init() {
	// cron add 命令 flags
	cronAddCmd.Flags().StringVarP(&cronName, "name", "n", "", "Job name (required)")
	cronAddCmd.Flags().StringVarP(&cronType, "type", "t", "every", "Schedule type: every, cron, once")
	cronAddCmd.Flags().StringVarP(&cronSchedule, "schedule", "s", "", "Cron expression (for type=cron)")
	cronAddCmd.Flags().Int64VarP(&cronEvery, "every", "e", 3600000, "Interval in milliseconds (for type=every)")
	cronAddCmd.Flags().StringVarP(&cronAt, "at", "a", "", "Execute at time (for type=once, format: 2006-01-02 15:04:05)")
	cronAddCmd.Flags().StringVarP(&cronMessage, "message", "m", "", "Message to send to agent (required)")
	cronAddCmd.Flags().StringVarP(&cronChannel, "channel", "c", "", "Output channel")
	cronAddCmd.Flags().BoolVarP(&cronDeliver, "deliver", "d", false, "Deliver result to channel")
	cronAddCmd.MarkFlagRequired("name")
	cronAddCmd.MarkFlagRequired("message")

	// 添加子命令
	cronCmd.AddCommand(cronAddCmd)
	cronCmd.AddCommand(cronListCmd)
	cronCmd.AddCommand(cronRemoveCmd)
	cronCmd.AddCommand(cronEnableCmd)
	cronCmd.AddCommand(cronDisableCmd)
	cronCmd.AddCommand(cronRunCmd)
	cronCmd.AddCommand(cronStatusCmd)

	rootCmd.AddCommand(cronCmd)
}

// cronCmd cron 根命令
var cronCmd = &cobra.Command{
	Use:   "cron",
	Short: "Manage scheduled jobs",
	Long:  "Add, list, remove and manage scheduled cron jobs",
}

// cronAddCmd 添加任务
var cronAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new scheduled job",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		if _, err := logging.Init(config.GetDataDir()); err != nil {
			fmt.Printf("⚠ logging init error: %v\n", err)
		}

		storePath := filepath.Join(cfg.Agents.Defaults.Workspace, ".cron", "jobs.json")
		service := cron.NewService(storePath)

		// 构建 Schedule
		schedule := cron.Schedule{}
		switch cronType {
		case "every":
			schedule.Type = cron.ScheduleTypeEvery
			schedule.EveryMs = cronEvery
		case "cron":
			if cronSchedule == "" {
				return fmt.Errorf("--schedule is required for type=cron")
			}
			schedule.Type = cron.ScheduleTypeCron
			schedule.Expr = cronSchedule
		case "once":
			if cronAt == "" {
				return fmt.Errorf("--at is required for type=once")
			}
			schedule.Type = cron.ScheduleTypeOnce
			t, err := time.Parse("2006-01-02 15:04:05", cronAt)
			if err != nil {
				return fmt.Errorf("invalid time format, use: 2006-01-02 15:04:05")
			}
			schedule.AtMs = t.UnixMilli()
		default:
			return fmt.Errorf("invalid type: %s, use: every, cron, or once", cronType)
		}

		// 构建 Payload
		payload := cron.Payload{
			Message:  cronMessage,
			Channels: []string{cronChannel},
			Deliver:  cronDeliver,
		}

		job, err := service.AddJob(cronName, schedule, payload)
		if err != nil {
			return fmt.Errorf("failed to add job: %w", err)
		}

		fmt.Printf("✓ Job added: %s (%s)\n", job.Name, job.ID)
		fmt.Printf("  Type: %s\n", job.Schedule.Type)
		switch job.Schedule.Type {
		case cron.ScheduleTypeEvery:
			fmt.Printf("  Every: %d ms\n", job.Schedule.EveryMs)
		case cron.ScheduleTypeCron:
			fmt.Printf("  Expression: %s\n", job.Schedule.Expr)
		case cron.ScheduleTypeOnce:
			fmt.Printf("  At: %s\n", time.UnixMilli(job.Schedule.AtMs).Format("2006-01-02 15:04:05"))
		}

		return nil
	},
}

// cronListCmd 列出任务
var cronListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all scheduled jobs",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		storePath := filepath.Join(cfg.Agents.Defaults.Workspace, ".cron", "jobs.json")
		service := cron.NewService(storePath)

		jobs := service.ListJobs()
		if len(jobs) == 0 {
			fmt.Println("No scheduled jobs")
			return nil
		}

		fmt.Printf("%-20s %-15s %-10s %-10s %s\n", "ID", "NAME", "TYPE", "STATUS", "NEXT RUN")
		fmt.Println(string(make([]byte, 80)))
		for _, job := range jobs {
			status := "disabled"
			if job.Enabled {
				status = "enabled"
			}
			nextRun := "-"
			if t, ok := job.GetNextRun(); ok {
				nextRun = t.Format("01-02 15:04")
			}
			fmt.Printf("%-20s %-15s %-10s %-10s %s\n", job.ID, job.Name, job.Schedule.Type, status, nextRun)
		}

		return nil
	},
}

// cronRemoveCmd 删除任务
var cronRemoveCmd = &cobra.Command{
	Use:   "remove [job-id]",
	Short: "Remove a scheduled job",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		storePath := filepath.Join(cfg.Agents.Defaults.Workspace, ".cron", "jobs.json")
		service := cron.NewService(storePath)

		if !service.RemoveJob(args[0]) {
			return fmt.Errorf("job not found: %s", args[0])
		}

		fmt.Printf("✓ Job removed: %s\n", args[0])
		return nil
	},
}

// cronEnableCmd 启用任务
var cronEnableCmd = &cobra.Command{
	Use:   "enable [job-id]",
	Short: "Enable a scheduled job",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		storePath := filepath.Join(cfg.Agents.Defaults.Workspace, ".cron", "jobs.json")
		service := cron.NewService(storePath)

		if _, ok := service.EnableJob(args[0], true); !ok {
			return fmt.Errorf("job not found: %s", args[0])
		}

		fmt.Printf("✓ Job enabled: %s\n", args[0])
		return nil
	},
}

// cronDisableCmd 禁用任务
var cronDisableCmd = &cobra.Command{
	Use:   "disable [job-id]",
	Short: "Disable a scheduled job",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		storePath := filepath.Join(cfg.Agents.Defaults.Workspace, ".cron", "jobs.json")
		service := cron.NewService(storePath)

		if _, ok := service.EnableJob(args[0], false); !ok {
			return fmt.Errorf("job not found: %s", args[0])
		}

		fmt.Printf("✓ Job disabled: %s\n", args[0])
		return nil
	},
}

// cronStatusCmd 查看状态
var cronStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show cron service status",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		storePath := filepath.Join(cfg.Agents.Defaults.Workspace, ".cron", "jobs.json")
		service := cron.NewService(storePath)

		status := service.Status()
		fmt.Printf("Cron Service Status:\n")
		fmt.Printf("  Running: %v\n", status["running"])
		fmt.Printf("  Total Jobs: %d\n", status["totalJobs"])
		fmt.Printf("  Enabled Jobs: %d\n", status["enabledJobs"])
		fmt.Printf("  Store Path: %s\n", status["storePath"])

		return nil
	},
}

// cronRunCmd 启动 cron 服务
var cronRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the cron scheduler daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		apiKey := cfg.GetAPIKey("")
		apiBase := cfg.GetAPIBase("")
		if apiKey == "" {
			return fmt.Errorf("no API key configured")
		}

		storePath := filepath.Join(cfg.Agents.Defaults.Workspace, ".cron", "jobs.json")
		service := cron.NewService(storePath)

		// 设置任务处理器
		service.SetJobHandler(func(job *cron.Job) (string, error) {
			return executeCronJob(cfg, apiKey, apiBase, service, job)
		})

		// 启动服务
		if err := service.Start(); err != nil {
			return fmt.Errorf("failed to start cron service: %w", err)
		}

		fmt.Printf("%s Cron scheduler started\n", logo)
		fmt.Printf("  Store: %s\n", storePath)
		fmt.Printf("  Jobs: %d enabled\n", service.Status()["enabledJobs"])
		fmt.Println("\nPress Ctrl+C to stop")

		// 等待中断信号
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		fmt.Println("\nShutting down cron service...")
		service.Stop()

		return nil
	},
}

// executeCronJob 执行定时任务
func executeCronJob(cfg *config.Config, apiKey, apiBase string, cronService *cron.Service, job *cron.Job) (string, error) {
	// 创建 Provider
	provider, err := providers.NewProvider(
		apiKey,
		apiBase,
		cfg.GetAPIFormat(cfg.Agents.Defaults.Model),
		cfg.Agents.Defaults.Model,
		cfg.Agents.Defaults.MaxTokens,
		cfg.Agents.Defaults.Temperature,
		cfg.SupportsImageInput,
	)
	if err != nil {
		return "", fmt.Errorf("failed to create provider: %w", err)
	}

	// 创建消息总线
	messageBus := bus.NewMessageBus(100)

	// 创建 Agent 循环
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
	// Use job's execution mode (defaults to auto for cron jobs to avoid waiting for user confirmation)
	executionMode := job.GetExecutionMode()
	if executionMode == cron.ExecutionModeAsk || executionMode == "" {
		// Cron jobs should default to auto mode to prevent hanging
		executionMode = cron.ExecutionModeAuto
	}
	agentLoop.UpdateRuntimeExecutionMode(executionMode)
	defer agentLoop.Close()

	// 执行单次任务
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// 添加用户消息
	userMsg := buildCronUserMessage(job)

	resultChan := make(chan string, 1)
	errorChan := make(chan error, 1)

	go func() {
		// 使用 agent 处理消息
		// Each cron job gets its own unique session based on job ID to prevent history mixing
		// 使用第一个渠道作为主渠道
		primaryChannel := "desktop"
		if len(job.Payload.Channels) > 0 {
			primaryChannel = job.Payload.Channels[0]
		}
		msg := bus.NewInboundMessage(primaryChannel, "cron", job.Payload.To, userMsg)
		msg.SessionKey = "cron:" + job.ID
		resp, err := agentLoop.ProcessMessage(ctx, msg)
		if err != nil {
			errorChan <- err
			return
		}
		if resp == nil {
			resultChan <- ""
			return
		}
		resultChan <- resp.Content
	}()

	var result string
	var execErr error

	select {
	case result = <-resultChan:
		// Continue to delivery logic
	case execErr = <-errorChan:
		// Continue to delivery logic with error
	case <-ctx.Done():
		return "", ctx.Err()
	}

	// Deliver result to channels if configured
	if job.Payload.Deliver && len(job.Payload.Channels) > 0 && job.Payload.To != "" {
		deliverCronResult(cfg, job, result, execErr)
	}

	if execErr != nil {
		return "", execErr
	}
	return result, nil
}

func buildCronUserMessage(job *cron.Job) string {
	if job == nil {
		return "[Cron Job] empty job"
	}
	channelPrefix := ""
	if len(job.Payload.Channels) > 0 {
		channelPrefix = fmt.Sprintf("[%s] ", strings.Join(job.Payload.Channels, ", "))
	}
	return fmt.Sprintf("%s[Cron Job: %s] %s", channelPrefix, job.Name, job.Payload.Message)
}

func enqueueCronJob(messageBus *bus.MessageBus, job *cron.Job) (string, error) {
	if messageBus == nil {
		return "", fmt.Errorf("message bus not available")
	}
	if job == nil {
		return "", fmt.Errorf("job is nil")
	}
	if len(job.Payload.Channels) == 0 {
		return "", fmt.Errorf("cron job channels is empty")
	}
	if job.Payload.To == "" {
		return "", fmt.Errorf("cron job target is empty")
	}

	// Use the first channel as the primary channel for message routing
	primaryChannel := job.Payload.Channels[0]
	msg := bus.NewInboundMessage(primaryChannel, "cron", job.Payload.To, buildCronUserMessage(job))
	if err := messageBus.PublishInbound(msg); err != nil {
		return "", fmt.Errorf("failed to enqueue cron job: %w", err)
	}
	return fmt.Sprintf("enqueued cron job %s", job.ID), nil
}

// deliverCronResult delivers cron job result to configured channels
func deliverCronResult(cfg *config.Config, job *cron.Job, result string, execErr error) {
	if len(job.Payload.Channels) == 0 || job.Payload.To == "" {
		return
	}

	// Build message content
	var content string
	if execErr != nil {
		content = fmt.Sprintf("❌ **定时任务执行失败**\n\n**任务**: %s\n**错误**: %v", job.Name, execErr)
	} else {
		if result == "" {
			content = fmt.Sprintf("✅ **定时任务完成**\n\n**任务**: %s\n\n无输出内容", job.Name)
		} else {
			// Truncate if too long
			displayResult := result
			if len(result) > 4000 {
				displayResult = result[:4000] + "\n\n...(内容已截断)"
			}
			content = fmt.Sprintf("✅ **定时任务完成**\n\n**任务**: %s\n\n**输出**:\n%s", job.Name, displayResult)
		}
	}

	// Send to each configured channel
	for _, channelName := range job.Payload.Channels {
		switch channelName {
		case "telegram":
			if cfg.Channels.Telegram.Enabled && cfg.Channels.Telegram.Token != "" {
				tgChannel := channels.NewTelegramChannel(&channels.TelegramConfig{
					Token:   cfg.Channels.Telegram.Token,
					Enabled: true,
					Proxy:   cfg.Channels.Telegram.Proxy,
				})
				if err := tgChannel.SendMessage(job.Payload.To, content); err != nil {
					if lg := logging.Get(); lg != nil && lg.Cron != nil {
						lg.Cron.Printf("cron deliver failed channel=telegram job=%s err=%v", job.ID, err)
					}
				} else {
					if lg := logging.Get(); lg != nil && lg.Cron != nil {
						lg.Cron.Printf("cron delivered channel=telegram job=%s", job.ID)
					}
				}
			}
		case "discord":
			if cfg.Channels.Discord.Enabled && cfg.Channels.Discord.Token != "" {
				dcChannel := channels.NewDiscordChannel(&channels.DiscordConfig{
					Token:   cfg.Channels.Discord.Token,
					Enabled: true,
				})
				if err := dcChannel.SendMessage(job.Payload.To, content); err != nil {
					if lg := logging.Get(); lg != nil && lg.Cron != nil {
						lg.Cron.Printf("cron deliver failed channel=discord job=%s err=%v", job.ID, err)
					}
				} else {
					if lg := logging.Get(); lg != nil && lg.Cron != nil {
						lg.Cron.Printf("cron delivered channel=discord job=%s", job.ID)
					}
				}
			}
		case "whatsapp":
			if cfg.Channels.WhatsApp.Enabled && cfg.Channels.WhatsApp.BridgeURL != "" {
				waChannel := channels.NewWhatsAppChannel(&channels.WhatsAppConfig{
					Enabled:     true,
					BridgeURL:   cfg.Channels.WhatsApp.BridgeURL,
					BridgeToken: cfg.Channels.WhatsApp.BridgeToken,
				})
				if err := waChannel.SendMessage(job.Payload.To, content); err != nil {
					if lg := logging.Get(); lg != nil && lg.Cron != nil {
						lg.Cron.Printf("cron deliver failed channel=whatsapp job=%s err=%v", job.ID, err)
					}
				} else {
					if lg := logging.Get(); lg != nil && lg.Cron != nil {
						lg.Cron.Printf("cron delivered channel=whatsapp job=%s", job.ID)
					}
				}
			}
		case "slack":
			if cfg.Channels.Slack.Enabled && cfg.Channels.Slack.BotToken != "" {
				slackChannel := channels.NewSlackChannel(&channels.SlackConfig{
					Enabled:  true,
					BotToken: cfg.Channels.Slack.BotToken,
					AppToken: cfg.Channels.Slack.AppToken,
				})
				if err := slackChannel.SendMessage(job.Payload.To, content); err != nil {
					if lg := logging.Get(); lg != nil && lg.Cron != nil {
						lg.Cron.Printf("cron deliver failed channel=slack job=%s err=%v", job.ID, err)
					}
				} else {
					if lg := logging.Get(); lg != nil && lg.Cron != nil {
						lg.Cron.Printf("cron delivered channel=slack job=%s", job.ID)
					}
				}
			}
		case "email":
			if cfg.Channels.Email.Enabled && cfg.Channels.Email.SMTPHost != "" {
				emailChannel := channels.NewEmailChannel(&channels.EmailConfig{
					Enabled:          true,
					SMTPHost:         cfg.Channels.Email.SMTPHost,
					SMTPPort:         cfg.Channels.Email.SMTPPort,
					SMTPUsername:     cfg.Channels.Email.SMTPUsername,
					SMTPPassword:     cfg.Channels.Email.SMTPPassword,
					SMTPUseTLS:       cfg.Channels.Email.SMTPUseTLS,
					SMTPUseSSL:       cfg.Channels.Email.SMTPUseSSL,
					FromAddress:      cfg.Channels.Email.FromAddress,
					AutoReplyEnabled: cfg.Channels.Email.AutoReplyEnabled,
				})
				if err := emailChannel.SendMessage(job.Payload.To, content); err != nil {
					if lg := logging.Get(); lg != nil && lg.Cron != nil {
						lg.Cron.Printf("cron deliver failed channel=email job=%s err=%v", job.ID, err)
					}
				} else {
					if lg := logging.Get(); lg != nil && lg.Cron != nil {
						lg.Cron.Printf("cron delivered channel=email job=%s", job.ID)
					}
				}
			}
		default:
			if lg := logging.Get(); lg != nil && lg.Cron != nil {
				lg.Cron.Printf("cron deliver skipped unsupported channel=%s job=%s", channelName, job.ID)
			}
		}
	}
}
