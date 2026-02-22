package webui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Lichas/nanobot-go/internal/agent"
	"github.com/Lichas/nanobot-go/internal/channels"
	"github.com/Lichas/nanobot-go/internal/config"
	"github.com/Lichas/nanobot-go/internal/cron"
	"github.com/Lichas/nanobot-go/internal/logging"
	"github.com/Lichas/nanobot-go/internal/session"
	workspaceSkills "github.com/Lichas/nanobot-go/internal/skills"
)

type Server struct {
	cfg               *config.Config
	agentLoop         *agent.AgentLoop
	cronService       *cron.Service
	channelRegistry   *channels.Registry
	server            *http.Server
	uiDir             string
	skillsStateMgr    *workspaceSkills.StateManager
	notificationStore *NotificationStore
	wsHub             *WebSocketHub
}

type messagePayload struct {
	SessionKey     string   `json:"sessionKey"`
	Content        string   `json:"content"`
	Channel        string   `json:"channel"`
	ChatID         string   `json:"chatId"`
	SelectedSkills []string `json:"selectedSkills,omitempty"`
	Stream         bool     `json:"stream,omitempty"`
}

func NewServer(cfg *config.Config, agentLoop *agent.AgentLoop, cronService *cron.Service, registry *channels.Registry) *Server {
	s := &Server{
		cfg:               cfg,
		agentLoop:         agentLoop,
		cronService:       cronService,
		channelRegistry:   registry,
		uiDir:             findUIDir(),
		skillsStateMgr:    workspaceSkills.NewStateManager(filepath.Join(cfg.Agents.Defaults.Workspace, ".skills_state.json")),
		notificationStore: NewNotificationStore(),
		wsHub:             NewWebSocketHub(),
	}

	// Start WebSocket hub
	go s.wsHub.Run()

	return s
}

func (s *Server) Start(ctx context.Context, host string, port int) error {
	addr := fmt.Sprintf("%s:%d", host, port)
	mux := http.NewServeMux()

	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/sessions", s.handleSessions)
	mux.HandleFunc("/api/sessions/", s.handleSessionByKey)
	mux.HandleFunc("/api/skills", s.handleSkills)
	mux.HandleFunc("/api/skills/", s.handleSkillsPath)
	mux.HandleFunc("/api/skills/install", s.handleSkillsInstall)
	mux.HandleFunc("/api/message", s.handleMessage)
	mux.HandleFunc("/api/config", s.handleConfig)
	mux.HandleFunc("/api/gateway/restart", s.handleGatewayRestart)
	mux.HandleFunc("/api/cron", s.handleCron)
	mux.HandleFunc("/api/cron/", s.handleCronByID)
	mux.HandleFunc("/api/upload", s.handleUpload)
	mux.HandleFunc("/api/uploads/", s.handleGetUpload)
	mux.HandleFunc("/api/notifications/pending", s.handleGetPendingNotifications)
	mux.HandleFunc("/api/notifications/", s.handleMarkNotificationDelivered)
	mux.HandleFunc("/api/providers/test", s.handleTestProvider)
	mux.HandleFunc("/ws", s.handleWebSocket)

	mux.Handle("/", spaHandler(s.uiDir))

	s.server = &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		<-ctx.Done()
		_ = s.Stop(context.Background())
	}()

	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	if s.server == nil {
		return nil
	}
	return s.server.Shutdown(ctx)
}

// AddNotification adds a notification to the store
func (s *Server) AddNotification(title, body string, data map[string]interface{}) string {
	return s.notificationStore.Add(title, body, data)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	status := map[string]interface{}{
		"workspace":           s.cfg.Agents.Defaults.Workspace,
		"model":               s.cfg.Agents.Defaults.Model,
		"restrictToWorkspace": s.cfg.Tools.RestrictToWorkspace,
	}

	if s.channelRegistry != nil {
		var enabled []string
		for _, ch := range s.channelRegistry.GetEnabled() {
			enabled = append(enabled, ch.Name())
		}
		status["channels"] = enabled

		if wa, ok := s.channelRegistry.Get("whatsapp"); ok {
			if waChannel, ok := wa.(*channels.WhatsAppChannel); ok {
				status["whatsapp"] = waChannel.Status()
			}
		}

		if tg, ok := s.channelRegistry.Get("telegram"); ok {
			if tgChannel, ok := tg.(*channels.TelegramChannel); ok {
				status["telegram"] = tgChannel.Status()
			}
		}
	}

	if s.cronService != nil {
		status["cron"] = s.cronService.Status()
	}

	writeJSON(w, status)
}

