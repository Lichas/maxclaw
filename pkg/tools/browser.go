package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// BrowserToolOptions browser automation options.
type BrowserToolOptions struct {
	NodePath   string
	ScriptPath string
	TimeoutSec int
	Chrome     WebFetchChromeOptions
}

// BrowserTool provides Playwright-style browser actions via Node runner.
type BrowserTool struct {
	BaseTool
	options BrowserToolOptions
}

// NewBrowserTool creates browser tool.
func NewBrowserTool(options BrowserToolOptions) *BrowserTool {
	options = normalizeBrowserToolOptions(options)
	return &BrowserTool{
		BaseTool: BaseTool{
			name:        "browser",
			description: "Control browser pages with actions: navigate, snapshot, screenshot, act, tabs. Use for JS-heavy or login-required sites.",
			parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"action": map[string]interface{}{
						"type":        "string",
						"description": "Browser action",
						"enum":        []interface{}{"navigate", "snapshot", "screenshot", "act", "tabs", "open"},
					},
					"url": map[string]interface{}{
						"type":        "string",
						"description": "Target URL for navigate/snapshot/screenshot or tabs(new)",
					},
					"selector": map[string]interface{}{
						"type":        "string",
						"description": "CSS selector for act",
					},
					"ref": map[string]interface{}{
						"type":        "integer",
						"description": "Reference id from previous snapshot for act",
					},
					"text": map[string]interface{}{
						"type":        "string",
						"description": "Input text for act(type)",
					},
					"act": map[string]interface{}{
						"type":        "string",
						"description": "Sub action when action=act",
						"enum":        []interface{}{"click", "click_xy", "type", "press", "wait"},
					},
					"x": map[string]interface{}{
						"type":        "integer",
						"description": "X coordinate for act(click_xy)",
						"minimum":     0.0,
					},
					"y": map[string]interface{}{
						"type":        "integer",
						"description": "Y coordinate for act(click_xy)",
						"minimum":     0.0,
					},
					"key": map[string]interface{}{
						"type":        "string",
						"description": "Keyboard key for act(press), default Enter",
					},
					"wait_ms": map[string]interface{}{
						"type":        "integer",
						"description": "Wait duration in milliseconds for act(wait)",
						"minimum":     0.0,
					},
					"tab_action": map[string]interface{}{
						"type":        "string",
						"description": "Sub action when action=tabs",
						"enum":        []interface{}{"list", "switch", "close", "new"},
					},
					"tab_index": map[string]interface{}{
						"type":        "integer",
						"description": "Target tab index for tabs(switch/close)",
						"minimum":     0.0,
					},
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Output screenshot path when action=screenshot",
					},
					"full_page": map[string]interface{}{
						"type":        "boolean",
						"description": "Capture full page screenshot",
					},
					"max_chars": map[string]interface{}{
						"type":        "integer",
						"description": "Max chars for snapshot text",
						"minimum":     200.0,
						"maximum":     50000.0,
					},
					"timeout_ms": map[string]interface{}{
						"type":        "integer",
						"description": "Action timeout in milliseconds",
						"minimum":     1000.0,
						"maximum":     180000.0,
					},
				},
				"required": []string{"action"},
			},
		},
		options: options,
	}
}

type browserToolRequest struct {
	Action    string                `json:"action"`
	URL       string                `json:"url,omitempty"`
	Selector  string                `json:"selector,omitempty"`
	Ref       int                   `json:"ref,omitempty"`
	Text      string                `json:"text,omitempty"`
	Act       string                `json:"act,omitempty"`
	X         int                   `json:"x,omitempty"`
	Y         int                   `json:"y,omitempty"`
	Key       string                `json:"key,omitempty"`
	WaitMs    int                   `json:"waitMs,omitempty"`
	TabAction string                `json:"tabAction,omitempty"`
	TabIndex  int                   `json:"tabIndex,omitempty"`
	Path      string                `json:"path,omitempty"`
	FullPage  bool                  `json:"fullPage,omitempty"`
	MaxChars  int                   `json:"maxChars,omitempty"`
	TimeoutMs int                   `json:"timeoutMs,omitempty"`
	SessionID string                `json:"sessionId,omitempty"`
	Chrome    *browserChromeRequest `json:"chrome,omitempty"`
}

type browserToolResult struct {
	OK      bool                   `json:"ok"`
	Summary string                 `json:"summary,omitempty"`
	Error   string                 `json:"error,omitempty"`
	Data    map[string]interface{} `json:"data,omitempty"`
}

