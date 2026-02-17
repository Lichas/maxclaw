package cli

import (
	"fmt"
	"os"

	"github.com/Lichas/nanobot-go/internal/config"
	"github.com/spf13/cobra"
)

// statusCmd 状态命令
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show nanobot status",
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath := config.GetConfigPath()
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		fmt.Printf("%s nanobot Status\n\n", logo)

		// 配置文件状态
		if _, err := os.Stat(configPath); err == nil {
			fmt.Printf("Config: %s ✓\n", configPath)
		} else {
			fmt.Printf("Config: %s ✗ (not found)\n", configPath)
		}

		// 工作空间状态
		workspace := cfg.Agents.Defaults.Workspace
		if _, err := os.Stat(workspace); err == nil {
			fmt.Printf("Workspace: %s ✓\n", workspace)
		} else {
			fmt.Printf("Workspace: %s ✗ (not found)\n", workspace)
		}

		// 模型配置
		fmt.Printf("Model: %s\n", cfg.Agents.Defaults.Model)

		// API Key 状态
		fmt.Printf("OpenRouter API: ")
		if cfg.Providers.OpenRouter.APIKey != "" {
			fmt.Println("✓")
		} else {
			fmt.Println("✗ (not set)")
		}

		fmt.Printf("Anthropic API: ")
		if cfg.Providers.Anthropic.APIKey != "" {
			fmt.Println("✓")
		} else {
			fmt.Println("✗ (not set)")
		}

		fmt.Printf("OpenAI API: ")
		if cfg.Providers.OpenAI.APIKey != "" {
			fmt.Println("✓")
		} else {
			fmt.Println("✗ (not set)")
		}

		// 频道状态
		fmt.Println("\nChannels:")
		if cfg.Channels.Telegram.Enabled {
			fmt.Println("  Telegram: ✓ enabled")
		} else {
			fmt.Println("  Telegram: ✗ disabled")
		}
		if cfg.Channels.Discord.Enabled {
			fmt.Println("  Discord: ✓ enabled")
		} else {
			fmt.Println("  Discord: ✗ disabled")
		}
		if cfg.Channels.WhatsApp.Enabled {
			fmt.Println("  WhatsApp: ✓ enabled")
		} else {
			fmt.Println("  WhatsApp: ✗ disabled")
		}
		if cfg.Channels.WebSocket.Enabled {
			fmt.Println("  WebSocket: ✓ enabled")
		} else {
			fmt.Println("  WebSocket: ✗ disabled")
		}
		if cfg.Channels.Slack.Enabled {
			fmt.Println("  Slack: ✓ enabled")
		} else {
			fmt.Println("  Slack: ✗ disabled")
		}
		if cfg.Channels.Email.Enabled {
			fmt.Println("  Email: ✓ enabled")
		} else {
			fmt.Println("  Email: ✗ disabled")
		}
		if cfg.Channels.QQ.Enabled {
			fmt.Println("  QQ: ✓ enabled")
		} else {
			fmt.Println("  QQ: ✗ disabled")
		}
		if cfg.Channels.Feishu.Enabled {
			fmt.Println("  Feishu: ✓ enabled")
		} else {
			fmt.Println("  Feishu: ✗ disabled")
		}

		return nil
	},
}