func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	list, err := listSessions(s.cfg.Agents.Defaults.Workspace)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, map[string]interface{}{"sessions": list})
}

func (s *Server) handleSessionByKey(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleSessionGet(w, r)
	case http.MethodPost:
		s.handleSessionPost(w, r)
	case http.MethodDelete:
		s.handleSessionDelete(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleSessionGet(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimPrefix(r.URL.Path, "/api/sessions/")
	if key == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	mgr := session.NewManager(s.cfg.Agents.Defaults.Workspace)
	sess := mgr.GetOrCreate(key)
	writeJSON(w, sess)
}

func (s *Server) handleSessionPost(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/sessions/")
	parts := strings.Split(path, "/")
	if len(parts) < 1 || parts[0] == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	key := parts[0]

	// Check if it's a rename request: /api/sessions/{key}/rename
	if len(parts) >= 2 && parts[1] == "rename" {
		var req struct {
			Title string `json:"title"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, err)
			return
		}

		mgr := session.NewManager(s.cfg.Agents.Defaults.Workspace)
		sess := mgr.GetOrCreate(key)

		// Update the last message as the title
		if len(sess.Messages) > 0 {
			sess.Messages[len(sess.Messages)-1].Content = req.Title
			sess.Messages[len(sess.Messages)-1].Timestamp = time.Now()
		} else {
			// If no messages, create a system message with the title
			sess.Messages = append(sess.Messages, session.Message{
				Role:      "system",
				Content:   req.Title,
				Timestamp: time.Now(),
			})
		}

		if err := mgr.Save(sess); err != nil {
			writeError(w, err)
			return
		}

		writeJSON(w, map[string]interface{}{
			"ok":    true,
			"key":   key,
			"title": req.Title,
		})
		return
	}

	w.WriteHeader(http.StatusNotFound)
}

func (s *Server) handleSessionDelete(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimPrefix(r.URL.Path, "/api/sessions/")
	if key == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	mgr := session.NewManager(s.cfg.Agents.Defaults.Workspace)
	if err := mgr.Delete(key); err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, map[string]interface{}{
		"ok":  true,
		"key": key,
	})
}

func (s *Server) handleMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var payload messagePayload

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, err)
		return
	}

	if payload.Content == "" {
		writeError(w, fmt.Errorf("content is required"))
		return
	}

	if payload.SessionKey == "" {
		payload.SessionKey = "webui:default"
	}
	if payload.Channel == "" {
		payload.Channel = "webui"
	}
	if payload.ChatID == "" {
		payload.ChatID = payload.SessionKey
	}

	if wantsStreamResponse(r, payload) {
		s.handleMessageStream(w, r, payload)
		return
	}

	resp, err := s.agentLoop.ProcessDirectWithSkills(r.Context(), payload.Content, payload.SessionKey, payload.Channel, payload.ChatID, payload.SelectedSkills)
	if err != nil {
		writeError(w, err)
		if lg := logging.Get(); lg != nil && lg.Web != nil {
			lg.Web.Printf("message error session=%s channel=%s err=%v", payload.SessionKey, payload.Channel, err)
		}
		return
	}

	if lg := logging.Get(); lg != nil && lg.Web != nil {
		lg.Web.Printf("message session=%s channel=%s content=%q", payload.SessionKey, payload.Channel, logging.Truncate(payload.Content, 300))
	}

	writeJSON(w, map[string]interface{}{
		"response":   resp,
		"sessionKey": payload.SessionKey,
	})
}

func (s *Server) handleMessageStream(w http.ResponseWriter, r *http.Request, payload messagePayload) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, fmt.Errorf("streaming is not supported by this server"))
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	writeSSE := func(v interface{}) error {
		body, err := json.Marshal(v)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", body); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	var streamWriteErr error
	resp, err := s.agentLoop.ProcessDirectEventStreamWithSkills(
		ctx,
		payload.Content,
		payload.SessionKey,
		payload.Channel,
		payload.ChatID,
		payload.SelectedSkills,
		func(event agent.StreamEvent) {
			if streamWriteErr != nil {
				return
			}

			if event.Type == "" {
				event.Type = "content_delta"
			}

			if err := writeSSE(event); err != nil {
				streamWriteErr = err
				cancel()
			}
		},
	)

	if streamWriteErr != nil {
		if lg := logging.Get(); lg != nil && lg.Web != nil {
			lg.Web.Printf("stream write aborted session=%s channel=%s err=%v", payload.SessionKey, payload.Channel, streamWriteErr)
		}
		return
	}

	if err != nil {
		_ = writeSSE(map[string]interface{}{
			"type":       "error",
			"error":      err.Error(),
			"sessionKey": payload.SessionKey,
		})
		_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
		flusher.Flush()
		if lg := logging.Get(); lg != nil && lg.Web != nil {
			lg.Web.Printf("message stream error session=%s channel=%s err=%v", payload.SessionKey, payload.Channel, err)
		}
		return
	}

	_ = writeSSE(map[string]interface{}{
		"type":       "final",
		"response":   resp,
		"sessionKey": payload.SessionKey,
		"done":       true,
	})
	_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
	flusher.Flush()

	if lg := logging.Get(); lg != nil && lg.Web != nil {
		lg.Web.Printf("message stream session=%s channel=%s content=%q", payload.SessionKey, payload.Channel, logging.Truncate(payload.Content, 300))
	}
}

func (s *Server) handleSkills(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	entries, err := workspaceSkills.Discover(filepath.Join(s.cfg.Agents.Defaults.Workspace, "skills"))
	if err != nil {
		writeError(w, err)
		return
	}

	type skillSummary struct {
		Name        string `json:"name"`
		DisplayName string `json:"displayName"`
		Description string `json:"description,omitempty"`
		Enabled     bool   `json:"enabled"`
	}

	results := make([]skillSummary, 0, len(entries))
	for _, entry := range entries {
		desc := entry.Description
		if desc == "" {
			desc = summarizeSkillBody(entry.Body, 120)
		}
		results = append(results, skillSummary{
			Name:        entry.Name,
			DisplayName: entry.DisplayName,
			Description: desc,
			Enabled:     s.skillsStateMgr.IsEnabled(entry.Name),
		})
	}

	writeJSON(w, map[string]interface{}{
		"skills": results,
	})
}

func (s *Server) handleSkillsPath(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/skills/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	skillName := parts[0]
	action := parts[1]

	switch action {
	case "enable":
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if err := s.skillsStateMgr.SetEnabled(skillName, true); err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, map[string]interface{}{"ok": true, "name": skillName, "enabled": true})

	case "disable":
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if err := s.skillsStateMgr.SetEnabled(skillName, false); err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, map[string]interface{}{"ok": true, "name": skillName, "enabled": false})

	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (s *Server) handleSkillsInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Type   string `json:"type"`
		Source string `json:"source"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, err)
		return
	}

	if req.Source == "" {
		writeError(w, fmt.Errorf("source is required"))
		return
	}

	skillsDir := filepath.Join(s.cfg.Agents.Defaults.Workspace, "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		writeError(w, fmt.Errorf("failed to create skills directory: %w", err))
		return
	}

	var result map[string]interface{}
	var err error

	switch req.Type {
	case "github":
		result, err = s.installSkillFromGitHub(skillsDir, req.Source)
	case "zip":
		result, err = s.installSkillFromZip(skillsDir, req.Source)
	case "folder":
		result, err = s.installSkillFromFolder(skillsDir, req.Source)
	default:
		writeError(w, fmt.Errorf("unsupported install type: %s", req.Type))
		return
	}

	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, result)
}

func (s *Server) installSkillFromGitHub(skillsDir string, repoURL string) (map[string]interface{}, error) {
	// Parse GitHub URL to extract repo URL, branch and subdirectory
	// Supports:
	// - https://github.com/user/repo
	// - https://github.com/user/repo/tree/branch/subdir
	// - git@github.com:user/repo.git

	repoBase, branch, subDir := parseGitHubURL(repoURL)
	if repoBase == "" {
		return nil, fmt.Errorf("invalid GitHub URL")
	}

	// Determine skill name: use subdir name if present, otherwise repo name
	var skillName string
	if subDir != "" {
		skillName = filepath.Base(subDir)
	} else {
		skillName = extractRepoName(repoBase)
	}

	targetDir := filepath.Join(skillsDir, skillName)

	// Remove existing directory if present
	if _, err := os.Stat(targetDir); err == nil {
		if err := os.RemoveAll(targetDir); err != nil {
			return nil, fmt.Errorf("failed to remove existing skill: %w", err)
		}
	}

	// Create target directory
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create skill directory: %w", err)
	}

	// Use sparse checkout if subdirectory is specified
	if subDir != "" {
		// Initialize git repo
		cmd := exec.Command("git", "init")
		cmd.Dir = targetDir
		if output, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("git init failed: %w\n%s", err, string(output))
		}

		// Add remote
		cmd = exec.Command("git", "remote", "add", "origin", repoBase)
		cmd.Dir = targetDir
		if output, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("git remote add failed: %w\n%s", err, string(output))
		}

		// Configure sparse checkout
		cmd = exec.Command("git", "config", "core.sparseCheckout", "true")
		cmd.Dir = targetDir
		if output, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("git config failed: %w\n%s", err, string(output))
		}

		// Write sparse-checkout file
		sparseFile := filepath.Join(targetDir, ".git", "info", "sparse-checkout")
		if err := os.WriteFile(sparseFile, []byte(subDir+"/\n"), 0644); err != nil {
			return nil, fmt.Errorf("failed to write sparse-checkout: %w", err)
		}

		// Pull the specific directory
		cmd = exec.Command("git", "pull", "--depth", "1", "origin", branch)
		cmd.Dir = targetDir
		if output, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("git pull failed: %w\n%s", err, string(output))
		}

		// Move contents from subdir to targetDir root
		subDirPath := filepath.Join(targetDir, subDir)
		if err := moveDirContents(subDirPath, targetDir); err != nil {
			return nil, fmt.Errorf("failed to move skill contents: %w", err)
		}
	} else {
		// Simple clone for root-level repos
		cmd := exec.Command("git", "clone", "--depth", "1", "--branch", branch, repoBase, targetDir)
		if output, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("git clone failed: %w\n%s", err, string(output))
		}
	}

	// Remove .git directory to save space
	gitDir := filepath.Join(targetDir, ".git")
	_ = os.RemoveAll(gitDir)

	return map[string]interface{}{
		"ok":       true,
		"name":     skillName,
		"type":     "github",
		"location": targetDir,
	}, nil
}

