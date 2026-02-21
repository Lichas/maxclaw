package webui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Lichas/nanobot-go/internal/agent"
	"github.com/Lichas/nanobot-go/internal/channels"
	"github.com/Lichas/nanobot-go/internal/config"
	"github.com/Lichas/nanobot-go/internal/cron"
	"github.com/Lichas/nanobot-go/internal/logging"
	"github.com/Lichas/nanobot-go/internal/session"
)

type Server struct {
	cfg             *config.Config
	agentLoop       *agent.AgentLoop
	cronService     *cron.Service
	channelRegistry *channels.Registry
	server          *http.Server
	uiDir           string
}

type messagePayload struct {
	SessionKey string `json:"sessionKey"`
	Content    string `json:"content"`
	Channel    string `json:"channel"`
	ChatID     string `json:"chatId"`
	Stream     bool   `json:"stream,omitempty"`
}

func NewServer(cfg *config.Config, agentLoop *agent.AgentLoop, cronService *cron.Service, registry *channels.Registry) *Server {
	return &Server{
		cfg:             cfg,
		agentLoop:       agentLoop,
		cronService:     cronService,
		channelRegistry: registry,
		uiDir:           findUIDir(),
	}
}

func (s *Server) Start(ctx context.Context, host string, port int) error {
	addr := fmt.Sprintf("%s:%d", host, port)
	mux := http.NewServeMux()

	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/sessions", s.handleSessions)
	mux.HandleFunc("/api/sessions/", s.handleSessionByKey)
	mux.HandleFunc("/api/message", s.handleMessage)
	mux.HandleFunc("/api/config", s.handleConfig)
	mux.HandleFunc("/api/gateway/restart", s.handleGatewayRestart)

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
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	key := strings.TrimPrefix(r.URL.Path, "/api/sessions/")
	if key == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	mgr := session.NewManager(s.cfg.Agents.Defaults.Workspace)
	sess := mgr.GetOrCreate(key)
	writeJSON(w, sess)
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

	resp, err := s.agentLoop.ProcessDirect(r.Context(), payload.Content, payload.SessionKey, payload.Channel, payload.ChatID)
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
	resp, err := s.agentLoop.ProcessDirectEventStream(
		ctx,
		payload.Content,
		payload.SessionKey,
		payload.Channel,
		payload.ChatID,
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

type sessionSummary struct {
	Key           string `json:"key"`
	MessageCount  int    `json:"messageCount"`
	LastMessageAt string `json:"lastMessageAt,omitempty"`
	LastMessage   string `json:"lastMessage,omitempty"`
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