// Execute runs browser action.
func (t *BrowserTool) Execute(ctx context.Context, params map[string]interface{}) (string, error) {
	action := strings.ToLower(strings.TrimSpace(asString(params["action"])))
	if action == "" {
		return "", fmt.Errorf("action is required")
	}

	timeoutMs := t.options.TimeoutSec * 1000
	if timeoutMs <= 0 {
		timeoutMs = 30000
	}
	if v, ok := asInt(params["timeout_ms"]); ok && v >= 1000 && v <= 180000 {
		timeoutMs = v
	}

	req := browserToolRequest{
		Action:    action,
		URL:       strings.TrimSpace(asString(params["url"])),
		Selector:  strings.TrimSpace(asString(params["selector"])),
		Text:      asString(params["text"]),
		Act:       strings.ToLower(strings.TrimSpace(asString(params["act"]))),
		X:         -1,
		Y:         -1,
		Key:       strings.TrimSpace(asString(params["key"])),
		TabAction: strings.ToLower(strings.TrimSpace(asString(params["tab_action"]))),
		Path:      strings.TrimSpace(asString(params["path"])),
		TimeoutMs: timeoutMs,
		Chrome: &browserChromeRequest{
			CDPEndpoint:      t.options.Chrome.CDPEndpoint,
			ProfileName:      t.options.Chrome.ProfileName,
			UserDataDir:      t.options.Chrome.UserDataDir,
			Channel:          t.options.Chrome.Channel,
			Headless:         t.options.Chrome.Headless,
			AutoStartCDP:     t.options.Chrome.AutoStartCDP,
			TakeoverExisting: t.options.Chrome.TakeoverExisting,
			HostUserDataDir:  t.options.Chrome.HostUserDataDir,
			LaunchTimeoutMs:  t.options.Chrome.LaunchTimeoutMs,
		},
	}

	if v, ok := asInt(params["ref"]); ok && v > 0 {
		req.Ref = v
	}
	if v, ok := asInt(params["wait_ms"]); ok && v >= 0 {
		req.WaitMs = v
	}
	if v, ok := asInt(params["tab_index"]); ok && v >= 0 {
		req.TabIndex = v
	}
	if v, ok := asInt(params["x"]); ok && v >= 0 {
		req.X = v
	}
	if v, ok := asInt(params["y"]); ok && v >= 0 {
		req.Y = v
	}
	if v, ok := asInt(params["max_chars"]); ok && v >= 200 && v <= 50000 {
		req.MaxChars = v
	}
	if v, ok := params["full_page"].(bool); ok {
		req.FullPage = v
	}

	channel, chatID := RuntimeContextFrom(ctx)
	req.SessionID = browserSessionID(channel, chatID)
	if action == "screenshot" {
		req.Path = resolveBrowserScreenshotPath(ctx, req.Path, req.SessionID)
	}

	scriptPath := strings.TrimSpace(t.options.ScriptPath)
	if scriptPath == "" {
		return "", fmt.Errorf("browser tool requires scriptPath")
	}
	scriptPath = resolveScriptPath(scriptPath)
	if stat, err := os.Stat(scriptPath); err != nil || stat.IsDir() {
		return "", fmt.Errorf("browser script not found: %s", scriptPath)
	}

	nodePath := strings.TrimSpace(t.options.NodePath)
	if nodePath == "" {
		nodePath = "node"
	}

	payload, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to encode browser request: %w", err)
	}

	output, err := runNodeScriptWithPlaywrightRetry(ctx, nodePath, scriptPath, payload)
	if err != nil {
		return "", fmt.Errorf("browser command failed: %s", enrichBrowserExecutionError(err.Error()))
	}

	var result browserToolResult
	if err := json.Unmarshal(output, &result); err != nil {
		return "", fmt.Errorf("browser response parse error: %w", err)
	}
	if !result.OK {
		if strings.TrimSpace(result.Error) == "" {
			result.Error = "unknown browser error"
		}
		return "", fmt.Errorf("browser error: %s", result.Error)
	}

	if strings.TrimSpace(result.Summary) != "" {
		return strings.TrimSpace(result.Summary), nil
	}
	if len(result.Data) > 0 {
		buf, _ := json.MarshalIndent(result.Data, "", "  ")
		return string(buf), nil
	}
	return "browser action completed", nil
}

func normalizeBrowserToolOptions(options BrowserToolOptions) BrowserToolOptions {
	if strings.TrimSpace(options.NodePath) == "" {
		options.NodePath = "node"
	}
	if options.TimeoutSec <= 0 {
		options.TimeoutSec = 30
	}
	if strings.TrimSpace(options.Chrome.ProfileName) == "" {
		options.Chrome.ProfileName = defaultChromeProfileName
	}
	if strings.TrimSpace(options.Chrome.Channel) == "" {
		options.Chrome.Channel = defaultChromeChannel
	}
	if options.Chrome.LaunchTimeoutMs <= 0 {
		options.Chrome.LaunchTimeoutMs = defaultChromeLaunchTimeoutMs
	}
	if strings.TrimSpace(options.Chrome.CDPEndpoint) == "" && strings.TrimSpace(options.Chrome.UserDataDir) == "" {
		options.Chrome.UserDataDir = defaultChromeUserDataDir(options.Chrome.ProfileName)
	}
	if strings.TrimSpace(options.ScriptPath) == "" {
		options.ScriptPath = defaultBrowserScriptPath("")
	} else {
		options.ScriptPath = defaultBrowserScriptPath(options.ScriptPath)
	}
	return options
}

