package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// WebSearchTool 网页搜索工具
type WebSearchTool struct {
	BaseTool
	APIKey     string
	MaxResults int
}

// NewWebSearchTool 创建网页搜索工具
func NewWebSearchTool(apiKey string, maxResults int) *WebSearchTool {
	if maxResults <= 0 {
		maxResults = 5
	}

	return &WebSearchTool{
		BaseTool: BaseTool{
			name:        "web_search",
			description: "Search the web using Brave Search API. Use for finding current information, news, or research topics.",
			parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "Search query",
						"minLength":   1,
						"maxLength":   500,
					},
					"count": map[string]interface{}{
						"type":        "integer",
						"description": "Number of results to return (1-10)",
						"minimum":     1,
						"maximum":     10,
					},
				},
				"required": []string{"query"},
			},
		},
		APIKey:     apiKey,
		MaxResults: maxResults,
	}
}

// Execute 执行网页搜索
func (t *WebSearchTool) Execute(ctx context.Context, params map[string]interface{}) (string, error) {
	query, _ := params["query"].(string)
	if query == "" {
		return "", fmt.Errorf("query is required")
	}

	if t.APIKey == "" {
		return "", fmt.Errorf("web search API key not configured")
	}

	count := t.MaxResults
	if v, ok := params["count"].(float64); ok {
		c := int(v)
		if c > 0 && c <= 10 {
			count = c
		}
	}

	// Brave Search API
	apiURL := fmt.Sprintf("https://api.search.brave.com/res/v1/web/search?q=%s&count=%d",
		url.QueryEscape(query), count)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Subscription-Token", t.APIKey)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("search request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("search API returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Web struct {
			Results []struct {
				Title string `json:"title"`
				URL   string `json:"url"`
				Desc  string `json:"description"`
			}
		} `json:"web"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to parse search result: %w", err)
	}

	if len(result.Web.Results) == 0 {
		return "No results found for: " + query, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Search results for: %s\n\n", query))
	for i, r := range result.Web.Results {
		sb.WriteString(fmt.Sprintf("%d. %s\n   URL: %s\n   %s\n\n",
			i+1, r.Title, r.URL, r.Desc))
	}

	return sb.String(), nil
}

// WebFetchTool 网页抓取工具
type WebFetchTool struct {
	BaseTool
	options WebFetchOptions
}

// WebFetchOptions 网页抓取选项
type WebFetchOptions struct {
	Mode       string
	NodePath   string
	ScriptPath string
	TimeoutSec int
	UserAgent  string
	WaitUntil  string
	Chrome     WebFetchChromeOptions
}

// WebFetchChromeOptions Chrome 抓取选项
type WebFetchChromeOptions struct {
	CDPEndpoint      string
	ProfileName      string
	UserDataDir      string
	Channel          string
	Headless         bool
	AutoStartCDP     bool
	TakeoverExisting bool
	HostUserDataDir  string
	LaunchTimeoutMs  int
}

const (
	defaultWebFetchUserAgent     = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	defaultWebFetchWaitUntil     = "domcontentloaded"
	defaultChromeProfileName     = "chrome"
	defaultChromeChannel         = "chrome"
	defaultChromeLaunchTimeoutMs = 15000
)

// NewWebFetchTool 创建网页抓取工具
func NewWebFetchTool(options WebFetchOptions) *WebFetchTool {
	options = normalizeWebFetchOptions(options)
	return &WebFetchTool{
		BaseTool: BaseTool{
			name:        "web_fetch",
			description: "Fetch and extract text content from a web page. Use for reading documentation, articles, or any web content.",
			parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"url": map[string]interface{}{
						"type":        "string",
						"description": "URL to fetch",
					},
					"max_length": map[string]interface{}{
						"type":        "integer",
						"description": "Maximum content length to return (default: 10000)",
						"minimum":     100,
						"maximum":     50000,
					},
				},
				"required": []string{"url"},
			},
		},
		options: options,
	}
}

// Execute 执行网页抓取
func (t *WebFetchTool) Execute(ctx context.Context, params map[string]interface{}) (string, error) {
	fetchURL, _ := params["url"].(string)
	if fetchURL == "" {
		return "", fmt.Errorf("url is required")
	}

	maxLength := 10000
	if v, ok := params["max_length"].(float64); ok {
		m := int(v)
		if m >= 100 && m <= 50000 {
			maxLength = m
		}
	}

	mode := strings.ToLower(strings.TrimSpace(t.options.Mode))
	if mode == "" {
		mode = "http"
	}

	switch mode {
	case "browser", "chrome":
		return t.executeBrowserFetch(ctx, fetchURL, maxLength, mode)
	default:
		return t.executeHTTPFetch(ctx, fetchURL, maxLength)
	}
}

func (t *WebFetchTool) executeHTTPFetch(ctx context.Context, fetchURL string, maxLength int) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", fetchURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", t.options.UserAgent)

	client := &http.Client{
		Timeout: time.Duration(t.options.TimeoutSec) * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/json") {
		// JSON content
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("failed to read body: %w", err)
		}
		return truncateText(string(body), maxLength), nil
	}

	// HTML content - simple text extraction
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read body: %w", err)
	}

	// Simple HTML to text conversion
	text := extractTextFromHTML(string(body))

	return truncateText(text, maxLength), nil
}

func (t *WebFetchTool) executeBrowserFetch(ctx context.Context, fetchURL string, maxLength int, mode string) (string, error) {
	scriptPath := strings.TrimSpace(t.options.ScriptPath)
	if scriptPath == "" {
		return "", fmt.Errorf("web_fetch browser/chrome mode requires tools.web.fetch.scriptPath")
	}
	scriptPath = resolveScriptPath(scriptPath)
	if stat, err := os.Stat(scriptPath); err != nil || stat.IsDir() {
		return "", fmt.Errorf("web_fetch script not found: %s", scriptPath)
	}

	nodePath := strings.TrimSpace(t.options.NodePath)
	if nodePath == "" {
		nodePath = "node"
	}

	req := browserFetchRequest{
		URL:       fetchURL,
		TimeoutMs: t.options.TimeoutSec * 1000,
		UserAgent: t.options.UserAgent,
		WaitUntil: t.options.WaitUntil,
		Mode:      mode,
	}
	if mode == "chrome" {
		req.Chrome = &browserChromeRequest{
			CDPEndpoint:      t.options.Chrome.CDPEndpoint,
			ProfileName:      t.options.Chrome.ProfileName,
			UserDataDir:      t.options.Chrome.UserDataDir,
			Channel:          t.options.Chrome.Channel,
			Headless:         t.options.Chrome.Headless,
			AutoStartCDP:     t.options.Chrome.AutoStartCDP,
			TakeoverExisting: t.options.Chrome.TakeoverExisting,
			HostUserDataDir:  t.options.Chrome.HostUserDataDir,
			LaunchTimeoutMs:  t.options.Chrome.LaunchTimeoutMs,
		}
	}
	payload, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to encode browser fetch request: %w", err)
	}

	cmd := exec.CommandContext(ctx, nodePath, scriptPath)
	cmd.Stdin = bytes.NewReader(payload)
	cmd.Env = append(os.Environ(), "PLAYWRIGHT_BROWSERS_PATH=0")

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		out := strings.TrimSpace(stderr.String())
		if out == "" {
			out = err.Error()
		}
		return "", fmt.Errorf("browser fetch failed: %s", out)
	}

	var result browserFetchResult
	if err := json.Unmarshal(output, &result); err != nil {
		return "", fmt.Errorf("browser fetch parse error: %w", err)
	}
	if !result.OK {
		if result.Error == "" {
			result.Error = "unknown browser fetch error"
		}
		return "", fmt.Errorf("browser fetch error: %s", result.Error)
	}

	text := strings.TrimSpace(result.Text)
	if result.Title != "" {
		text = result.Title + "\n\n" + text
	}
	return truncateText(text, maxLength), nil
}

type browserFetchRequest struct {
	URL       string                `json:"url"`
	TimeoutMs int                   `json:"timeoutMs"`
	UserAgent string                `json:"userAgent,omitempty"`
	WaitUntil string                `json:"waitUntil,omitempty"`
	Mode      string                `json:"mode,omitempty"`
	Chrome    *browserChromeRequest `json:"chrome,omitempty"`
}

type browserChromeRequest struct {
	CDPEndpoint      string `json:"cdpEndpoint,omitempty"`
	ProfileName      string `json:"profileName,omitempty"`
	UserDataDir      string `json:"userDataDir,omitempty"`
	Channel          string `json:"channel,omitempty"`
	Headless         bool   `json:"headless"`
	AutoStartCDP     bool   `json:"autoStartCDP"`
	TakeoverExisting bool   `json:"takeoverExisting"`
	HostUserDataDir  string `json:"hostUserDataDir,omitempty"`
	LaunchTimeoutMs  int    `json:"launchTimeoutMs,omitempty"`
}

type browserFetchResult struct {
	OK    bool   `json:"ok"`
	URL   string `json:"url,omitempty"`
	Title string `json:"title,omitempty"`
	Text  string `json:"text,omitempty"`
	Error string `json:"error,omitempty"`
}

func normalizeWebFetchOptions(options WebFetchOptions) WebFetchOptions {
	if strings.TrimSpace(options.Mode) == "" {
		options.Mode = "http"
	}
	if options.TimeoutSec <= 0 {
		options.TimeoutSec = 30
	}
	if strings.TrimSpace(options.UserAgent) == "" {
		options.UserAgent = defaultWebFetchUserAgent
	}
	if strings.TrimSpace(options.WaitUntil) == "" {
		options.WaitUntil = defaultWebFetchWaitUntil
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
	if strings.EqualFold(strings.TrimSpace(options.Mode), "chrome") &&
		strings.TrimSpace(options.Chrome.CDPEndpoint) == "" &&
		strings.TrimSpace(options.Chrome.UserDataDir) == "" {
		options.Chrome.UserDataDir = defaultChromeUserDataDir(options.Chrome.ProfileName)
	}
	return options
}

func defaultChromeUserDataDir(profileName string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	if strings.TrimSpace(profileName) == "" {
		profileName = defaultChromeProfileName
	}
	return filepath.Join(home, ".nanobot", "browser", profileName, "user-data")
}

func resolveScriptPath(path string) string {
	path = os.ExpandEnv(path)
	if strings.HasPrefix(path, "~") {
		if home, err := os.UserHomeDir(); err == nil {
			if path == "~" {
				path = home
			} else if strings.HasPrefix(path, "~/") {
				path = filepath.Join(home, path[2:])
			}
		}
	}
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return path
}

func truncateText(text string, maxLength int) string {
	if maxLength <= 0 {
		return text
	}
	if len(text) <= maxLength {
		return text
	}
	return text[:maxLength] + "\n\n... (content truncated)"
}

// extractTextFromHTML 简单的 HTML 到文本提取
func extractTextFromHTML(html string) string {
	// 移除 script 和 style 标签及其内容
	html = removeTag(html, "script")
	html = removeTag(html, "style")

	// 替换常见标签为换行
	replacements := map[string]string{
		"</p>":   "\n\n",
		"<br>":   "\n",
		"<br/>":  "\n",
		"<br />": "\n",
		"</div>": "\n",
		"</h1>":  "\n\n",
		"</h2>":  "\n\n",
		"</h3>":  "\n\n",
		"</h4>":  "\n\n",
		"</h5>":  "\n\n",
		"</h6>":  "\n\n",
		"</li>":  "\n",
	}

	for tag, replacement := range replacements {
		html = strings.ReplaceAll(html, tag, replacement)
	}

	// 移除所有其他 HTML 标签
	inTag := false
	var result strings.Builder
	for _, r := range html {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			result.WriteRune(r)
		}
	}

	// 规范化空白字符
	text := result.String()
	text = strings.ReplaceAll(text, "&nbsp;", " ")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&quot;", `"`)

	// 合并多个换行
	for strings.Contains(text, "\n\n\n") {
		text = strings.ReplaceAll(text, "\n\n\n", "\n\n")
	}

	return strings.TrimSpace(text)
}

// removeTag 移除指定标签及其内容
func removeTag(html, tag string) string {
	startTag := "<" + tag
	endTag := "</" + tag + ">"

	for {
		start := strings.Index(strings.ToLower(html), startTag)
		if start == -1 {
			break
		}

		end := strings.Index(html[start:], ">")
		if end == -1 {
			break
		}
		end += start + 1

		// 检查是否是自闭合标签
		if html[end-2:end-1] == "/" {
			html = html[:start] + html[end:]
			continue
		}

		// 查找结束标签
		endStart := strings.Index(strings.ToLower(html[end:]), endTag)
		if endStart == -1 {
			break
		}
		endStart += end

		html = html[:start] + html[endStart+len(endTag):]
	}

	return html
}
