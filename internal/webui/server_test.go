package webui

import (
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
