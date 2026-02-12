package agent

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildSkillsSectionSelectors(t *testing.T) {
	workspace := t.TempDir()
	skillsDir := filepath.Join(workspace, "skills")
	require.NoError(t, os.MkdirAll(skillsDir, 0755))

	require.NoError(t, os.WriteFile(filepath.Join(skillsDir, "README.md"), []byte("# Skills"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(skillsDir, "alpha.md"), []byte("# Alpha\nDo alpha things."), 0644))

	betaDir := filepath.Join(skillsDir, "beta")
	require.NoError(t, os.MkdirAll(betaDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(betaDir, "SKILL.md"), []byte("# Beta\nDo beta things."), 0644))

	builder := NewContextBuilder(workspace)

	allSkills := builder.buildSkillsSection("do something")
	assert.Contains(t, allSkills, "### Alpha")
	assert.Contains(t, allSkills, "### Beta")

	betaOnly := builder.buildSkillsSection("use @skill:beta")
	assert.NotContains(t, betaOnly, "### Alpha")
	assert.Contains(t, betaOnly, "### Beta")

	alphaOnly := builder.buildSkillsSection("use $alpha now")
	assert.Contains(t, alphaOnly, "### Alpha")
	assert.NotContains(t, alphaOnly, "### Beta")

	none := builder.buildSkillsSection("disable with @skill:none")
	assert.Equal(t, "", none)
}
