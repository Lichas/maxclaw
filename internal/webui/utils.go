package webui

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Lichas/maxclaw/internal/config"
	"github.com/Lichas/maxclaw/internal/session"
)

func parseGitHubURL(repoURL string) (string, string, string) {
	repoURL = strings.TrimSuffix(repoURL, ".git")

	// Check for /tree/ pattern (GitHub web URL with branch/path)
	if idx := strings.Index(repoURL, "/tree/"); idx != -1 {
		repoBase := repoURL[:idx]
		remainder := repoURL[idx+6:] // skip "/tree/"

		parts := strings.SplitN(remainder, "/", 2)
		branch := parts[0]
		if branch == "" {
			branch = "main"
		}

		subDir := ""
		if len(parts) > 1 {
			subDir = parts[1]
		}

		return repoBase, branch, subDir
	}

	// Check for /blob/ pattern (also convert to tree-like handling)
	if idx := strings.Index(repoURL, "/blob/"); idx != -1 {
		repoBase := repoURL[:idx]
		remainder := repoURL[idx+6:]

		parts := strings.SplitN(remainder, "/", 2)
		branch := parts[0]
		if branch == "" {
			branch = "main"
		}

		subDir := ""
		if len(parts) > 1 {
			// For blob URLs pointing to a file, get the directory
			subDir = filepath.Dir(parts[1])
			if subDir == "." {
				subDir = ""
			}
		}

		return repoBase, branch, subDir
	}

	// Default: no subdirectory, try to detect default branch
	return repoURL, "main", ""
}

// moveDirContents moves all files from srcDir to dstDir
func moveDirContents(srcDir, dstDir string) error {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(srcDir, entry.Name())
		dstPath := filepath.Join(dstDir, entry.Name())

		// Skip .git directory
		if entry.Name() == ".git" {
			continue
		}

		if err := os.Rename(srcPath, dstPath); err != nil {
			// If rename fails (cross-device), try copy
			if entry.IsDir() {
				if err := copyDir(srcPath, dstPath); err != nil {
					return err
				}
			} else {
				if err := copyFile(srcPath, dstPath); err != nil {
					return err
				}
			}
		}
	}

	// Remove the now-empty source directory
	return os.RemoveAll(srcDir)
}

// copyDir recursively copies a directory
func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// copyFile copies a single file
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

func extractRepoName(repoURL string) string {
	// Handle various GitHub URL formats
	// https://github.com/user/repo
	// https://github.com/user/repo.git
	// git@github.com:user/repo.git

	repoURL = strings.TrimSuffix(repoURL, ".git")

	if idx := strings.LastIndex(repoURL, "/"); idx != -1 && idx < len(repoURL)-1 {
		return repoURL[idx+1:]
	}

	// Handle git@ format
	if idx := strings.LastIndex(repoURL, ":"); idx != -1 && idx < len(repoURL)-1 {
		part := repoURL[idx+1:]
		if slashIdx := strings.LastIndex(part, "/"); slashIdx != -1 {
			return part[slashIdx+1:]
		}
		return part
	}

	return ""
}

func wantsStreamResponse(r *http.Request, payload messagePayload) bool {
	if payload.Stream {
		return true
	}
	if stream := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("stream"))); stream == "1" || stream == "true" || stream == "yes" {
		return true
	}
	accept := strings.ToLower(r.Header.Get("Accept"))
	return strings.Contains(accept, "text/event-stream")
}

// configUpdateRequest 配置更新请求，支持动态 providers
type configUpdateRequest struct {
	Agents    *config.AgentsConfig             `json:"agents,omitempty"`
	Channels  *config.ChannelsConfig           `json:"channels,omitempty"`
	Providers map[string]config.ProviderConfig `json:"providers,omitempty"`
	Gateway   *config.GatewayConfig            `json:"gateway,omitempty"`
	Tools     *config.ToolsConfig              `json:"tools,omitempty"`
}

func listSessions(workspace string) ([]sessionSummary, error) {
	dir := filepath.Join(workspace, ".sessions")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []sessionSummary{}, nil
		}
		return nil, err
	}

	mgr := session.NewManager(workspace)
	var results []sessionSummary
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var sess session.Session
		if err := json.Unmarshal(data, &sess); err != nil {
			continue
		}
		if session.RefreshTitle(&sess) {
			_ = mgr.Save(&sess)
		}
		summary := sessionSummary{
			Key:          sess.Key,
			MessageCount: len(sess.Messages),
			Title:        sess.Title,
		}
		if len(sess.Messages) > 0 {
			last := sess.Messages[len(sess.Messages)-1]
			summary.LastMessage = last.Content
			summary.LastMessageAt = last.Timestamp.Format(time.RFC3339)
		}
		results = append(results, summary)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].LastMessageAt > results[j].LastMessageAt
	})

	return results, nil
}

func summarizeSkillBody(body string, maxRunes int) string {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return ""
	}
	firstLine := strings.SplitN(trimmed, "\n", 2)[0]
	if maxRunes <= 0 || utf8.RuneCountInString(firstLine) <= maxRunes {
		return firstLine
	}
	return string([]rune(firstLine)[:maxRunes]) + "..."
}

type sessionSummary struct {
	Key           string `json:"key"`
	Title         string `json:"title,omitempty"`
	MessageCount  int    `json:"messageCount"`
	LastMessageAt string `json:"lastMessageAt,omitempty"`
	LastMessage   string `json:"lastMessage,omitempty"`
}