// parseGitHubURL parses a GitHub URL and returns (repoBaseURL, branch, subDir)
// Examples:
// - https://github.com/user/repo -> (https://github.com/user/repo, "main", "")
// - https://github.com/user/repo/tree/main/skills -> (https://github.com/user/repo, "main", "skills")
// - https://github.com/user/repo/tree/dev -> (https://github.com/user/repo, "dev", "")
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

func (s *Server) installSkillFromZip(skillsDir string, zipPath string) (map[string]interface{}, error) {
	// Validate zip path
	if _, err := os.Stat(zipPath); err != nil {
		return nil, fmt.Errorf("zip file not found: %w", err)
	}

	// Extract zip file name as skill name
	skillName := strings.TrimSuffix(filepath.Base(zipPath), filepath.Ext(zipPath))
	targetDir := filepath.Join(skillsDir, skillName)

	// Remove existing directory if present
	if _, err := os.Stat(targetDir); err == nil {
		if err := os.RemoveAll(targetDir); err != nil {
			return nil, fmt.Errorf("failed to remove existing skill: %w", err)
		}
	}

	// Create target directory
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create target directory: %w", err)
	}

	// Extract zip
	cmd := exec.Command("unzip", "-q", "-o", zipPath, "-d", targetDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		// Try using unzip on macOS/Linux, if not available return error
		if errors.Is(err, exec.ErrNotFound) {
			return nil, fmt.Errorf("unzip command not found, please install unzip")
		}
		return nil, fmt.Errorf("unzip failed: %w\n%s", err, string(output))
	}

	return map[string]interface{}{
		"ok":       true,
		"name":     skillName,
		"type":     "zip",
		"location": targetDir,
	}, nil
}

