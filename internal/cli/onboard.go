package cli

import (
	"fmt"
	"os"

	"github.com/Lichas/nanobot-go/internal/config"
	"github.com/spf13/cobra"
)

// onboardCmd 初始化命令
var onboardCmd = &cobra.Command{
	Use:   "onboard",
	Short: "Initialize nanobot configuration and workspace",
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath := config.GetConfigPath()

		// 检查配置文件是否已存在
		if _, err := os.Stat(configPath); err == nil {
			fmt.Printf("Config already exists at %s\n", configPath)
			fmt.Print("Overwrite? (y/N): ")
			var response string
			fmt.Scanln(&response)
			if response != "y" && response != "Y" {
				return nil
			}
		}

		// 创建默认配置
		cfg := config.DefaultConfig()
		if err := config.SaveConfig(cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
		fmt.Printf("✓ Created config at %s\n", configPath)

		// 创建工作空间
		if err := config.EnsureWorkspace(); err != nil {
			return fmt.Errorf("failed to create workspace: %w", err)
		}
		fmt.Printf("✓ Created workspace at %s\n", config.GetWorkspacePath())

		// 创建模板文件
		if err := config.CreateWorkspaceTemplates(); err != nil {
			return fmt.Errorf("failed to create templates: %w", err)
		}
		fmt.Println("  Created AGENTS.md")
		fmt.Println("  Created SOUL.md")
		fmt.Println("  Created USER.md")
		fmt.Println("  Created skills/README.md")
		fmt.Println("  Created skills/example/SKILL.md")
		fmt.Println("  Created memory/MEMORY.md")
		fmt.Println("  Created memory/heartbeat.md")

		fmt.Printf("\n%s nanobot is ready!\n\n", logo)
		fmt.Println("Next steps:")
		fmt.Println("  1. Add your API key to ~/.nanobot/config.json")
		fmt.Println("     Get one at: https://openrouter.ai/keys")
		fmt.Println("  2. Chat: nanobot agent -m \"Hello!\"")
		fmt.Println("\nWant Telegram/WhatsApp? See: https://github.com/HKUDS/nanobot")

		return nil
	},
}
