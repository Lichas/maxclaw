package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Lichas/nanobot-go/internal/config"
	"github.com/spf13/cobra"
)

var (
	browserLoginTimeoutSec int
	browserLoginNodePath   string
	browserLoginScriptPath string
	browserLoginProfile    string
	browserLoginUserData   string
	browserLoginChannel    string
)

var browserCmd = &cobra.Command{
	Use:   "browser",
	Short: "Browser profile utilities",
}

var browserLoginCmd = &cobra.Command{
	Use:   "login [url]",
	Short: "Open managed Chrome profile for manual login",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		targetURL := "https://x.com"
		if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
			targetURL = strings.TrimSpace(args[0])
		}

		chromeCfg := cfg.Tools.Web.Fetch.Chrome
		profileName := firstNonEmpty(browserLoginProfile, chromeCfg.ProfileName, "chrome")
		channel := firstNonEmpty(browserLoginChannel, chromeCfg.Channel, "chrome")
		userDataDir := firstNonEmpty(browserLoginUserData, chromeCfg.UserDataDir)
		if strings.TrimSpace(userDataDir) == "" {
			userDataDir = defaultBrowserLoginUserDataDir(profileName)
		}
		userDataDir = expandCLIPath(userDataDir)

		nodePath := firstNonEmpty(browserLoginNodePath, cfg.Tools.Web.Fetch.NodePath, "node")
		scriptPath := strings.TrimSpace(browserLoginScriptPath)
		if scriptPath == "" {
			scriptPath = resolveBrowserLoginScriptPath(cfg.Tools.Web.Fetch.ScriptPath)
		}
		if scriptPath == "" {
			return fmt.Errorf("cannot find login script (expected webfetcher/login.mjs); set --script-path explicitly")
		}

		timeoutSec := browserLoginTimeoutSec
		if timeoutSec < 0 {
			timeoutSec = 0
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Opening managed browser profile for manual login...\n")
		fmt.Fprintf(cmd.OutOrStdout(), "URL: %s\n", targetURL)
		fmt.Fprintf(cmd.OutOrStdout(), "Profile: %s\n", profileName)
		fmt.Fprintf(cmd.OutOrStdout(), "User data dir: %s\n", userDataDir)

		commandArgs := []string{
			scriptPath,
			"--url", targetURL,
			"--profile-name", profileName,
			"--user-data-dir", userDataDir,
			"--channel", channel,
			"--timeout-sec", strconv.Itoa(timeoutSec),
		}

		run := exec.CommandContext(cmd.Context(), nodePath, commandArgs...)
		run.Env = append(os.Environ(), "PLAYWRIGHT_BROWSERS_PATH=0")
		run.Stdin = os.Stdin
		run.Stdout = cmd.OutOrStdout()
		run.Stderr = cmd.ErrOrStderr()
		if err := run.Run(); err != nil {
			return fmt.Errorf("browser login command failed: %w", err)
		}
		return nil
	},
}

func init() {
	browserCmd.AddCommand(browserLoginCmd)

	browserLoginCmd.Flags().IntVar(&browserLoginTimeoutSec, "timeout-sec", 0, "Auto close after N seconds (0 waits for Enter)")
	browserLoginCmd.Flags().StringVar(&browserLoginNodePath, "node-path", "", "Node.js binary path")
	browserLoginCmd.Flags().StringVar(&browserLoginScriptPath, "script-path", "", "Path to webfetcher/login.mjs")
	browserLoginCmd.Flags().StringVar(&browserLoginProfile, "profile", "", "Managed profile name")
	browserLoginCmd.Flags().StringVar(&browserLoginUserData, "user-data-dir", "", "Managed user data directory")
	browserLoginCmd.Flags().StringVar(&browserLoginChannel, "channel", "", "Chrome channel (chrome/chrome-beta/chrome-dev/msedge...)")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func expandCLIPath(input string) string {
	path := os.ExpandEnv(strings.TrimSpace(input))
	if path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			path = home
		}
	} else if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(home, path[2:])
		}
	}
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return path
}

func defaultBrowserLoginUserDataDir(profileName string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	if strings.TrimSpace(profileName) == "" {
		profileName = "chrome"
	}
	return filepath.Join(home, ".nanobot", "browser", profileName, "user-data")
}

func resolveBrowserLoginScriptPath(configScriptPath string) string {
	candidates := []string{}
	if strings.TrimSpace(configScriptPath) != "" {
		resolved := expandCLIPath(configScriptPath)
		candidates = append(candidates, filepath.Join(filepath.Dir(resolved), "login.mjs"))
	}

	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(exeDir, "webfetcher", "login.mjs"),
			filepath.Join(exeDir, "..", "webfetcher", "login.mjs"),
		)
	}

	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(cwd, "webfetcher", "login.mjs"))
	}

	for _, candidate := range candidates {
		resolved := expandCLIPath(candidate)
		stat, err := os.Stat(resolved)
		if err == nil && !stat.IsDir() {
			return resolved
		}
	}
	return ""
}
