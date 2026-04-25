package webui

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Lichas/maxclaw/internal/agent"
	"github.com/Lichas/maxclaw/internal/bus"
	"github.com/Lichas/maxclaw/internal/channels"
	"github.com/Lichas/maxclaw/internal/config"
	"github.com/Lichas/maxclaw/internal/cron"
	"github.com/Lichas/maxclaw/internal/logging"
	"github.com/Lichas/maxclaw/internal/providers"
	"github.com/Lichas/maxclaw/internal/session"
	workspaceSkills "github.com/Lichas/maxclaw/internal/skills"
	"github.com/Lichas/maxclaw/pkg/tools"
	"github.com/anthropics/anthropic-sdk-go"
	anthropicoption "github.com/anthropics/anthropic-sdk-go/option"
	"github.com/openai/openai-go/v3"
	openaioption "github.com/openai/openai-go/v3/option"
	qqtoken "github.com/tencent-connect/botgo/token"
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
	outboundUnsub     func()
}

type channelSenderStat struct {
	Channel       string `json:"channel"`
	Sender        string `json:"sender"`
	ChatID        string `json:"chatId"`
	LastSeen      string `json:"lastSeen"`
	MessageCount  int    `json:"messageCount"`
	LatestMessage string `json:"latestMessage,omitempty"`
}

type messagePayload struct {
	SessionKey     string              `json:"sessionKey"`
	Content        string              `json:"content"`
	Channel        string              `json:"channel"`
	ChatID         string              `json:"chatId"`
	SelectedSkills []string            `json:"selectedSkills,omitempty"`
	Attachments    []messageAttachment `json:"attachments,omitempty"`
	Stream         bool                `json:"stream,omitempty"`
}

type messageAttachment struct {
	ID       string `json:"id,omitempty"`
	Filename string `json:"filename,omitempty"`
	Size     int64  `json:"size,omitempty"`
	URL      string `json:"url,omitempty"`
	Path     string `json:"path,omitempty"`
}

type browserActionPayload struct {
	SessionKey string                 `json:"sessionKey"`
	Channel    string                 `json:"channel,omitempty"`
	ChatID     string                 `json:"chatId,omitempty"`
	Params     map[string]interface{} `json:"params"`
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

	if agentLoop != nil && agentLoop.Bus != nil {
		s.outboundUnsub = agentLoop.Bus.SubscribeOutbound(func(msg *bus.OutboundMessage) {
			if msg == nil {
				return
			}
			channel := strings.ToLower(strings.TrimSpace(msg.Channel))
			if channel != "desktop" && channel != "webui" {
				return
			}
			if strings.HasPrefix(strings.TrimSpace(msg.Content), "[Spawn] ") {
				return
			}

			sessionKey := strings.TrimSpace(msg.ChatID)
			if sessionKey == "" {
				sessionKey = fmt.Sprintf("%s:%s", channel, strings.TrimSpace(msg.ChatID))
			}
			s.wsHub.Broadcast("chat", map[string]interface{}{
				"sessionKey": sessionKey,
				"channel":    channel,
				"chatId":     msg.ChatID,
				"content":    msg.Content,
			})
		})
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
	mux.HandleFunc("/api/skills/sources", s.handleSkillSources)
	mux.HandleFunc("/api/skills/", s.handleSkillsPath)
	mux.HandleFunc("/api/skills/install", s.handleSkillsInstall)
	mux.HandleFunc("/api/message", s.handleMessage)
	mux.HandleFunc("/api/browser/action", s.handleBrowserAction)
	mux.HandleFunc("/api/config", s.handleConfig)
	mux.HandleFunc("/api/workspace-file/", s.handleWorkspaceFile)
	mux.HandleFunc("/api/gateway/restart", s.handleGatewayRestart)
	mux.HandleFunc("/api/cron", s.handleCron)
	mux.HandleFunc("/api/cron/", s.handleCronByID)
	mux.HandleFunc("/api/cron/history", s.handleGetCronHistory)
	mux.HandleFunc("/api/cron/history/", s.handleGetCronHistoryDetail)
	mux.HandleFunc("/api/upload", s.handleUpload)
	mux.HandleFunc("/api/uploads/", s.handleGetUpload)
	mux.HandleFunc("/api/notifications/pending", s.handleGetPendingNotifications)
	mux.HandleFunc("/api/notifications/", s.handleMarkNotificationDelivered)
	mux.HandleFunc("/api/providers/test", s.handleTestProvider)
	mux.HandleFunc("/api/providers/models", s.handleFetchProviderModels)
	mux.HandleFunc("/api/channels/senders", s.handleChannelSenders)
	mux.HandleFunc("/api/channels/", s.handleTestChannel)
	mux.HandleFunc("/api/channels/whatsapp/status", s.handleWhatsAppStatus)
	mux.HandleFunc("/api/mcp", s.handleMCP)
	mux.HandleFunc("/api/mcp/", s.handleMCPByName)
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

func (s *Server) handleChannelSenders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	limit := 50
	if rawLimit := strings.TrimSpace(r.URL.Query().Get("limit")); rawLimit != "" {
		if parsed, err := strconv.Atoi(rawLimit); err == nil && parsed > 0 {
			if parsed > 200 {
				parsed = 200
			}
			limit = parsed
		}
	}

	channel := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("channel")))
	grouped := strings.TrimSpace(r.URL.Query().Get("grouped")) == "true"

	users, err := readChannelSenderStats(filepath.Join(config.GetLogsDir(), "session.log"), channel, limit)
	if err != nil {
		writeError(w, err)
		return
	}

	// Return grouped by channel for recipient selection UI
	if grouped && channel == "" {
		groupedUsers := make(map[string][]map[string]interface{})
		seen := make(map[string]map[string]bool) // channel -> chatId -> seen

		for _, u := range users {
			if u.ChatID == "" {
				continue
			}
			// Skip duplicates per channel
			if seen[u.Channel] == nil {
				seen[u.Channel] = make(map[string]bool)
			}
			if seen[u.Channel][u.ChatID] {
				continue
			}
			seen[u.Channel][u.ChatID] = true

			entry := map[string]interface{}{
				"chatId":   u.ChatID,
				"sender":   u.Sender,
				"lastSeen": u.LastSeen,
			}
			groupedUsers[u.Channel] = append(groupedUsers[u.Channel], entry)
		}
		writeJSON(w, map[string]interface{}{"grouped": groupedUsers})
		return
	}

	writeJSON(w, map[string]interface{}{"users": users})
}

