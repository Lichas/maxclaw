package webui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Lichas/maxclaw/internal/agent"
	"github.com/Lichas/maxclaw/internal/bus"
	"github.com/Lichas/maxclaw/internal/config"
	"github.com/Lichas/maxclaw/internal/session"
	"github.com/Lichas/maxclaw/pkg/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnrichContentWithAttachments(t *testing.T) {
	workspace := filepath.Join(string(filepath.Separator), "tmp", "ws")
	s := &Server{
		cfg: &config.Config{
			Agents: config.AgentsConfig{
				Defaults: config.AgentDefaults{
					Workspace: workspace,
				},
			},
		},
	}

	content := "总结一下这个文件"
	attachments := []messageAttachment{
		{
			Filename: "report.md",
			Path:     filepath.Join(workspace, ".uploads", "20260222_abcd1234.md"),
		},
	}

	out := s.enrichContentWithAttachments(content, attachments)
	assert.Contains(t, out, content)
	assert.Contains(t, out, "Attached files (local paths):")
	assert.Contains(t, out, "report.md")
	assert.Contains(t, out, filepath.Join(workspace, ".uploads", "20260222_abcd1234.md"))
	assert.Contains(t, out, "read it from the path above")
}

func TestEnrichContentWithAttachmentsURLFallbackAndDeduplicate(t *testing.T) {
	workspace := filepath.Join(string(filepath.Separator), "tmp", "ws")
	s := &Server{
		cfg: &config.Config{
			Agents: config.AgentsConfig{
				Defaults: config.AgentDefaults{
					Workspace: workspace,
				},
			},
		},
	}

	content := "请处理附件"
	attachments := []messageAttachment{
		{
			Filename: "plan.docx",
			URL:      "/api/uploads/20260222_a1b2c3d4.docx",
		},
		{
			Filename: "plan-copy.docx",
			URL:      "/api/uploads/20260222_a1b2c3d4.docx",
		},
	}

	out := s.enrichContentWithAttachments(content, attachments)
	expectedPath := filepath.Join(workspace, ".uploads", "20260222_a1b2c3d4.docx")
	assert.Contains(t, out, expectedPath)
	assert.Equal(t, 1, strings.Count(out, expectedPath))
}

func TestExtractImageAttachmentUsesLocalImagePath(t *testing.T) {
	workspace := filepath.Join(string(filepath.Separator), "tmp", "ws")
	s := &Server{
		cfg: &config.Config{
			Agents: config.AgentsConfig{
				Defaults: config.AgentDefaults{
					Workspace: workspace,
				},
			},
		},
	}

	got := s.extractImageAttachment([]messageAttachment{
		{
			Filename: "screenshot.png",
			Path:     filepath.Join(workspace, ".uploads", "20260222_image.png"),
		},
	})

	assert.Equal(t, &bus.MediaAttachment{
		Type:      "image",
		Filename:  "screenshot.png",
		LocalPath: filepath.Join(workspace, ".uploads", "20260222_image.png"),
		MimeType:  "image/png",
	}, got)
}

func TestExtractImageAttachmentIgnoresNonImageFiles(t *testing.T) {
	workspace := filepath.Join(string(filepath.Separator), "tmp", "ws")
	s := &Server{
		cfg: &config.Config{
			Agents: config.AgentsConfig{
				Defaults: config.AgentDefaults{
					Workspace: workspace,
				},
			},
		},
	}

	got := s.extractImageAttachment([]messageAttachment{
		{
			Filename: "report.pdf",
			Path:     filepath.Join(workspace, ".uploads", "20260222_report.pdf"),
		},
	})

	assert.Nil(t, got)
}

func TestNormalizeMiniMaxBaseURLRewritesLegacyChinaDomain(t *testing.T) {
	got := normalizeMiniMaxBaseURL("https://api.minimax.com/v1")
	assert.Equal(t, "https://api.minimaxi.com/v1", got)
}

