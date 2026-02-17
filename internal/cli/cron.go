package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Lichas/nanobot-go/internal/agent"
	"github.com/Lichas/nanobot-go/internal/bus"
	"github.com/Lichas/nanobot-go/internal/config"
	"github.com/Lichas/nanobot-go/internal/cron"
	"github.com/Lichas/nanobot-go/internal/logging"
	"github.com/Lichas/nanobot-go/internal/providers"
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
			Message: cronMessage,
			Channel: cronChannel,
			Deliver: cronDeliver,
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
	provider, err := providers.NewOpenAIProvider(
		apiKey,
		apiBase,
		cfg.Agents.Defaults.Model,
		cfg.Agents.Defaults.MaxTokens,
		cfg.Agents.Defaults.Temperature,
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
	)
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
		msg := bus.NewInboundMessage(job.Payload.Channel, "cron", job.Payload.To, userMsg)
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

	select {
	case result := <-resultChan:
		return result, nil
	case err := <-errorChan:
		return "", err
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func buildCronUserMessage(job *cron.Job) string {
	if job == nil {
		return "[Cron Job] empty job"
	}
	channelPrefix := ""
	if job.Payload.Channel != "" {
		channelPrefix = fmt.Sprintf("[%s] ", job.Payload.Channel)
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
	if job.Payload.Channel == "" {
		return "", fmt.Errorf("cron job channel is empty")
	}
	if job.Payload.To == "" {
		return "", fmt.Errorf("cron job target is empty")
	}

	msg := bus.NewInboundMessage(job.Payload.Channel, "cron", job.Payload.To, buildCronUserMessage(job))
	if err := messageBus.PublishInbound(msg); err != nil {
		return "", fmt.Errorf("failed to enqueue cron job: %w", err)
	}
	return fmt.Sprintf("enqueued cron job %s", job.ID), nil
}
