package tools

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeWebFetchOptionsChromeDefaults(t *testing.T) {
	t.Setenv("HOME", "/tmp/maxclaw-home")

	opts := normalizeWebFetchOptions(WebFetchOptions{
		Mode: "chrome",
	})

	assert.Equal(t, "chrome", opts.Chrome.ProfileName)
	assert.Equal(t, "chrome", opts.Chrome.Channel)
	assert.Equal(t, 15000, opts.Chrome.LaunchTimeoutMs)
	assert.Equal(
		t,
		filepath.Join("/tmp/maxclaw-home", ".maxclaw", "browser", "chrome", "user-data"),
		opts.Chrome.UserDataDir,
	)
	assert.Equal(t, 600, opts.RenderWaitMs)
	assert.Equal(t, 4000, opts.SmartWaitMs)
	assert.Equal(t, 500, opts.StableWaitMs)
}

func TestNormalizeWebFetchOptionsChromeCdpEndpointDoesNotForceUserDataDir(t *testing.T) {
	opts := normalizeWebFetchOptions(WebFetchOptions{
		Mode: "chrome",
		Chrome: WebFetchChromeOptions{
			CDPEndpoint: "http://127.0.0.1:9222",
		},
	})

	assert.Equal(t, "http://127.0.0.1:9222", opts.Chrome.CDPEndpoint)
	assert.Empty(t, opts.Chrome.UserDataDir)
	assert.Equal(t, 15000, opts.Chrome.LaunchTimeoutMs)
}

func TestShouldFallbackToBrowserFetch(t *testing.T) {
	assert.True(t, shouldFallbackToBrowserFetch("很抱歉，网站不能正常加载，请启用JavaScript后继续。"))
	assert.True(t, shouldFallbackToBrowserFetch("Access denied"))
	assert.False(t, shouldFallbackToBrowserFetch("Welcome to dashboard. Latest report is ready."))
}
