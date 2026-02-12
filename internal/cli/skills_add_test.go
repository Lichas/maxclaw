package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseGitHubSkillSource(t *testing.T) {
	t.Run("owner-repo shorthand", func(t *testing.T) {
		src, err := parseGitHubSkillSource("vercel-labs/agent-skills")
		require.NoError(t, err)
		assert.Equal(t, "vercel-labs/agent-skills", src.OwnerRepo)
		assert.Equal(t, "main", src.Ref)
		assert.Equal(t, "", src.BasePath)
	})

	t.Run("tree url", func(t *testing.T) {
		src, err := parseGitHubSkillSource("https://github.com/vercel-labs/agent-skills/tree/main/skills")
		require.NoError(t, err)
		assert.Equal(t, "vercel-labs/agent-skills", src.OwnerRepo)
		assert.Equal(t, "main", src.Ref)
		assert.Equal(t, "skills", src.BasePath)
	})

	t.Run("invalid host", func(t *testing.T) {
		_, err := parseGitHubSkillSource("https://example.com/a/b")
		require.Error(t, err)
	})
}

func TestFindSkillInRepo(t *testing.T) {
	repo := t.TempDir()

	require.NoError(t, os.MkdirAll(filepath.Join(repo, "skills", "vercel-react-best-practices"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(repo, "skills", "vercel-react-best-practices", "SKILL.md"), []byte("# Vercel React"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(repo, "skills", "simple.md"), []byte("# Simple"), 0644))

	matchDir, err := findSkillInRepo(repo, "", "vercel-react-best-practices")
	require.NoError(t, err)
	require.NotNil(t, matchDir)
	assert.True(t, matchDir.IsDir)
	assert.Equal(t, filepath.Join(repo, "skills", "vercel-react-best-practices"), matchDir.Path)

	matchFile, err := findSkillInRepo(repo, "skills", "simple")
	require.NoError(t, err)
	require.NotNil(t, matchFile)
	assert.False(t, matchFile.IsDir)
	assert.Equal(t, filepath.Join(repo, "skills", "simple.md"), matchFile.Path)
}
