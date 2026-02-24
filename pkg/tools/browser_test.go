package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBrowserOptionsFromWebFetch(t *testing.T) {
	opts := BrowserOptionsFromWebFetch(WebFetchOptions{
		NodePath:   "/opt/node",
		ScriptPath: "/repo/webfetcher/fetch.mjs",
		TimeoutSec: 40,
		Chrome: WebFetchChromeOptions{
			ProfileName: "work",
			Channel:     "chrome-beta",
		},
	})

	assert.Equal(t, "/opt/node", opts.NodePath)
	assert.Equal(t, filepath.Clean("/repo/webfetcher/browser.mjs"), filepath.Clean(opts.ScriptPath))
	assert.Equal(t, 40, opts.TimeoutSec)
	assert.Equal(t, "work", opts.Chrome.ProfileName)
	assert.Equal(t, "chrome-beta", opts.Chrome.Channel)
}

func TestNormalizeBrowserToolOptionsDefaults(t *testing.T) {
	t.Setenv("HOME", "/tmp/maxclaw-home")

	opts := normalizeBrowserToolOptions(BrowserToolOptions{})
	assert.Equal(t, "node", opts.NodePath)
	assert.Equal(t, 30, opts.TimeoutSec)
	assert.Equal(t, "chrome", opts.Chrome.ProfileName)
	assert.Equal(t, "chrome", opts.Chrome.Channel)
	assert.Equal(t, 15000, opts.Chrome.LaunchTimeoutMs)
	assert.Equal(
		t,
		filepath.Join("/tmp/maxclaw-home", ".maxclaw", "browser", "chrome", "user-data"),
		opts.Chrome.UserDataDir,
	)
	assert.Equal(
		t,
		filepath.Join("webfetcher", "browser.mjs"),
		filepath.Join(filepath.Base(filepath.Dir(opts.ScriptPath)), filepath.Base(opts.ScriptPath)),
	)
}

func TestBrowserSessionID(t *testing.T) {
	assert.Equal(t, "telegram_chat-42", browserSessionID("telegram", "chat-42"))
	assert.Equal(t, "default", browserSessionID("", ""))
	assert.Equal(t, "weird___id", browserSessionID("weird<>", " id"))
}

func TestResolveBrowserScreenshotPathDefaultsToSessionDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	SetWorkspaceDir(tmpDir)
	t.Cleanup(func() {
		SetWorkspaceDir("")
	})

	ctx := WithRuntimeContextWithSession(context.Background(), "desktop", "desktop:task-1", "desktop:task-1")
	resolved := resolveBrowserScreenshotPath(ctx, "", "desktop_task-1")

	expectedPrefix := filepath.Join(tmpDir, ".sessions", "desktop_task-1", "screenshots") + string(os.PathSeparator)
	assert.True(t, strings.HasPrefix(resolved, expectedPrefix), "resolved=%s", resolved)
	assert.True(t, strings.HasSuffix(resolved, ".png"), "resolved=%s", resolved)
}

func TestResolveBrowserScreenshotPathResolvesRelativePathInSessionDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	SetWorkspaceDir(tmpDir)
	t.Cleanup(func() {
		SetWorkspaceDir("")
	})

	ctx := WithRuntimeContextWithSession(context.Background(), "desktop", "desktop:task-2", "desktop:task-2")
	resolved := resolveBrowserScreenshotPath(ctx, "captures/home.png", "desktop_task-2")

	assert.Equal(
		t,
		filepath.Join(tmpDir, ".sessions", "desktop_task-2", "captures", "home.png"),
		resolved,
	)
}
