package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Lichas/maxclaw/internal/config"
	"github.com/gorilla/websocket"
	"github.com/mdp/qrterminal/v3"
	"github.com/spf13/cobra"
)

var (
	whatsappBridgeFlag string
	whatsappTimeoutSec int
)

func init() {
	whatsappCmd.AddCommand(whatsappBindCmd)
	whatsappBindCmd.Flags().StringVar(&whatsappBridgeFlag, "bridge", "", "Bridge WebSocket URL (default from config)")
	whatsappBindCmd.Flags().IntVar(&whatsappTimeoutSec, "timeout", 180, "Timeout seconds to wait for QR/connection")
}

var whatsappCmd = &cobra.Command{
	Use:   "whatsapp",
	Short: "WhatsApp channel utilities",
}

var whatsappBindCmd = &cobra.Command{
	Use:   "bind",
	Short: "Bind WhatsApp by scanning QR code",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		bridgeURL := whatsappBridgeFlag
		if bridgeURL == "" {
			bridgeURL = cfg.Channels.WhatsApp.BridgeURL
		}
		if bridgeURL == "" {
			bridgeURL = "ws://localhost:3001"
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(whatsappTimeoutSec)*time.Second)
		defer cancel()

		fmt.Printf("%s WhatsApp bridge: %s\n", logo, bridgeURL)
		fmt.Println("Waiting for QR code...")

		conn, _, err := websocket.DefaultDialer.DialContext(ctx, bridgeURL, nil)
		if err != nil {
			return fmt.Errorf("failed to connect to bridge: %w", err)
		}
		defer conn.Close()

		if token := strings.TrimSpace(cfg.Channels.WhatsApp.BridgeToken); token != "" {
			authPayload := map[string]string{
				"type":  "auth",
				"token": token,
			}
			authData, err := json.Marshal(authPayload)
			if err != nil {
				return fmt.Errorf("failed to encode bridge auth payload: %w", err)
			}
			if err := conn.WriteMessage(websocket.TextMessage, authData); err != nil {
				return fmt.Errorf("failed to send bridge auth: %w", err)
			}
		}

		msgCh := make(chan bridgeEvent)
		errCh := make(chan error, 1)

		go func() {
			for {
				_, data, err := conn.ReadMessage()
				if err != nil {
					errCh <- err
					return
				}
				var msg bridgeEvent
				if err := json.Unmarshal(data, &msg); err != nil {
					continue
				}
				msgCh <- msg
			}
		}()

		lastQR := ""
		for {
			select {
			case <-ctx.Done():
				return fmt.Errorf("timeout waiting for QR/connection")
			case err := <-errCh:
				return fmt.Errorf("bridge connection error: %w", err)
			case msg := <-msgCh:
				switch msg.Type {
				case "qr":
					if msg.QR == "" || msg.QR == lastQR {
						continue
					}
					lastQR = msg.QR
					fmt.Println()
					fmt.Println("Scan this QR code with WhatsApp (Linked Devices):")
					fmt.Println()
					qrterminal.GenerateHalfBlock(msg.QR, qrterminal.L, cmd.OutOrStdout())
				case "status":
					if msg.Status == "connected" {
						fmt.Println("\n✅ WhatsApp connected.")
						cfg.Channels.WhatsApp.Enabled = true
						cfg.Channels.WhatsApp.BridgeURL = bridgeURL
						if err := config.SaveConfig(cfg); err != nil {
							return fmt.Errorf("failed to save config: %w", err)
						}
						fmt.Printf("Config saved: %s\n", config.GetConfigPath())
						return nil
					}
				case "error":
					if msg.Error != "" {
						fmt.Printf("Bridge error: %s\n", msg.Error)
					}
				}
			}
		}
	},
}

type bridgeEvent struct {
	Type   string `json:"type"`
	Status string `json:"status"`
	QR     string `json:"qr"`
	Error  string `json:"error"`
}