// BrowserOptionsFromWebFetch derives browser tool options from web_fetch config.
func BrowserOptionsFromWebFetch(fetch WebFetchOptions) BrowserToolOptions {
	return BrowserToolOptions{
		NodePath:   fetch.NodePath,
		ScriptPath: defaultBrowserScriptPath(fetch.ScriptPath),
		TimeoutSec: fetch.TimeoutSec,
		Chrome:     fetch.Chrome,
	}
}

func defaultBrowserScriptPath(webFetchScriptPath string) string {
	candidates := []string{}
	if strings.TrimSpace(webFetchScriptPath) != "" {
		resolved := resolveScriptPath(webFetchScriptPath)
		candidates = append(candidates, filepath.Join(filepath.Dir(resolved), "browser.mjs"))
	}
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(exeDir, "webfetcher", "browser.mjs"),
			filepath.Join(exeDir, "..", "webfetcher", "browser.mjs"),
		)
	}
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(cwd, "webfetcher", "browser.mjs"))
	}
	for _, candidate := range candidates {
		resolved := resolveScriptPath(candidate)
		if stat, err := os.Stat(resolved); err == nil && !stat.IsDir() {
			return resolved
		}
	}
	if strings.TrimSpace(webFetchScriptPath) != "" {
		return resolveScriptPath(filepath.Join(filepath.Dir(resolveScriptPath(webFetchScriptPath)), "browser.mjs"))
	}
	return resolveScriptPath("webfetcher/browser.mjs")
}

func browserSessionID(channel, chatID string) string {
	raw := strings.TrimSpace(channel) + "_" + strings.TrimSpace(chatID)
	if strings.Trim(raw, "_") == "" {
		return "default"
	}

	var b strings.Builder
	for _, ch := range raw {
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') {
			b.WriteRune(ch)
			continue
		}
		switch ch {
		case '-', '_', '.':
			b.WriteRune(ch)
		default:
			b.WriteByte('_')
		}
	}
	sanitized := strings.Trim(b.String(), "_.-")
	if sanitized == "" {
		return "default"
	}
	return sanitized
}

func resolveBrowserScreenshotPath(ctx context.Context, requestedPath, sessionID string) string {
	requestedPath = strings.TrimSpace(requestedPath)
	if requestedPath == "" {
		return defaultBrowserScreenshotPath(ctx, sessionID)
	}

	if requestedPath == "~" || strings.HasPrefix(requestedPath, "~/") || filepath.IsAbs(requestedPath) {
		return requestedPath
	}

	if sessionBase, ok := sessionBaseDirFromContext(ctx); ok {
		return filepath.Join(sessionBase, requestedPath)
	}

	return requestedPath
}

func defaultBrowserScreenshotPath(ctx context.Context, sessionID string) string {
	nowMillis := time.Now().UnixMilli()
	fileName := fmt.Sprintf("browser-%d.png", nowMillis)

	if sessionBase, ok := sessionBaseDirFromContext(ctx); ok {
		return filepath.Join(sessionBase, "screenshots", fileName)
	}

	homeDir, err := os.UserHomeDir()
	baseName := sanitizePathSegment(sessionID)
	if err == nil && strings.TrimSpace(homeDir) != "" {
		return filepath.Join(homeDir, ".maxclaw", "browser", "screenshots", fmt.Sprintf("%s-%d.png", baseName, nowMillis))
	}

	return filepath.Join(".maxclaw", "browser", "screenshots", fmt.Sprintf("%s-%d.png", baseName, nowMillis))
}

func asString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func asInt(v interface{}) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	default:
		return 0, false
	}
}

func enrichBrowserExecutionError(raw string) string {
	trimmed := strings.TrimSpace(raw)
	lower := strings.ToLower(trimmed)

	if strings.Contains(lower, "processsingleton") || strings.Contains(lower, "singletonlock") {
		return trimmed + "\nHint: Chrome profile is already in use. Close the other Chrome instance using this userDataDir, or configure tools.web.fetch.chrome.cdpEndpoint (e.g. http://127.0.0.1:9222) to reuse the running browser session."
	}

	return trimmed
}
