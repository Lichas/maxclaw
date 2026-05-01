package cli

import (
	"bytes"
	"context"
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

// ExecuteWithOutput runs the root command writing output to the provided writer.
func ExecuteWithOutput(out *bytes.Buffer) error {
	rootCmd.SetOut(out)
	rootCmd.SetErr(out)
	return rootCmd.Execute()
}

// ExecuteWithContext runs the root command with the supplied context.
func ExecuteWithContext(ctx context.Context, out *bytes.Buffer) error {
	rootCmd.SetOut(out)
	rootCmd.SetErr(out)
	return rootCmd.ExecuteContext(ctx)
}

// ExecuteGateway runs the gateway command as a standalone binary entrypoint.
func ExecuteGateway() error {
	gatewayCmd.Use = "maxclaw-gateway"
	return gatewayCmd.Execute()
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
		fmt.Fprintf(cmd.OutOrStdout(), "%s maxclaw v%s\n", logo, version)
	},
}