func TestHandleTestProviderMiniMaxUsesRawAuthorizationAndChatCompletions(t *testing.T) {
	var (
		requestPath string
		authHeader  string
	)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		authHeader = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"ok"}`))
	}))
	defer upstream.Close()

	s := &Server{}
	body := bytes.NewBufferString(fmt.Sprintf(`{"name":"MiniMax","apiKey":"sk-mini","baseURL":"%s/v1","apiFormat":"openai"}`, upstream.URL))
	req := httptest.NewRequest(http.MethodPost, "/api/providers/test", body)
	rec := httptest.NewRecorder()

	s.handleTestProvider(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "/v1/chat/completions", requestPath)
	assert.Equal(t, "Bearer sk-mini", authHeader)
}

func TestHandleTestProviderAnthropicUsesOfficialSDKModelsEndpoint(t *testing.T) {
	var (
		requestPath string
		apiKey      string
	)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		apiKey = r.Header.Get("X-Api-Key")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[],"first_id":"","has_more":false,"last_id":""}`))
	}))
	defer upstream.Close()

	s := &Server{}
	body := bytes.NewBufferString(fmt.Sprintf(`{"name":"Anthropic","apiKey":"sk-ant","baseURL":"%s","apiFormat":"anthropic"}`, upstream.URL))
	req := httptest.NewRequest(http.MethodPost, "/api/providers/test", body)
	rec := httptest.NewRecorder()

	s.handleTestProvider(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "/v1/models", requestPath)
	assert.Equal(t, "sk-ant", apiKey)
}

func TestHandleTestProviderOpenAIUsesOfficialSDKModelsEndpoint(t *testing.T) {
	var (
		requestPath string
		authHeader  string
	)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		authHeader = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[]}`))
	}))
	defer upstream.Close()

	s := &Server{}
	body := bytes.NewBufferString(fmt.Sprintf(`{"name":"OpenAI","apiKey":"sk-openai","baseURL":"%s/v1","apiFormat":"openai"}`, upstream.URL))
	req := httptest.NewRequest(http.MethodPost, "/api/providers/test", body)
	rec := httptest.NewRecorder()

	s.handleTestProvider(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "/v1/models", requestPath)
	assert.Equal(t, "Bearer sk-openai", authHeader)
}

func TestReadChannelSenderStatsAggregatesInboundMessages(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "session.log")
	content := strings.Join([]string{
		`2026/03/08 10:21:38.999215 inbound channel=qq chat=qq-openid-1 sender=qq-openid-1 content="first"`,
		`2026/03/08 10:22:15.668501 inbound channel=qq chat=qq-openid-1 sender=qq-openid-1 content="second message"`,
		`2026/03/08 10:23:08.935875 inbound channel=telegram chat=123 sender=alice content="hello tg"`,
		`2026/03/08 10:24:24.413482 outbound channel=qq chat=qq-openid-1 content="ignored"`,
		"",
	}, "\n")
	assert.NoError(t, os.WriteFile(logPath, []byte(content), 0644))

	stats, err := readChannelSenderStats(logPath, "", 10)
	assert.NoError(t, err)
	if assert.Len(t, stats, 2) {
		assert.Equal(t, "telegram", stats[0].Channel)
		assert.Equal(t, "alice", stats[0].Sender)
		assert.Equal(t, 1, stats[0].MessageCount)
		assert.Equal(t, "hello tg", stats[0].LatestMessage)

		assert.Equal(t, "qq", stats[1].Channel)
		assert.Equal(t, "qq-openid-1", stats[1].Sender)
		assert.Equal(t, "qq-openid-1", stats[1].ChatID)
		assert.Equal(t, 2, stats[1].MessageCount)
		assert.Equal(t, "second message", stats[1].LatestMessage)
	}
}

func TestHandleSkillSourcesReturnsRecommendedSources(t *testing.T) {
	s := &Server{}

	req := httptest.NewRequest(http.MethodGet, "/api/skills/sources", nil)
	rec := httptest.NewRecorder()
	s.handleSkillSources(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var payload struct {
		Sources []struct {
			ID   string `json:"id"`
			Type string `json:"type"`
		} `json:"sources"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.NotEmpty(t, payload.Sources)

	hasClawHub := false
	for _, source := range payload.Sources {
		if source.ID == "clawhub" && source.Type == "clawhub" {
			hasClawHub = true
			break
		}
	}
	assert.True(t, hasClawHub)
}

func TestReadChannelSenderStatsFiltersByChannel(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "session.log")
	content := strings.Join([]string{
		`2026/03/08 10:21:38.999215 inbound channel=qq chat=qq-openid-1 sender=qq-openid-1 content="first"`,
		`2026/03/08 10:23:08.935875 inbound channel=telegram chat=123 sender=alice content="hello tg"`,
		"",
	}, "\n")
	assert.NoError(t, os.WriteFile(logPath, []byte(content), 0644))

	stats, err := readChannelSenderStats(logPath, "qq", 10)
	assert.NoError(t, err)
	if assert.Len(t, stats, 1) {
		assert.Equal(t, "qq", stats[0].Channel)
		assert.Equal(t, "qq-openid-1", stats[0].Sender)
	}
}

