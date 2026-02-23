package cli

import (
	"fmt"
	"os"

	"github.com/Lichas/maxclaw/internal/config"
	"github.com/Lichas/maxclaw/internal/skills"
	"github.com/spf13/cobra"
)

// onboardCmd 初始化命令
var onboardCmd = &cobra.Command{
	Use:   "onboard",
	Short: "Initialize maxclaw configuration and workspace",
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

		// 安装官方 skills
		fmt.Println("\n📦 Installing official skills from anthropics/skills...")
		installer := skills.NewInstaller(config.GetWorkspacePath())
		if err := installer.InstallOfficialSkills(); err != nil {
			// 检查是否是网络错误
			if _, ok := err.(*skills.NetworkError); ok {
				fmt.Println("  ⚠ Network issue detected. Unable to download official skills.")
				fmt.Println("\n  Troubleshooting options:")
				fmt.Println("    1. Set proxy: export HTTPS_PROXY=http://127.0.0.1:7890")
				fmt.Println("    2. Manual download: https://github.com/anthropics/skills")
				fmt.Println("    3. Retry later: maxclaw skills install --official")
			} else {
				fmt.Printf("  ⚠ Failed to install official skills: %v\n", err)
				fmt.Println("  You can manually install them later with: maxclaw skills install --official")
			}
		} else {
			// 列出已安装的 skills
			installedSkills, _ := installer.ListInstalledSkills()
			if len(installedSkills) > 0 {
				fmt.Printf("  Installed %d official skills:\n", len(installedSkills))
				for _, skill := range installedSkills {
					fmt.Printf("    - %s\n", skill)
				}
			}
		}

		fmt.Printf("\n%s maxclaw is ready!\n\n", logo)
		fmt.Println("Next steps:")
		fmt.Println("  1. Add your API key to ~/.maxclaw/config.json")
		fmt.Println("     Get one at: https://openrouter.ai/keys")
		fmt.Println("  2. Chat: maxclaw agent -m \"Hello!\"")
		fmt.Println("\nWant Telegram/WhatsApp? See: https://github.com/Lichas/maxclaw")

		return nil
	},
}