func (s *Server) installSkillFromFolder(skillsDir string, sourceFolder string) (map[string]interface{}, error) {
	// Validate source folder
	info, err := os.Stat(sourceFolder)
	if err != nil {
		return nil, fmt.Errorf("source folder not found: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("source is not a directory")
	}

	// Use folder name as skill name
	skillName := filepath.Base(sourceFolder)
	targetDir := filepath.Join(skillsDir, skillName)

	// Remove existing directory if present
	if _, err := os.Stat(targetDir); err == nil {
		if err := os.RemoveAll(targetDir); err != nil {
			return nil, fmt.Errorf("failed to remove existing skill: %w", err)
		}
	}

	// Copy directory (using cp -r for simplicity)
	cmd := exec.Command("cp", "-r", sourceFolder, targetDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("copy failed: %w\n%s", err, string(output))
	}

	return map[string]interface{}{
		"ok":       true,
		"name":     skillName,
		"type":     "folder",
		"location": targetDir,
	}, nil
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

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cfg, err := config.LoadConfig()
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, cfg)
	case http.MethodPut:
		var cfg config.Config
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			writeError(w, err)
			return
		}
		if err := config.SaveConfig(&cfg); err != nil {
			writeError(w, err)
			return
		}
		updated, err := config.LoadConfig()
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, updated)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleGatewayRestart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	rootDir, script, err := findRestartScript()
	if err != nil {
		writeError(w, err)
		return
	}

	cmd := exec.Command("bash", script)
	cmd.Dir = rootDir
	if err := cmd.Start(); err != nil {
		writeError(w, fmt.Errorf("failed to restart gateway: %w", err))
		return
	}

	if lg := logging.Get(); lg != nil && lg.Web != nil {
		lg.Web.Printf("gateway restart triggered script=%s pid=%d", script, cmd.Process.Pid)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"message": "gateway restart triggered",
	})
}