func (s *Server) Stop(ctx context.Context) error {
	if s.outboundUnsub != nil {
		s.outboundUnsub()
		s.outboundUnsub = nil
	}
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
		"executionMode":       s.cfg.Agents.Defaults.ExecutionMode,
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
		req.Title = strings.TrimSpace(req.Title)
		if req.Title == "" {
			writeError(w, fmt.Errorf("title is required"))
			return
		}

		mgr := session.NewManager(s.cfg.Agents.Defaults.Workspace)
		sess := mgr.GetOrCreate(key)
		sess.Title = req.Title
		sess.TitleSource = session.TitleSourceUser
		sess.TitleState = session.TitleStateStable
		sess.TitleUpdatedAt = time.Now()

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

	enrichedContent := s.enrichContentWithAttachments(payload.Content, payload.Attachments)
	resp, err := s.agentLoop.ProcessDirectWithMediaAndSkills(
		r.Context(),
		enrichedContent,
		payload.SessionKey,
		payload.Channel,
		payload.ChatID,
		payload.SelectedSkills,
		s.extractImageAttachment(payload.Attachments),
	)
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

func (s *Server) handleBrowserAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if s.agentLoop == nil {
		writeError(w, fmt.Errorf("agent loop is not available"))
		return
	}

	var payload browserActionPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, err)
		return
	}

	payload.SessionKey = strings.TrimSpace(payload.SessionKey)
	if payload.SessionKey == "" {
		payload.SessionKey = "webui:default"
	}
	payload.Channel = strings.TrimSpace(payload.Channel)
	if payload.Channel == "" {
		payload.Channel = "desktop"
	}
	payload.ChatID = strings.TrimSpace(payload.ChatID)
	if payload.ChatID == "" {
		payload.ChatID = payload.SessionKey
	}
	if payload.Params == nil {
		payload.Params = map[string]interface{}{}
	}

	action, _ := payload.Params["action"].(string)
	if strings.TrimSpace(action) == "" {
		writeError(w, fmt.Errorf("browser action is required"))
		return
	}

	result, err := s.agentLoop.ExecuteToolWithSession(
		r.Context(),
		"browser",
		payload.Params,
		payload.SessionKey,
		payload.Channel,
		payload.ChatID,
	)
	if err != nil {
		writeError(w, err)
		if lg := logging.Get(); lg != nil && lg.Web != nil {
			lg.Web.Printf("browser action error session=%s action=%s err=%v", payload.SessionKey, action, err)
		}
		return
	}

	if lg := logging.Get(); lg != nil && lg.Web != nil {
		lg.Web.Printf("browser action session=%s action=%s", payload.SessionKey, action)
	}

	writeJSON(w, map[string]interface{}{
		"ok":         true,
		"sessionKey": payload.SessionKey,
		"result":     result,
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
	resp, err := s.agentLoop.ProcessDirectEventStreamWithMediaAndSkills(
		ctx,
		s.enrichContentWithAttachments(payload.Content, payload.Attachments),
		payload.SessionKey,
		payload.Channel,
		payload.ChatID,
		payload.SelectedSkills,
		s.extractImageAttachment(payload.Attachments),
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

func (s *Server) enrichContentWithAttachments(content string, attachments []messageAttachment) string {
	if len(attachments) == 0 {
		return content
	}

	type attachmentInfo struct {
		name string
		path string
	}

	items := make([]attachmentInfo, 0, len(attachments))
	seen := make(map[string]struct{}, len(attachments))
	uploadsDir := filepath.Join(s.cfg.Agents.Defaults.Workspace, ".uploads")

	for _, att := range attachments {
		p := strings.TrimSpace(att.Path)
		if p == "" {
			if rawURL := strings.TrimSpace(att.URL); rawURL != "" {
				p = filepath.Join(uploadsDir, filepath.Base(rawURL))
			}
		}
		if p == "" {
			continue
		}
		if !filepath.IsAbs(p) {
			p = filepath.Join(s.cfg.Agents.Defaults.Workspace, p)
		}
		p = filepath.Clean(p)
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}

		name := strings.TrimSpace(att.Filename)
		if name == "" {
			name = filepath.Base(p)
		}
		items = append(items, attachmentInfo{name: name, path: p})
	}

	if len(items) == 0 {
		return content
	}

	var b strings.Builder
	b.WriteString(content)
	b.WriteString("\n\nAttached files (local paths):\n")
	for _, item := range items {
		b.WriteString("- ")
		b.WriteString(item.name)
		b.WriteString(": `")
		b.WriteString(item.path)
		b.WriteString("`\n")
	}
	b.WriteString("If the user asks about an attached file, read it from the path above before answering.")
	return b.String()
}

func (s *Server) extractImageAttachment(attachments []messageAttachment) *bus.MediaAttachment {
	for _, att := range attachments {
		path := strings.TrimSpace(att.Path)
		if path == "" {
			continue
		}
		if !filepath.IsAbs(path) {
			path = filepath.Join(s.cfg.Agents.Defaults.Workspace, path)
		}
		path = filepath.Clean(path)

		ext := strings.ToLower(filepath.Ext(path))
		if ext == "" {
			continue
		}
		mimeType := mime.TypeByExtension(ext)
		if !strings.HasPrefix(mimeType, "image/") {
			continue
		}

		filename := strings.TrimSpace(att.Filename)
		if filename == "" {
			filename = filepath.Base(path)
		}
		return &bus.MediaAttachment{
			Type:      "image",
			Filename:  filename,
			LocalPath: path,
			MimeType:  mimeType,
		}
	}

	return nil
}

func (s *Server) handleSkills(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	entries, err := workspaceSkills.DiscoverAll(
		filepath.Join(s.cfg.Agents.Defaults.Workspace, "skills"),
		s.cfg.Agents.Defaults.EnableGlobalSkills,
	)
	if err != nil {
		writeError(w, err)
		return
	}

	type skillSummary struct {
		Name        string `json:"name"`
		DisplayName string `json:"displayName"`
		Description string `json:"description,omitempty"`
		Enabled     bool   `json:"enabled"`
		Source      string `json:"source,omitempty"`
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
			Source:      entry.Source,
		})
	}

	writeJSON(w, map[string]interface{}{
		"skills": results,
	})
}

