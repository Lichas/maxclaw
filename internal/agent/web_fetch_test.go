package agent

import (
	"testing"

	"github.com/Lichas/nanobot-go/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestBuildWebFetchOptionsIncludesChromeConfig(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Tools.Web.Fetch.Mode = "chrome"
	cfg.Tools.Web.Fetch.NodePath = "/opt/homebrew/bin/node"
	cfg.Tools.Web.Fetch.ScriptPath = "/tmp/fetch.mjs"
	cfg.Tools.Web.Fetch.Timeout = 45
	cfg.Tools.Web.Fetch.UserAgent = "custom-agent"
	cfg.Tools.Web.Fetch.WaitUntil = "networkidle"
	cfg.Tools.Web.Fetch.Chrome = config.WebFetchChromeConfig{
		CDPEndpoint: "http://127.0.0.1:9222",
		ProfileName: "host",
		UserDataDir: "/tmp/chrome-profile",
		Channel:     "chrome-beta",
		Headless:    false,
	}

	got := BuildWebFetchOptions(cfg)
	assert.Equal(t, "chrome", got.Mode)
	assert.Equal(t, "/opt/homebrew/bin/node", got.NodePath)
	assert.Equal(t, "/tmp/fetch.mjs", got.ScriptPath)
	assert.Equal(t, 45, got.TimeoutSec)
	assert.Equal(t, "custom-agent", got.UserAgent)
	assert.Equal(t, "networkidle", got.WaitUntil)
	assert.Equal(t, "http://127.0.0.1:9222", got.Chrome.CDPEndpoint)
	assert.Equal(t, "host", got.Chrome.ProfileName)
	assert.Equal(t, "/tmp/chrome-profile", got.Chrome.UserDataDir)
	assert.Equal(t, "chrome-beta", got.Chrome.Channel)
	assert.False(t, got.Chrome.Headless)
}

func TestBuildWebFetchOptionsUsesConfigDefaults(t *testing.T) {
	cfg := config.DefaultConfig()

	got := BuildWebFetchOptions(cfg)
	assert.Equal(t, "chrome", got.Chrome.ProfileName)
	assert.Equal(t, "chrome", got.Chrome.Channel)
	assert.True(t, got.Chrome.Headless)
}