// cronRequest 创建定时任务的请求格式（与前端对齐）
type cronRequest struct {
	Title   string `json:"title"`
	Prompt  string `json:"prompt"`
	Cron    string `json:"cron,omitempty"`    // cron 表达式
	Every   string `json:"every,omitempty"`   // 毫秒间隔
	At      string `json:"at,omitempty"`      // ISO8601 时间
	WorkDir string `json:"workDir,omitempty"`
}

// cronJobResponse 定时任务响应格式（与前端对齐）
type cronJobResponse struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	Prompt        string `json:"prompt"`
	Schedule      string `json:"schedule"`
	ScheduleType  string `json:"scheduleType"`
	WorkDir       string `json:"workDir,omitempty"`
	Enabled       bool   `json:"enabled"`
	CreatedAt     string `json:"createdAt"`
	LastRun       string `json:"lastRun,omitempty"`
	NextRun       string `json:"nextRun,omitempty"`
}

func (s *Server) handleCron(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleCronList(w, r)
	case http.MethodPost:
		s.handleCronCreate(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleCronList(w http.ResponseWriter, r *http.Request) {
	if s.cronService == nil {
		writeJSON(w, map[string]interface{}{"jobs": []cronJobResponse{}})
		return
	}

	jobs := s.cronService.ListJobs()
	responses := make([]cronJobResponse, 0, len(jobs))

	for _, job := range jobs {
		responses = append(responses, s.toCronJobResponse(job))
	}

	writeJSON(w, map[string]interface{}{"jobs": responses})
}

func (s *Server) handleCronCreate(w http.ResponseWriter, r *http.Request) {
	if s.cronService == nil {
		writeError(w, fmt.Errorf("cron service not available"))
		return
	}

	var req cronRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, err)
		return
	}

	if req.Title == "" || req.Prompt == "" {
		writeError(w, fmt.Errorf("title and prompt are required"))
		return
	}

	// 根据请求字段确定调度类型
	var schedule cron.Schedule
	switch {
	case req.Cron != "":
		schedule = cron.Schedule{
			Type: cron.ScheduleTypeCron,
			Expr: req.Cron,
		}
	case req.Every != "":
		// 尝试解析毫秒数
		var everyMs int64
		if _, err := fmt.Sscanf(req.Every, "%d", &everyMs); err != nil {
			writeError(w, fmt.Errorf("invalid every format: %v", err))
			return
		}
		schedule = cron.Schedule{
			Type:    cron.ScheduleTypeEvery,
			EveryMs: everyMs,
		}
	case req.At != "":
		// 尝试解析 ISO8601 时间
		at, err := time.Parse(time.RFC3339, req.At)
		if err != nil {
			// 尝试其他格式
			at, err = time.Parse("2006-01-02T15:04:05", req.At)
			if err != nil {
				writeError(w, fmt.Errorf("invalid at format: %v", err))
				return
			}
		}
		schedule = cron.Schedule{
			Type: cron.ScheduleTypeOnce,
			AtMs: at.UnixMilli(),
		}
	default:
		writeError(w, fmt.Errorf("schedule is required (cron, every, or at)"))
		return
	}

	payload := cron.Payload{
		Message: req.Prompt,
		Channel: "desktop",
	}

	job, err := s.cronService.AddJob(req.Title, schedule, payload)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, s.toCronJobResponse(job))
}