// ProviderTestRequest represents a provider test request
type ProviderTestRequest struct {
	Name      string `json:"name"`
	APIKey    string `json:"apiKey"`
	BaseURL   string `json:"baseURL,omitempty"`
	APIFormat string `json:"apiFormat"`
}

type providerProbePayload struct {
	Model    string `json:"model"`
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
	MaxTokens   int     `json:"max_tokens"`
	Temperature float64 `json:"temperature"`
}

func readChannelSenderStats(logPath, filterChannel string, limit int) ([]channelSenderStat, error) {
	file, err := os.Open(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []channelSenderStat{}, nil
		}
		return nil, err
	}
	defer file.Close()

	seen := map[string]channelSenderStat{}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		entry, ok := parseInboundSenderLogLine(scanner.Text())
		if !ok || strings.TrimSpace(entry.Sender) == "" {
			continue
		}
		if filterChannel != "" && entry.Channel != filterChannel {
			continue
		}

		key := entry.Channel + "\x00" + entry.Sender
		prev, exists := seen[key]
		entry.MessageCount = prev.MessageCount + 1
		if !exists || entry.LastSeen >= prev.LastSeen {
			seen[key] = entry
			continue
		}
		prev.MessageCount++
		seen[key] = prev
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	users := make([]channelSenderStat, 0, len(seen))
	for _, entry := range seen {
		users = append(users, entry)
	}
	sort.Slice(users, func(i, j int) bool {
		return users[i].LastSeen > users[j].LastSeen
	})
	if limit > 0 && len(users) > limit {
		users = users[:limit]
	}
	return users, nil
}

func parseInboundSenderLogLine(line string) (channelSenderStat, bool) {
	const marker = " inbound channel="
	idx := strings.Index(line, marker)
	if idx <= 0 {
		return channelSenderStat{}, false
	}

	timestampRaw := strings.TrimSpace(line[:idx])
	lastSeen, err := time.Parse("2006/01/02 15:04:05.000000", timestampRaw)
	if err != nil {
		return channelSenderStat{}, false
	}

	rest := line[idx+len(marker):]
	channelPart, rest, ok := strings.Cut(rest, " ")
	if !ok || strings.TrimSpace(channelPart) == "" {
		return channelSenderStat{}, false
	}
	chatPart, rest, ok := strings.Cut(rest, " ")
	if !ok || !strings.HasPrefix(chatPart, "chat=") {
		return channelSenderStat{}, false
	}
	senderPart, contentPart, ok := strings.Cut(rest, " ")
	if !ok || !strings.HasPrefix(senderPart, "sender=") || !strings.HasPrefix(contentPart, "content=") {
		return channelSenderStat{}, false
	}

	latestMessage := strings.TrimPrefix(contentPart, "content=")
	if unquoted, err := strconv.Unquote(latestMessage); err == nil {
		latestMessage = unquoted
	}

	return channelSenderStat{
		Channel:       strings.ToLower(strings.TrimSpace(channelPart)),
		Sender:        strings.TrimPrefix(senderPart, "sender="),
		ChatID:        strings.TrimPrefix(chatPart, "chat="),
		LastSeen:      lastSeen.Format(time.RFC3339Nano),
		MessageCount:  1,
		LatestMessage: latestMessage,
	}, true
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": err.Error(),
	})
}

func spaHandler(uiDir string) http.Handler {
	if uiDir == "" {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("Web UI not built"))
		})
	}

	fs := http.Dir(uiDir)
	fileServer := http.FileServer(fs)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		path := r.URL.Path
		if path == "/" {
			path = "/index.html"
		}

		f, err := fs.Open(path)
		if err != nil {
			// SPA fallback
			r.URL.Path = "/index.html"
			fileServer.ServeHTTP(w, r)
			return
		}
		_ = f.Close()
		fileServer.ServeHTTP(w, r)
	})
}

func findUIDir() string {
	candidates := []string{}

	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(exeDir, "webui", "dist"),
			filepath.Join(exeDir, "..", "webui", "dist"),
		)
	}

	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(cwd, "webui", "dist"))
	}

	for _, dir := range candidates {
		if stat, err := os.Stat(dir); err == nil && stat.IsDir() {
			return dir
		}
	}

	return ""
}

func findRestartScript() (string, string, error) {
	var roots []string
	if envRoot := os.Getenv("MAXCLAW_ROOT"); envRoot != "" {
		roots = append(roots, envRoot)
	}
	if envRoot := os.Getenv("NANOBOT_ROOT"); envRoot != "" {
		roots = append(roots, envRoot)
	}
	if cwd, err := os.Getwd(); err == nil {
		roots = append(roots, cwd)
	}
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		roots = append(roots, exeDir, filepath.Join(exeDir, ".."))
	}

	seen := make(map[string]struct{}, len(roots))
	for _, root := range roots {
		cleanRoot := filepath.Clean(root)
		if _, ok := seen[cleanRoot]; ok {
			continue
		}
		seen[cleanRoot] = struct{}{}

		script := filepath.Join(cleanRoot, "scripts", "restart_daemon.sh")
		if stat, err := os.Stat(script); err == nil && !stat.IsDir() {
			return cleanRoot, script, nil
		}
	}

	return "", "", fmt.Errorf("restart script not found")
}

// handleWhatsAppStatus returns WhatsApp channel status including QR code
