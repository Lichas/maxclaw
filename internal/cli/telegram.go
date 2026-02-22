package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Lichas/maxclaw/internal/config"
	"github.com/mdp/qrterminal/v3"
	"github.com/spf13/cobra"
)

var (
	telegramTokenFlag string
)

func init() {
	telegramCmd.AddCommand(telegramBindCmd)
	telegramBindCmd.Flags().StringVar(&telegramTokenFlag, "token", "", "Telegram bot token from @BotFather")
	_ = telegramBindCmd.MarkFlagRequired("token")
}

var telegramCmd = &cobra.Command{
	Use:   "telegram",
	Short: "Telegram channel utilities",
}

var telegramBindCmd = &cobra.Command{
	Use:   "bind",
	Short: "Bind Telegram bot token and print QR link",
	RunE: func(cmd *cobra.Command, args []string) error {
		info, err := telegramGetMe(telegramTokenFlag)
		if err != nil {
			return err
		}

		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		cfg.Channels.Telegram.Enabled = true
		cfg.Channels.Telegram.Token = telegramTokenFlag
		if err := config.SaveConfig(cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		fmt.Printf("%s Telegram bot: @%s\n", logo, info.Username)
		fmt.Printf("Config saved: %s\n", config.GetConfigPath())

		if info.Username != "" {
			link := fmt.Sprintf("https://t.me/%s", info.Username)
			fmt.Println("\nScan this QR to open the bot chat:")
			qrterminal.GenerateHalfBlock(link, qrterminal.L, cmd.OutOrStdout())
		}

		return nil
	},
}

type telegramBotInfo struct {
	ID       int64
	Username string
	Name     string
}

func telegramGetMe(token string) (*telegramBotInfo, error) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/getMe", token)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("telegram getMe failed: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		OK     bool `json:"ok"`
		Result struct {
			ID        int64  `json:"id"`
			Username  string `json:"username"`
			FirstName string `json:"first_name"`
		} `json:"result"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("telegram getMe parse failed: %w", err)
	}
	if !result.OK {
		if result.Description == "" {
			result.Description = "invalid token"
		}
		return nil, fmt.Errorf("telegram getMe error: %s", result.Description)
	}

	return &telegramBotInfo{
		ID:       result.Result.ID,
		Username: result.Result.Username,
		Name:     result.Result.FirstName,
	}, nil
}