func (s *Server) handleCronByID(w http.ResponseWriter, r *http.Request) {
	if s.cronService == nil {
		writeError(w, fmt.Errorf("cron service not available"))
		return
	}

	// 路径格式: /api/cron/{id}/enable 或 /api/cron/{id}/disable 或 /api/cron/{id}
	path := strings.TrimPrefix(r.URL.Path, "/api/cron/")
	parts := strings.Split(path, "/")
	if len(parts) < 1 || parts[0] == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	jobID := parts[0]

	// 检查是否是操作请求（enable/disable）
	if len(parts) >= 2 {
		action := parts[1]
		switch action {
		case "enable":
			s.handleCronEnable(w, r, jobID, true)
		case "disable":
			s.handleCronEnable(w, r, jobID, false)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
		return
	}

	// 单一资源操作
	switch r.Method {
	case http.MethodDelete:
		s.handleCronDelete(w, r, jobID)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleCronEnable(w http.ResponseWriter, r *http.Request, jobID string, enabled bool) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	job, ok := s.cronService.EnableJob(jobID, enabled)
	if !ok {
		writeError(w, fmt.Errorf("job not found"))
		return
	}

	writeJSON(w, s.toCronJobResponse(job))
}

func (s *Server) handleCronDelete(w http.ResponseWriter, r *http.Request, jobID string) {
	if !s.cronService.RemoveJob(jobID) {
		writeError(w, fmt.Errorf("job not found"))
		return
	}

	writeJSON(w, map[string]bool{"ok": true})
}

// toCronJobResponse 将内部 Job 转换为前端期望的格式
func (s *Server) toCronJobResponse(job *cron.Job) cronJobResponse {
	resp := cronJobResponse{
		ID:        job.ID,
		Title:     job.Name,
		Prompt:    job.Payload.Message,
		Enabled:   job.Enabled,
		CreatedAt: time.UnixMilli(job.Created).Format(time.RFC3339),
	}

	switch job.Schedule.Type {
	case cron.ScheduleTypeCron:
		resp.ScheduleType = "cron"
		resp.Schedule = job.Schedule.Expr
	case cron.ScheduleTypeEvery:
		resp.ScheduleType = "every"
		resp.Schedule = fmt.Sprintf("%d", job.Schedule.EveryMs)
	case cron.ScheduleTypeOnce:
		resp.ScheduleType = "once"
		resp.Schedule = time.UnixMilli(job.Schedule.AtMs).Format(time.RFC3339)
	}

	// 计算下次执行时间
	if next, ok := job.GetNextRun(); ok {
		nextStr := next.Format(time.RFC3339)
		resp.NextRun = nextStr
	}

	return resp
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
		summary := sessionSummary{
			Key:          sess.Key,
			MessageCount: len(sess.Messages),
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

func (s *Server) handleTestProvider(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req ProviderTestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.APIKey == "" {
		http.Error(w, "API key is required", http.StatusBadRequest)
		return
	}

	// Test the provider connection
	client := &http.Client{Timeout: 10 * time.Second}

	var testURL string
	headers := make(map[string]string)

	switch req.APIFormat {
	case "anthropic":
		baseURL := req.BaseURL
		if baseURL == "" {
			baseURL = "https://api.anthropic.com"
		}
		testURL = baseURL + "/v1/models"
		headers["x-api-key"] = req.APIKey
		headers["anthropic-version"] = "2023-06-01"
	default: // openai
		baseURL := req.BaseURL
		if baseURL == "" {
			// Try to determine from provider name
			switch req.Name {
			case "DeepSeek":
				baseURL = "https://api.deepseek.com/v1"
			case "Moonshot":
				baseURL = "https://api.moonshot.cn/v1"
			case "Groq":
				baseURL = "https://api.groq.com/openai/v1"
			default:
				baseURL = "https://api.openai.com/v1"
			}
		}
		testURL = baseURL + "/models"
		headers["Authorization"] = "Bearer " + req.APIKey
	}

	httpReq, err := http.NewRequest("GET", testURL, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	for k, v := range headers {
		httpReq.Header.Set(k, v)
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		http.Error(w, "API returned status "+resp.Status, resp.StatusCode)
		return
	}

	writeJSON(w, map[string]bool{"ok": true})
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
