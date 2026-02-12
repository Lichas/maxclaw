package skills

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscoverAndFilter(t *testing.T) {
	base := t.TempDir()
	skillsDir := filepath.Join(base, "skills")
	require.NoError(t, os.MkdirAll(skillsDir, 0755))

	require.NoError(t, os.WriteFile(filepath.Join(skillsDir, "README.md"), []byte("# Skills"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(skillsDir, "web-fetch.md"), []byte("# Web Fetch\nBrowser mode."), 0644))

	bridgeDir := filepath.Join(skillsDir, "bridge_tools")
	require.NoError(t, os.MkdirAll(bridgeDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(bridgeDir, "SKILL.md"), []byte("# Bridge Tools\nManage bridge lifecycle."), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(bridgeDir, "README.md"), []byte("# Docs"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(bridgeDir, "rule-a.md"), []byte("# Rule A"), 0644))

	hiddenDir := filepath.Join(skillsDir, ".private")
	require.NoError(t, os.MkdirAll(hiddenDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(hiddenDir, "secret.md"), []byte("# Secret"), 0644))

	entries, err := Discover(skillsDir)
	require.NoError(t, err)
	require.Len(t, entries, 2)

	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name)
	}
	assert.ElementsMatch(t, []string{"web-fetch", "bridge_tools"}, names)

	byAtSkill := FilterByMessage(entries, "please use @skill:web-fetch")
	require.Len(t, byAtSkill, 1)
	assert.Equal(t, "web-fetch", byAtSkill[0].Name)

	byDollarSkill := FilterByMessage(entries, "run with $bridge_tools")
	require.Len(t, byDollarSkill, 1)
	assert.Equal(t, "bridge_tools", byDollarSkill[0].Name)

	byCanonicalAlias := FilterByMessage(entries, "use @skill:bridgetools")
	require.Len(t, byCanonicalAlias, 1)
	assert.Equal(t, "bridge_tools", byCanonicalAlias[0].Name)

	none := FilterByMessage(entries, "@skill:none")
	assert.Len(t, none, 0)

	all := FilterByMessage(entries, "$all")
	assert.Len(t, all, 2)
}