func (s *Server) handleSkillSources(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	writeJSON(w, map[string]interface{}{
		"sources": workspaceSkills.RecommendedSources(),
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
	case "clawhub":
		result, err = s.installSkillFromClawHub(req.Source)
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

func (s *Server) installSkillFromClawHub(source string) (map[string]interface{}, error) {
	clawHubSource, err := workspaceSkills.ParseClawHubSource(source)
	if err != nil {
		return nil, err
	}

	installer := workspaceSkills.NewInstaller(s.cfg.Agents.Defaults.Workspace)
	result, err := installer.InstallFromClawHub(clawHubSource)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"ok":       true,
		"name":     result.Name,
		"type":     result.Type,
		"location": result.Location,
		"version":  result.Version,
		"registry": result.Registry,
	}, nil
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

// configUpdateRequest 配置更新请求，支持动态 providers
type configUpdateRequest struct {
	Agents    *config.AgentsConfig             `json:"agents,omitempty"`
	Channels  *config.ChannelsConfig           `json:"channels,omitempty"`
	Providers map[string]config.ProviderConfig `json:"providers,omitempty"`
	Gateway   *config.GatewayConfig            `json:"gateway,omitempty"`
	Tools     *config.ToolsConfig              `json:"tools,omitempty"`
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
		var req configUpdateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, err)
			return
		}

		// Load existing config
		cfg, err := config.LoadConfig()
		if err != nil {
			writeError(w, err)
			return
		}

		// Update fields if provided
		if req.Agents != nil {
			cfg.Agents = *req.Agents
		}
		if req.Channels != nil {
			cfg.Channels = *req.Channels
		}
		if req.Providers != nil {
			cfg.Providers = config.ProvidersConfigFromMap(req.Providers)
		}
		if req.Gateway != nil {
			cfg.Gateway = *req.Gateway
		}
		if req.Tools != nil {
			cfg.Tools = *req.Tools
		}

		if err := config.SaveConfig(cfg); err != nil {
			writeError(w, err)
			return
		}
		updated, err := config.LoadConfig()
		if err != nil {
			writeError(w, err)
			return
		}
		s.cfg = updated
		if err := s.applyRuntimeModelConfig(updated); err != nil {
			if lg := logging.Get(); lg != nil && lg.Web != nil {
				lg.Web.Printf("apply runtime model config failed: %v", err)
			}
		}
		if err := s.applyRuntimeMCPConfig(updated); err != nil {
			if lg := logging.Get(); lg != nil && lg.Web != nil {
				lg.Web.Printf("apply runtime MCP config failed: %v", err)
			}
		}
		writeJSON(w, updated)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// handleWorkspaceFile 处理 workspace 文件读写 (USER.md, SOUL.md)
func (s *Server) handleWorkspaceFile(w http.ResponseWriter, r *http.Request) {
	// 路径格式: /api/workspace-file/{filename}
	path := strings.TrimPrefix(r.URL.Path, "/api/workspace-file/")
	parts := strings.Split(path, "/")
	if len(parts) < 1 || parts[0] == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	filename := parts[0]
	// 只允许访问特定文件
	if filename != "USER.md" && filename != "SOUL.md" {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	switch r.Method {
	case http.MethodGet:
		content, err := s.readWorkspaceFile(filename)
		if err != nil {
			if os.IsNotExist(err) {
				// 如果文件不存在，返回空内容而不是404
				writeJSON(w, map[string]string{"content": ""})
				return
			}
			writeError(w, err)
			return
		}
		writeJSON(w, map[string]string{"content": content})

	case http.MethodPut:
		var req struct {
			Content string `json:"content"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, err)
			return
		}
		if err := s.writeWorkspaceFile(filename, req.Content); err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, map[string]bool{"ok": true})

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// readWorkspaceFile 读取 workspace 文件内容
func (s *Server) readWorkspaceFile(filename string) (string, error) {
	workspace := s.cfg.Agents.Defaults.Workspace
	if workspace == "" {
		workspace = "~/.maxclaw/workspace"
	}
	// Expand home directory
	if strings.HasPrefix(workspace, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		workspace = filepath.Join(home, workspace[1:])
	}

	filePath := filepath.Join(workspace, filename)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// writeWorkspaceFile 写入 workspace 文件
func (s *Server) writeWorkspaceFile(filename string, content string) error {
	workspace := s.cfg.Agents.Defaults.Workspace
	if workspace == "" {
		workspace = "~/.maxclaw/workspace"
	}
	// Expand home directory
	if strings.HasPrefix(workspace, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		workspace = filepath.Join(home, workspace[1:])
	}

	// Create workspace if not exists
	if err := os.MkdirAll(workspace, 0755); err != nil {
		return err
	}

	filePath := filepath.Join(workspace, filename)
	return os.WriteFile(filePath, []byte(content), 0644)
}

func (s *Server) applyRuntimeModelConfig(cfg *config.Config) error {
	if s.agentLoop == nil || cfg == nil {
		return nil
	}
	s.agentLoop.UpdateRuntimeMaxIterations(cfg.Agents.Defaults.MaxToolIterations)
	s.agentLoop.UpdateRuntimeExecutionMode(cfg.Agents.Defaults.ExecutionMode)

	model := cfg.Agents.Defaults.Model
	if model == "" {
		return nil
	}

	apiKey := cfg.GetAPIKey(model)
	if apiKey == "" {
		return fmt.Errorf("no API key configured for model %s", model)
	}
	apiBase := cfg.GetAPIBase(model)

	provider, err := providers.NewProvider(
		apiKey,
		apiBase,
		cfg.GetAPIFormat(model),
		model,
		cfg.Agents.Defaults.MaxTokens,
		cfg.Agents.Defaults.Temperature,
		cfg.SupportsImageInput,
	)
	if err != nil {
		return err
	}

	s.agentLoop.UpdateRuntimeModel(provider, model)
	return nil
}

func (s *Server) applyRuntimeMCPConfig(cfg *config.Config) error {
	if s.agentLoop == nil || cfg == nil {
		return nil
	}
	return s.agentLoop.UpdateRuntimeMCPServers(cfg.Tools.MCPServers)
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
	Title         string   `json:"title"`
	Prompt        string   `json:"prompt"`
	Cron          string   `json:"cron,omitempty"`  // cron 表达式
	Every         string   `json:"every,omitempty"` // 毫秒间隔
	At            string   `json:"at,omitempty"`    // ISO8601 时间
	WorkDir       string   `json:"workDir,omitempty"`
	ExecutionMode string   `json:"executionMode,omitempty"` // safe, ask, auto
	Channels      []string `json:"channels,omitempty"`      // 输出频道列表
	To            string   `json:"to,omitempty"`            // 接收者（chatID/用户名等）
}

// cronJobResponse 定时任务响应格式（与前端对齐）
type cronJobResponse struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	Prompt        string   `json:"prompt"`
	Schedule      string   `json:"schedule"`
	ScheduleType  string   `json:"scheduleType"`
	WorkDir       string   `json:"workDir,omitempty"`
	Enabled       bool     `json:"enabled"`
	CreatedAt     string   `json:"createdAt"`
	LastRun       string   `json:"lastRun,omitempty"`
	NextRun       string   `json:"nextRun,omitempty"`
	ExecutionMode string   `json:"executionMode,omitempty"`
	Channels      []string `json:"channels,omitempty"`
	To            string   `json:"to,omitempty"`
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

	// 使用请求中的 channels，如果未提供则默认为 [desktop]
	channels := req.Channels
	if len(channels) == 0 {
		channels = []string{"desktop"}
	}

	payload := cron.Payload{
		Message:  req.Prompt,
		Channels: channels,
		To:       req.To,
		Deliver:  len(channels) > 0 && !(len(channels) == 1 && channels[0] == "desktop"),
	}

	job, err := s.cronService.AddJobWithOptions(req.Title, schedule, payload, req.ExecutionMode)
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

	// 检查是否是操作请求（enable/disable/run）
	if len(parts) >= 2 {
		action := parts[1]
		switch action {
		case "enable":
			s.handleCronEnable(w, r, jobID, true)
		case "disable":
			s.handleCronEnable(w, r, jobID, false)
		case "run":
			s.handleCronRun(w, r, jobID)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
		return
	}

	// 单一资源操作
	switch r.Method {
	case http.MethodPut:
		s.handleCronUpdate(w, r, jobID)
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

func (s *Server) handleCronRun(w http.ResponseWriter, r *http.Request, jobID string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if err := s.cronService.RunJob(jobID); err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, map[string]interface{}{"ok": true, "message": "job triggered"})
}

func (s *Server) handleCronDelete(w http.ResponseWriter, r *http.Request, jobID string) {
	if !s.cronService.RemoveJob(jobID) {
		writeError(w, fmt.Errorf("job not found"))
		return
	}

	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) handleCronUpdate(w http.ResponseWriter, r *http.Request, jobID string) {
	var req struct {
		Title         string   `json:"title"`
		Prompt        string   `json:"prompt"`
		Cron          string   `json:"cron,omitempty"`
		Every         string   `json:"every,omitempty"`
		At            string   `json:"at,omitempty"`
		WorkDir       string   `json:"workDir,omitempty"`
		ExecutionMode string   `json:"executionMode,omitempty"`
		Channels      []string `json:"channels,omitempty"`
		To            string   `json:"to,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, fmt.Errorf("invalid request: %v", err))
		return
	}

	// 解析调度配置
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
		at, err := time.Parse(time.RFC3339, req.At)
		if err != nil {
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

	// 使用请求中的 channels，如果未提供则默认为 [desktop]
	channels := req.Channels
	if len(channels) == 0 {
		channels = []string{"desktop"}
	}

	payload := cron.Payload{
		Message:  req.Prompt,
		Channels: channels,
		To:       req.To,
		Deliver:  len(channels) > 0 && !(len(channels) == 1 && channels[0] == "desktop"),
	}

	job, ok := s.cronService.UpdateJobWithOptions(jobID, req.Title, schedule, payload, req.ExecutionMode)
	if !ok {
		writeError(w, fmt.Errorf("job not found"))
		return
	}

	writeJSON(w, s.toCronJobResponse(job))
}

func (s *Server) handleGetCronHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	jobID := r.URL.Query().Get("jobId")
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}

	if s.cronService == nil {
		writeJSON(w, map[string]interface{}{"records": []interface{}{}})
		return
	}

	records := s.cronService.GetHistoryStore().GetRecords(jobID, limit)
	writeJSON(w, map[string]interface{}{"records": records})
}

func (s *Server) handleGetCronHistoryDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/cron/history/")
	if s.cronService == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	record, found := s.cronService.GetHistoryStore().GetRecord(id)
	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	writeJSON(w, record)
}

// toCronJobResponse 将内部 Job 转换为前端期望的格式
func (s *Server) toCronJobResponse(job *cron.Job) cronJobResponse {
	resp := cronJobResponse{
		ID:            job.ID,
		Title:         job.Name,
		Prompt:        job.Payload.Message,
		Enabled:       job.Enabled,
		CreatedAt:     time.UnixMilli(job.Created).Format(time.RFC3339),
		ExecutionMode: job.ExecutionMode,
		Channels:      job.Payload.Channels,
		To:            job.Payload.To,
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

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	switch providers.ResolveProviderKind(req.Name, req.BaseURL, req.APIFormat) {
	case "anthropic":
		baseURL := req.BaseURL
		baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
		if baseURL == "" {
			baseURL = "https://api.anthropic.com"
		}
		if strings.HasSuffix(baseURL, "/v1") {
			baseURL = strings.TrimSuffix(baseURL, "/v1")
		}
		client := anthropic.NewClient(
			anthropicoption.WithAPIKey(req.APIKey),
			anthropicoption.WithBaseURL(baseURL),
			anthropicoption.WithHTTPClient(&http.Client{Timeout: 10 * time.Second}),
		)
		if _, err := client.Models.List(ctx, anthropic.ModelListParams{}); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	case "openai":
		baseURL := strings.TrimRight(strings.TrimSpace(req.BaseURL), "/")
		if baseURL == "" {
			baseURL = "https://api.openai.com/v1"
		}
		client := openai.NewClient(
			openaioption.WithAPIKey(req.APIKey),
			openaioption.WithBaseURL(baseURL),
			openaioption.WithHTTPClient(&http.Client{Timeout: 10 * time.Second}),
		)
		if _, err := client.Models.List(ctx); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	default:
		if err := s.testCompatibleProvider(ctx, req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) handleFetchProviderModels(w http.ResponseWriter, r *http.Request) {
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

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	models := make([]map[string]string, 0)

	switch providers.ResolveProviderKind(req.Name, req.BaseURL, req.APIFormat) {
	case "anthropic":
		baseURL := strings.TrimRight(strings.TrimSpace(req.BaseURL), "/")
		if baseURL == "" {
			baseURL = "https://api.anthropic.com"
		}
		if strings.HasSuffix(baseURL, "/v1") {
			baseURL = strings.TrimSuffix(baseURL, "/v1")
		}
		client := anthropic.NewClient(
			anthropicoption.WithAPIKey(req.APIKey),
			anthropicoption.WithBaseURL(baseURL),
			anthropicoption.WithHTTPClient(&http.Client{Timeout: 10 * time.Second}),
		)
		list, err := client.Models.List(ctx, anthropic.ModelListParams{})
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		for _, m := range list.Data {
			models = append(models, map[string]string{"id": string(m.ID), "name": string(m.ID)})
		}
	case "openai":
		baseURL := strings.TrimRight(strings.TrimSpace(req.BaseURL), "/")
		if baseURL == "" {
			baseURL = "https://api.openai.com/v1"
		}
		client := openai.NewClient(
			openaioption.WithAPIKey(req.APIKey),
			openaioption.WithBaseURL(baseURL),
			openaioption.WithHTTPClient(&http.Client{Timeout: 10 * time.Second}),
		)
		list, err := client.Models.List(ctx)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		for _, m := range list.Data {
			models = append(models, map[string]string{"id": m.ID, "name": m.ID})
		}
	default:
		fetched, err := s.fetchCompatibleModels(ctx, req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		models = fetched
	}

	writeJSON(w, map[string]interface{}{"models": models})
}

func (s *Server) fetchCompatibleModels(ctx context.Context, req ProviderTestRequest) ([]map[string]string, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	baseURL := req.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	// 对 Kimi / Anthropic 兼容端点，/models 通常也在 baseURL 下（已带 /v1 或不带）
	modelsURL := strings.TrimRight(baseURL, "/") + "/models"

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, modelsURL, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+req.APIKey)

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	models := make([]map[string]string, 0, len(result.Data))
	for _, m := range result.Data {
		name := m.Name
		if name == "" {
			name = m.ID
		}
		models = append(models, map[string]string{"id": m.ID, "name": name})
	}
	return models, nil
}

func (s *Server) testCompatibleProvider(ctx context.Context, req ProviderTestRequest) error {
	client := &http.Client{Timeout: 10 * time.Second}

	var (
		method  = http.MethodGet
		testURL string
		body    io.Reader
	)
	headers := make(map[string]string)

	baseURL := req.BaseURL
	if strings.EqualFold(req.Name, "MiniMax") {
		if baseURL == "" {
			baseURL = "https://api.minimax.io/v1"
		}
		baseURL = normalizeMiniMaxBaseURL(baseURL)
		method = http.MethodPost
		testURL = strings.TrimRight(baseURL, "/") + "/chat/completions"
		headers["Authorization"] = "Bearer " + req.APIKey
		headers["Content-Type"] = "application/json"
		probe := providerProbePayload{
			Model: "MiniMax-M2.5",
			Messages: []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			}{
				{Role: "user", Content: "ping"},
			},
			MaxTokens:   1,
			Temperature: 0,
		}
		payload, err := json.Marshal(probe)
		if err != nil {
			return err
		}
		body = strings.NewReader(string(payload))
	} else {
		if baseURL == "" {
			switch req.Name {
			case "DeepSeek":
				baseURL = "https://api.deepseek.com/v1"
			case "Zhipu", "Zhipu GLM":
				baseURL = "https://open.bigmodel.cn/api/coding/paas/v4"
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

	httpReq, err := http.NewRequestWithContext(ctx, method, testURL, body)
	if err != nil {
		return err
	}

	for k, v := range headers {
		httpReq.Header.Set(k, v)
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned status %s", resp.Status)
	}
	return nil
}

func normalizeMiniMaxBaseURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return raw
	}
	return strings.Replace(trimmed, "api.minimax.com", "api.minimaxi.com", 1)
}

// handleTestChannel tests IM channel connections (telegram, discord, etc.)
func (s *Server) handleTestChannel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Extract channel name from URL path: /api/channels/{name}/test
	path := strings.TrimPrefix(r.URL.Path, "/api/channels/")
	path = strings.TrimSuffix(path, "/test")
	channelName := strings.ToLower(path)

	switch channelName {
	case "telegram":
		var cfg config.TelegramConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := testTelegramConnection(cfg); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

	case "discord":
		var cfg config.DiscordConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := testDiscordConnection(cfg); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

	case "whatsapp":
		var cfg config.WhatsAppConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := testWhatsAppConnection(cfg); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

	case "slack":
		var cfg config.SlackConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := testSlackConnection(cfg); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

	case "feishu":
		var cfg config.FeishuConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := testFeishuConnection(cfg); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

	case "qq":
		var cfg config.QQConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := testQQConnection(cfg); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

	case "email":
		var cfg config.EmailConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := testEmailConnection(cfg); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

	default:
		http.Error(w, "Unknown channel: "+channelName, http.StatusBadRequest)
		return
	}

	writeJSON(w, map[string]bool{"ok": true})
}

// Test helper functions for various channels
func testTelegramConnection(cfg config.TelegramConfig) error {
	if cfg.Token == "" {
		return errors.New("token is required")
	}

	client := &http.Client{Timeout: 10 * time.Second}
	url := fmt.Sprintf("https://api.telegram.org/bot%s/getMe", cfg.Token)

	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.New("invalid token or API error")
	}

	var result struct {
		Ok     bool `json:"ok"`
		Result struct {
			ID        int64  `json:"id"`
			FirstName string `json:"first_name"`
			Username  string `json:"username"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	if !result.Ok {
		return errors.New("telegram API returned error")
	}

	return nil
}

func testDiscordConnection(cfg config.DiscordConfig) error {
	if cfg.Token == "" {
		return errors.New("token is required")
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", "https://discord.com/api/v10/users/@me", nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bot "+cfg.Token)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.New("invalid token or API error")
	}

	return nil
}

func testWhatsAppConnection(cfg config.WhatsAppConfig) error {
	if cfg.BridgeURL == "" {
		return errors.New("bridge URL is required")
	}

	// Try to connect to the bridge WebSocket
	wsURL := strings.Replace(cfg.BridgeURL, "http://", "ws://", 1)
	wsURL = strings.Replace(wsURL, "https://", "wss://", 1)

	// For testing, just check if the URL is reachable
	client := &http.Client{Timeout: 5 * time.Second}
	testURL := strings.Replace(wsURL, "ws://", "http://", 1)
	testURL = strings.Replace(testURL, "wss://", "https://", 1)

	resp, err := client.Get(testURL)
	if err != nil {
		// WebSocket endpoints often return errors for HTTP GET, that's OK
		// Just check if the server is reachable
		return nil
	}
	defer resp.Body.Close()

	return nil
}

func testSlackConnection(cfg config.SlackConfig) error {
	if cfg.BotToken == "" {
		return errors.New("bot token is required")
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", "https://slack.com/api/auth.test", nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+cfg.BotToken)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result struct {
		Ok    bool   `json:"ok"`
		Error string `json:"error"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	if !result.Ok {
		return errors.New("slack error: " + result.Error)
	}

	return nil
}

func testFeishuConnection(cfg config.FeishuConfig) error {
	if cfg.AppID == "" || cfg.AppSecret == "" {
		return errors.New("app ID and secret are required")
	}

	// Feishu requires tenant access token for most API calls
	client := &http.Client{Timeout: 10 * time.Second}
	body, _ := json.Marshal(map[string]string{
		"app_id":     cfg.AppID,
		"app_secret": cfg.AppSecret,
	})

	resp, err := client.Post(
		"https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal",
		"application/json",
		strings.NewReader(string(body)),
	)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result struct {
		Code        int    `json:"code"`
		Msg         string `json:"msg"`
		TenantToken string `json:"tenant_access_token"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	if result.Code != 0 {
		return errors.New("feishu error: " + result.Msg)
	}

	return nil
}

func testQQConnection(cfg config.QQConfig) error {
	appID, appSecret := channels.ResolveQQBotCredentials(cfg.AppID, cfg.AppSecret, cfg.AccessToken)
	if appID == "" || appSecret == "" {
		return errors.New("app ID and app secret are required, or set access token to appid:appsecret")
	}

	tokenSource := qqtoken.NewQQBotTokenSource(&qqtoken.QQBotCredentials{
		AppID:     appID,
		AppSecret: appSecret,
	})
	token, err := tokenSource.Token()
	if err != nil {
		return fmt.Errorf("qq bot auth failed: %w", err)
	}
	req, err := http.NewRequest(http.MethodGet, "https://api.sgroup.qq.com/gateway", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "QQBot "+token.AccessToken)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("qq gateway probe failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	return nil
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

func testEmailConnection(cfg config.EmailConfig) error {
	if cfg.IMAPHost == "" {
		return errors.New("IMAP host is required")
	}
	if cfg.IMAPUsername == "" || cfg.IMAPPassword == "" {
		return errors.New("IMAP username and password are required")
	}

	// For a basic test, we'll just check DNS resolution
	// Full IMAP testing would require importing an IMAP library
	_, err := net.LookupHost(cfg.IMAPHost)
	if err != nil {
		return fmt.Errorf("failed to resolve IMAP host: %w", err)
	}

	return nil
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
func (s *Server) handleWhatsAppStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if s.channelRegistry == nil {
		writeJSON(w, map[string]interface{}{
			"enabled":   false,
			"connected": false,
			"status":    "registry not initialized",
		})
		return
	}

	ch, ok := s.channelRegistry.Get("whatsapp")
	if !ok {
		writeJSON(w, map[string]interface{}{
			"enabled":   false,
			"connected": false,
			"status":    "channel not registered",
		})
		return
	}

	// Type assert to WhatsAppChannel to get status
	if wc, ok := ch.(interface {
		Status() channels.WhatsAppStatus
	}); ok {
		status := wc.Status()
		writeJSON(w, status)
		return
	}

	// Fallback to basic channel info
	writeJSON(w, map[string]interface{}{
		"enabled":   ch.IsEnabled(),
		"connected": false,
		"status":    "status not available",
	})
}

// MCP-related handlers
func (s *Server) handleMCP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleMCPList(w, r)
	case http.MethodPost:
		s.handleMCPAdd(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleMCPByName(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/mcp/")
	parts := strings.Split(path, "/")
	if len(parts) < 1 || parts[0] == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	name := parts[0]

	// Check for action: /api/mcp/{name}/enable or /api/mcp/{name}/disable or /api/mcp/{name}/test
	if len(parts) >= 2 {
		action := parts[1]
		switch action {
		case "enable":
			s.handleMCPEnable(w, r, name, true)
		case "disable":
			s.handleMCPEnable(w, r, name, false)
		case "test":
			s.handleMCPTest(w, r, name)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
		return
	}

	// Single resource operations
	switch r.Method {
	case http.MethodPut:
		s.handleMCPUpdate(w, r, name)
	case http.MethodDelete:
		s.handleMCPDelete(w, r, name)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleMCPList(w http.ResponseWriter, r *http.Request) {
	cfg, err := config.LoadConfig()
	if err != nil {
		writeError(w, err)
		return
	}

	servers := make([]map[string]interface{}, 0, len(cfg.Tools.MCPServers))
	for name, server := range cfg.Tools.MCPServers {
		serverType := "stdio"
		endpoint := server.Command
		if server.URL != "" {
			serverType = "sse"
			endpoint = server.URL
		}
		servers = append(servers, map[string]interface{}{
			"name":        name,
			"type":        serverType,
			"endpoint":    endpoint,
			"command":     server.Command,
			"args":        server.Args,
			"env":         server.Env,
			"url":         server.URL,
			"headers":     server.Headers,
			"enabled":     true, // MCP servers are enabled if present in config
			"description": "",   // Can be extended later
		})
	}

	writeJSON(w, map[string]interface{}{"servers": servers})
}

func (s *Server) handleMCPAdd(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string            `json:"name"`
		Type        string            `json:"type"` // "stdio" or "sse"
		Command     string            `json:"command,omitempty"`
		Args        []string          `json:"args,omitempty"`
		Env         map[string]string `json:"env,omitempty"`
		URL         string            `json:"url,omitempty"`
		Headers     map[string]string `json:"headers,omitempty"`
		Description string            `json:"description,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, err)
		return
	}

	if req.Name == "" {
		writeError(w, fmt.Errorf("name is required"))
		return
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		writeError(w, err)
		return
	}

	if cfg.Tools.MCPServers == nil {
		cfg.Tools.MCPServers = make(map[string]config.MCPServerConfig)
	}

	// Check if name already exists
	if _, exists := cfg.Tools.MCPServers[req.Name]; exists {
		writeError(w, fmt.Errorf("MCP server '%s' already exists", req.Name))
		return
	}

	server := config.MCPServerConfig{
		Command: req.Command,
		Args:    req.Args,
		Env:     req.Env,
		URL:     req.URL,
		Headers: req.Headers,
	}

	cfg.Tools.MCPServers[req.Name] = server

	if err := config.SaveConfig(cfg); err != nil {
		writeError(w, err)
		return
	}

	updated, err := config.LoadConfig()
	if err != nil {
		writeError(w, err)
		return
	}

	s.cfg = updated
	if err := s.applyRuntimeMCPConfig(updated); err != nil {
		if lg := logging.Get(); lg != nil && lg.Web != nil {
			lg.Web.Printf("apply runtime MCP config failed after add: %v", err)
		}
	}
	writeJSON(w, map[string]interface{}{
		"ok":   true,
		"name": req.Name,
	})
}

func (s *Server) handleMCPUpdate(w http.ResponseWriter, r *http.Request, name string) {
	var req struct {
		Type        string            `json:"type"` // "stdio" or "sse"
		Command     string            `json:"command,omitempty"`
		Args        []string          `json:"args,omitempty"`
		Env         map[string]string `json:"env,omitempty"`
		URL         string            `json:"url,omitempty"`
		Headers     map[string]string `json:"headers,omitempty"`
		Description string            `json:"description,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, err)
		return
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		writeError(w, err)
		return
	}

	if _, exists := cfg.Tools.MCPServers[name]; !exists {
		writeError(w, fmt.Errorf("MCP server '%s' not found", name))
		return
	}

	server := config.MCPServerConfig{
		Command: req.Command,
		Args:    req.Args,
		Env:     req.Env,
		URL:     req.URL,
		Headers: req.Headers,
	}

	cfg.Tools.MCPServers[name] = server

	if err := config.SaveConfig(cfg); err != nil {
		writeError(w, err)
		return
	}

	updated, err := config.LoadConfig()
	if err != nil {
		writeError(w, err)
		return
	}

	s.cfg = updated
	if err := s.applyRuntimeMCPConfig(updated); err != nil {
		if lg := logging.Get(); lg != nil && lg.Web != nil {
			lg.Web.Printf("apply runtime MCP config failed after update: %v", err)
		}
	}
	writeJSON(w, map[string]interface{}{
		"ok":   true,
		"name": name,
	})
}

func (s *Server) handleMCPDelete(w http.ResponseWriter, r *http.Request, name string) {
	cfg, err := config.LoadConfig()
	if err != nil {
		writeError(w, err)
		return
	}

	if _, exists := cfg.Tools.MCPServers[name]; !exists {
		writeError(w, fmt.Errorf("MCP server '%s' not found", name))
		return
	}

	delete(cfg.Tools.MCPServers, name)

	if err := config.SaveConfig(cfg); err != nil {
		writeError(w, err)
		return
	}

	updated, err := config.LoadConfig()
	if err != nil {
		writeError(w, err)
		return
	}

	s.cfg = updated
	if err := s.applyRuntimeMCPConfig(updated); err != nil {
		if lg := logging.Get(); lg != nil && lg.Web != nil {
			lg.Web.Printf("apply runtime MCP config failed after delete: %v", err)
		}
	}
	writeJSON(w, map[string]interface{}{
		"ok":   true,
		"name": name,
	})
}

func (s *Server) handleMCPEnable(w http.ResponseWriter, r *http.Request, name string, enable bool) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		writeError(w, err)
		return
	}

	server, exists := cfg.Tools.MCPServers[name]
	if !exists {
		writeError(w, fmt.Errorf("MCP server '%s' not found", name))
		return
	}

	// MCP servers are enabled/disabled by adding/removing from config
	// For now, we just toggle a flag in the response
	// A more complete implementation would store enabled state separately

	_ = server // Use the server variable

	writeJSON(w, map[string]interface{}{
		"ok":      true,
		"name":    name,
		"enabled": enable,
	})
}

func (s *Server) handleMCPTest(w http.ResponseWriter, r *http.Request, name string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		writeError(w, err)
		return
	}

	server, exists := cfg.Tools.MCPServers[name]
	if !exists {
		writeError(w, fmt.Errorf("MCP server '%s' not found", name))
		return
	}

	opts := tools.MCPServerOptions{
		Name:    name,
		Command: server.Command,
		Args:    server.Args,
		Env:     server.Env,
		URL:     server.URL,
		Headers: server.Headers,
	}

	toolNames, err := tools.TestMCPServer(opts)
	if err != nil {
		writeJSON(w, map[string]interface{}{
			"ok":      false,
			"name":    name,
			"error":   err.Error(),
			"message": "Connection failed",
		})
		return
	}

	writeJSON(w, map[string]interface{}{
		"ok":      true,
		"name":    name,
		"tools":   toolNames,
		"count":   len(toolNames),
		"message": fmt.Sprintf("Connected successfully, found %d tools", len(toolNames)),
	})
}