func TestListSessionsBackfillsLegacyTitle(t *testing.T) {
	workspace := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(workspace, ".sessions"), 0755))

	legacy := map[string]any{
		"key": "desktop:legacy",
		"messages": []map[string]any{
			{
				"role":      "user",
				"content":   "帮我检查 QQ 图片消息为什么没有回复",
				"timestamp": "2026-03-09T10:00:00Z",
			},
		},
	}
	data, err := json.MarshalIndent(legacy, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(workspace, ".sessions", "desktop_legacy.json"), data, 0644))

	list, err := listSessions(workspace)
	require.NoError(t, err)
	if assert.Len(t, list, 1) {
		assert.Equal(t, "检查 QQ 图片消息为什么没有回复", list[0].Title)
		assert.Equal(t, "帮我检查 QQ 图片消息为什么没有回复", list[0].LastMessage)
	}

	mgr := session.NewManager(workspace)
	loaded := mgr.GetOrCreate("desktop:legacy")
	assert.Equal(t, "检查 QQ 图片消息为什么没有回复", loaded.Title)
	assert.Equal(t, session.TitleSourceAuto, loaded.TitleSource)
}

func TestRenameSessionStoresDedicatedTitle(t *testing.T) {
	workspace := t.TempDir()
	mgr := session.NewManager(workspace)
	sess := mgr.GetOrCreate("desktop:test")
	sess.AddMessage("user", "帮我修复 telegram 图片消息")
	sess.AddMessage("assistant", "好的")
	require.NoError(t, mgr.Save(sess))

	s := &Server{
		cfg: &config.Config{
			Agents: config.AgentsConfig{
				Defaults: config.AgentDefaults{Workspace: workspace},
			},
		},
	}

	body := bytes.NewBufferString(`{"title":"Telegram 图片支持修复"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/sessions/desktop:test/rename", body)
	rec := httptest.NewRecorder()

	s.handleSessionPost(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	updated := session.NewManager(workspace).GetOrCreate("desktop:test")
	assert.Equal(t, "Telegram 图片支持修复", updated.Title)
	assert.Equal(t, session.TitleSourceUser, updated.TitleSource)
	require.Len(t, updated.Messages, 2)
	assert.Equal(t, "好的", updated.Messages[1].Content)
}

func TestHandleMCPAddAndDeleteRefreshRuntimeTools(t *testing.T) {
	configHome := t.TempDir()
	workspace := filepath.Join(configHome, "workspace")
	t.Setenv("MAXCLAW_HOME", configHome)

	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Workspace = workspace
	require.NoError(t, config.SaveConfig(cfg))

	loop := agent.NewAgentLoop(
		bus.NewMessageBus(10),
		nil,
		workspace,
		"test-model",
		3,
		"",
		tools.WebFetchOptions{},
		config.ExecToolConfig{Timeout: 5},
		false,
		nil,
		nil,
		false,
	)
	defer loop.Close()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     interface{} `json:"id"`
			Method string      `json:"method"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Mcp-Session-Id", "test-session")

		switch req.Method {
		case "initialize":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"result": map[string]interface{}{
					"protocolVersion": "2024-11-05",
					"serverInfo": map[string]interface{}{
						"name":    "test-server",
						"version": "1.0.0",
					},
					"capabilities": map[string]interface{}{
						"tools": map[string]interface{}{},
					},
				},
			}))
		case "notifications/initialized":
			w.WriteHeader(http.StatusOK)
		case "tools/list":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"result": map[string]interface{}{
					"tools": []map[string]interface{}{
						{
							"name":        "ping",
							"description": "Ping tool",
							"inputSchema": map[string]interface{}{
								"type":       "object",
								"properties": map[string]interface{}{},
							},
						},
					},
				},
			}))
		default:
			t.Fatalf("unexpected MCP method: %s", req.Method)
		}
	}))
	defer upstream.Close()

	s := &Server{cfg: cfg, agentLoop: loop}

	addReq := httptest.NewRequest(http.MethodPost, "/api/mcp", bytes.NewBufferString(fmt.Sprintf(`{
		"name":"docs",
		"type":"sse",
		"url":"%s"
	}`, upstream.URL)))
	addRec := httptest.NewRecorder()
	s.handleMCP(addRec, addReq)
	require.Equal(t, http.StatusOK, addRec.Code)
	assert.Contains(t, loop.ListToolNames(), "mcp_docs_ping")

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/mcp/docs", nil)
	deleteRec := httptest.NewRecorder()
	s.handleMCPByName(deleteRec, deleteReq)
	require.Equal(t, http.StatusOK, deleteRec.Code)
	assert.NotContains(t, loop.ListToolNames(), "mcp_docs_ping")
}
