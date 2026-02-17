package agent

import (
	"os"
	"path/filepath"

	"github.com/Lichas/nanobot-go/internal/config"
	"github.com/Lichas/nanobot-go/pkg/tools"
)

// BuildWebFetchOptions converts config to tool options and resolves default script path.
func BuildWebFetchOptions(cfg *config.Config) tools.WebFetchOptions {
	opts := tools.WebFetchOptions{
		Mode:       cfg.Tools.Web.Fetch.Mode,
		NodePath:   cfg.Tools.Web.Fetch.NodePath,
		ScriptPath: cfg.Tools.Web.Fetch.ScriptPath,
		TimeoutSec: cfg.Tools.Web.Fetch.Timeout,
		UserAgent:  cfg.Tools.Web.Fetch.UserAgent,
		WaitUntil:  cfg.Tools.Web.Fetch.WaitUntil,
		Chrome: tools.WebFetchChromeOptions{
			CDPEndpoint: cfg.Tools.Web.Fetch.Chrome.CDPEndpoint,
			ProfileName: cfg.Tools.Web.Fetch.Chrome.ProfileName,
			UserDataDir: cfg.Tools.Web.Fetch.Chrome.UserDataDir,
			Channel:     cfg.Tools.Web.Fetch.Chrome.Channel,
			Headless:    cfg.Tools.Web.Fetch.Chrome.Headless,
		},
	}

	if opts.ScriptPath == "" {
		opts.ScriptPath = findWebFetchScript()
	}

	return opts
}

func findWebFetchScript() string {
	const rel = "webfetcher/fetch.mjs"

	candidates := []string{}
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(exeDir, rel),
			filepath.Join(exeDir, "..", rel),
		)
	}
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(cwd, rel))
	}

	for _, path := range candidates {
		if stat, err := os.Stat(path); err == nil && !stat.IsDir() {
			return path
		}
	}
	return ""
}
