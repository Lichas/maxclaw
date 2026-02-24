package tools

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

var playwrightInstallMu sync.Mutex

func runNodeScriptWithPlaywrightRetry(ctx context.Context, nodePath, scriptPath string, payload []byte) ([]byte, error) {
	output, stderr, err := runNodeScript(ctx, nodePath, scriptPath, payload)
	if err == nil {
		return output, nil
	}

	if !isPlaywrightMissingModuleError(stderr) {
		msg := strings.TrimSpace(stderr)
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("%s", msg)
	}

	if installErr := ensurePlaywrightDependency(ctx, scriptPath); installErr != nil {
		return nil, fmt.Errorf(
			"Playwright dependency is missing and auto-install failed: %v. Run `make webfetch-install` and retry",
			installErr,
		)
	}

	output, stderr, err = runNodeScript(ctx, nodePath, scriptPath, payload)
	if err != nil {
		msg := strings.TrimSpace(stderr)
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("%s", msg)
	}

	return output, nil
}

func runNodeScript(ctx context.Context, nodePath, scriptPath string, payload []byte) ([]byte, string, error) {
	cmd := exec.CommandContext(ctx, nodePath, scriptPath)
	cmd.Stdin = bytes.NewReader(payload)
	cmd.Env = append(os.Environ(), "PLAYWRIGHT_BROWSERS_PATH=0")

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	return output, strings.TrimSpace(stderr.String()), err
}

func isPlaywrightMissingModuleError(stderr string) bool {
	msg := strings.ToLower(stderr)
	if msg == "" {
		return false
	}
	if strings.Contains(msg, "cannot find package 'playwright'") {
		return true
	}
	if strings.Contains(msg, "cannot find module 'playwright'") {
		return true
	}
	return strings.Contains(msg, "err_module_not_found") && strings.Contains(msg, "playwright")
}

func ensurePlaywrightDependency(ctx context.Context, scriptPath string) error {
	playwrightInstallMu.Lock()
	defer playwrightInstallMu.Unlock()

	webfetchDir := filepath.Dir(scriptPath)
	pkgJSON := filepath.Join(webfetchDir, "package.json")
	if stat, err := os.Stat(pkgJSON); err != nil || stat.IsDir() {
		return fmt.Errorf("webfetcher package.json not found in %s", webfetchDir)
	}

	playwrightPkgJSON := filepath.Join(webfetchDir, "node_modules", "playwright", "package.json")
	if stat, err := os.Stat(playwrightPkgJSON); err == nil && !stat.IsDir() {
		return nil
	}

	args := playwrightInstallArgs(webfetchDir)
	cmd := exec.CommandContext(ctx, "npm", args...)
	cmd.Dir = webfetchDir
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("npm %s failed: %v (%s)", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func playwrightInstallArgs(webfetchDir string) []string {
	if stat, err := os.Stat(filepath.Join(webfetchDir, "package-lock.json")); err == nil && !stat.IsDir() {
		return []string{"ci"}
	}
	return []string{"install"}
}
