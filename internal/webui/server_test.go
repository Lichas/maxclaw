package webui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Lichas/maxclaw/internal/config"
	"github.com/stretchr/testify/assert"
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
