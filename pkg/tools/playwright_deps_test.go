package tools

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsPlaywrightMissingModuleError(t *testing.T) {
	stack := `node:internal/modules/package_json_reader:267
throw new ERR_MODULE_NOT_FOUND(packageName, fileURLToPath(base), null);
Error [ERR_MODULE_NOT_FOUND]: Cannot find package 'playwright' imported from /tmp/webfetcher/browser.mjs`
	assert.True(t, isPlaywrightMissingModuleError(stack))
	assert.True(t, isPlaywrightMissingModuleError("Cannot find module 'playwright'"))
	assert.False(t, isPlaywrightMissingModuleError("browser command failed: timeout"))
}

func TestPlaywrightInstallArgs(t *testing.T) {
	t.Run("uses ci when lockfile exists", func(t *testing.T) {
		dir := t.TempDir()
		lock := filepath.Join(dir, "package-lock.json")
		require.NoError(t, os.WriteFile(lock, []byte(`{}`), 0644))
		assert.Equal(t, []string{"ci"}, playwrightInstallArgs(dir))
	})

	t.Run("falls back to install without lockfile", func(t *testing.T) {
		dir := t.TempDir()
		assert.Equal(t, []string{"install"}, playwrightInstallArgs(dir))
	})
}
