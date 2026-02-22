package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	version = "0.1.0"
	logo    = `🤖`
)

// rootCmd 根命令
var rootCmd = &cobra.Command{
	Use:   "maxclaw",
	Short: "maxclaw - Personal AI Assistant",
	Long:  fmt.Sprintf("%s maxclaw - Ultra-Lightweight Personal AI Assistant", logo),
}

// Execute 执行根命令
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(onboardCmd)
	rootCmd.AddCommand(agentCmd)
	rootCmd.AddCommand(browserCmd)
	rootCmd.AddCommand(gatewayCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(whatsappCmd)
	rootCmd.AddCommand(telegramCmd)
	rootCmd.AddCommand(skillsCmd)
	rootCmd.AddCommand(versionCmd)
}

// versionCmd 版本命令
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("%s maxclaw v%s\n", logo, version)
	},
}
