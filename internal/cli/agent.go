package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"

	"github.com/Lichas/maxclaw/internal/agent"
	"github.com/Lichas/maxclaw/internal/bus"
	"github.com/Lichas/maxclaw/internal/config"
	"github.com/Lichas/maxclaw/internal/cron"
	"github.com/Lichas/maxclaw/internal/logging"
	"github.com/Lichas/maxclaw/internal/providers"
	"github.com/peterh/liner"
	"github.com/spf13/cobra"
)

var (
	messageFlag    string
	sessionIDFlag  string
	markdownFlag   bool
	logsFlag       bool
	noMarkdownFlag bool
	noLogsFlag     bool
)

func normalizeInteractiveInput(input string) string {
	return strings.TrimSpace(strings.TrimRight(input, "\r\n"))
}

func isExitCommand(input string) bool {
	switch strings.ToLower(strings.TrimSpace(input)) {
	case "exit", "quit":
		return true
	default:
		return false
	}
}

func resolveCLIChannel(renderMarkdown bool) string {
	if renderMarkdown {
		return "cli"
	}
	return "cli_plain"
}

var (
	mdHeadingRe  = regexp.MustCompile(`(?m)^\s{0,3}#{1,6}\s*`)
	mdLinkRe     = regexp.MustCompile(`\[(.*?)\]\((.*?)\)`)
	mdEmphasisRe = regexp.MustCompile(`[*_~` + "`" + `]+`)
)

func stripMarkdown(input string) string {
	out := mdHeadingRe.ReplaceAllString(input, "")
	out = mdLinkRe.ReplaceAllString(out, "$1 ($2)")
	out = mdEmphasisRe.ReplaceAllString(out, "")
	return strings.TrimSpace(out)
}

func formatAgentResponse(content string, renderMarkdown bool) string {
	if renderMarkdown {
		return content
	}
	return stripMarkdown(content)
}

func init() {
	agentCmd.Flags().StringVarP(&messageFlag, "message", "m", "", "Message to send to the agent")
	agentCmd.Flags().StringVarP(&sessionIDFlag, "session", "s", "cli:direct", "Session ID")
	agentCmd.Flags().BoolVar(&markdownFlag, "markdown", true, "Render assistant output as Markdown")
	agentCmd.Flags().BoolVar(&logsFlag, "logs", false, "Show runtime log file paths")
	agentCmd.Flags().BoolVar(&noMarkdownFlag, "no-markdown", false, "Disable markdown rendering")
	agentCmd.Flags().BoolVar(&noLogsFlag, "no-logs", false, "Hide runtime log file paths")
}

// agentCmd Agent 命令
var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Interact with the agent directly",
	RunE: func(cmd *cobra.Command, args []string) error {
		if noMarkdownFlag {
			markdownFlag = false
		}
		if noLogsFlag {
			logsFlag = false
		}

		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		if _, err := logging.Init(config.GetDataDir()); err != nil {
			fmt.Printf("⚠ logging init error: %v\n", err)
		}
		if logsFlag {
			fmt.Printf("Logs: %s\n", config.GetLogsDir())
		}

		// 检查 API key
		apiKey := cfg.GetAPIKey("")
		apiBase := cfg.GetAPIBase("")
		if apiKey == "" {
			return fmt.Errorf("no API key configured. Set one in ~/.maxclaw/config.json")
		}

		// 创建 Provider
		provider, err := providers.NewOpenAIProvider(
			apiKey,
			apiBase,
			cfg.Agents.Defaults.Model,
			cfg.Agents.Defaults.MaxTokens,
			cfg.Agents.Defaults.Temperature,
		)
		if err != nil {
			return fmt.Errorf("failed to create provider: %w", err)
		}

		// 创建组件
		messageBus := bus.NewMessageBus(100)

		// 创建 Cron 服务（agent 模式下也需要，但不启动）
		storePath := filepath.Join(cfg.Agents.Defaults.Workspace, ".cron", "jobs.json")
		cronService := cron.NewService(storePath)

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
		defer agentLoop.Close()

		if messageFlag != "" {
			// 单条消息模式
			ctx := context.Background()
			channel := resolveCLIChannel(markdownFlag)
			response, err := agentLoop.ProcessDirect(ctx, messageFlag, sessionIDFlag, channel, "direct")
			if err != nil {
				return err
			}
			// 流式输出已实时显示，仅在有内容时打印
			if response != "" {
				fmt.Printf("\n%s %s\n", logo, formatAgentResponse(response, markdownFlag))
			}
		} else {
			// 交互模式
			fmt.Printf("%s Interactive mode (type 'exit' or Ctrl+C to exit)\n\n", logo)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// 处理 Ctrl+C
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
			go func() {
				<-sigChan
				cancel()
			}()

			line := liner.NewLiner()
			defer line.Close()
			line.SetCtrlCAborts(true)

			historyPath := filepath.Join(config.GetDataDir(), ".agent_history")
			if f, err := os.Open(historyPath); err == nil {
				_, _ = line.ReadHistory(f)
				_ = f.Close()
			}
			defer func() {
				_ = os.MkdirAll(filepath.Dir(historyPath), 0755)
				if f, err := os.Create(historyPath); err == nil {
					_, _ = line.WriteHistory(f)
					_ = f.Close()
				}
			}()

			channel := resolveCLIChannel(markdownFlag)
			for {
				select {
				case <-ctx.Done():
					fmt.Println("\nGoodbye!")
					return nil
				default:
				}

				input, err := line.Prompt("You: ")
				if err != nil {
					if errors.Is(err, os.ErrClosed) {
						fmt.Println("\nGoodbye!")
						return nil
					}
					if errors.Is(err, liner.ErrPromptAborted) {
						fmt.Println("\nGoodbye!")
						return nil
					}
					if errors.Is(err, context.Canceled) {
						fmt.Println("\nGoodbye!")
						return nil
					}
					if errors.Is(err, io.EOF) {
						fmt.Println("\nGoodbye!")
						return nil
					}
					return err
				}

				input = normalizeInteractiveInput(input)
				if input == "" {
					continue
				}
				line.AppendHistory(input)
				if isExitCommand(input) {
					fmt.Println("Goodbye!")
					return nil
				}

				response, err := agentLoop.ProcessDirect(ctx, input, sessionIDFlag, channel, "direct")
				if err != nil {
					fmt.Printf("Error: %v\n", err)
					continue
				}

				// 流式输出已实时显示，仅在有内容时打印
				if response != "" {
					fmt.Printf("\n%s %s\n\n", logo, formatAgentResponse(response, markdownFlag))
				} else {
					fmt.Println()
				}
			}
		}

		return nil
	},
}
