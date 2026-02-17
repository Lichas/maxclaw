package tools

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeWebFetchOptionsChromeDefaults(t *testing.T) {
	t.Setenv("HOME", "/tmp/nanobot-home")

	opts := normalizeWebFetchOptions(WebFetchOptions{
		Mode: "chrome",
	})

	assert.Equal(t, "chrome", opts.Chrome.ProfileName)
	assert.Equal(t, "chrome", opts.Chrome.Channel)
	assert.Equal(
		t,
		filepath.Join("/tmp/nanobot-home", ".nanobot", "browser", "chrome", "user-data"),
		opts.Chrome.UserDataDir,
	)
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
}
