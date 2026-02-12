package agent

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/Lichas/nanobot-go/internal/skills"
)

const (
	maxSkillRunes       = 12000
	maxSkillsTotalRunes = 60000
)

func truncateRunes(s string, limit int, suffix string) string {
	if limit <= 0 {
		return s
	}
	if utf8.RuneCountInString(s) <= limit {
		return s
	}
	runes := []rune(s)
	if limit > len(runes) {
		return s
	}
	return strings.TrimSpace(string(runes[:limit])) + suffix
}

func (b *ContextBuilder) buildSkillsSection(currentMessage string) string {
	skillsDir := filepath.Join(b.workspace, "skills")
	entries, err := skills.Discover(skillsDir)
	if err != nil || len(entries) == 0 {
		return ""
	}

	selected := skills.FilterByMessage(entries, currentMessage)
	if len(selected) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Skills\n")

	used := 0
	for _, entry := range selected {
		body := truncateRunes(entry.Body, maxSkillRunes, "\n\n... (skill truncated)")
		section := fmt.Sprintf("### %s\n%s\n\n", entry.DisplayName, body)
		sectionRunes := utf8.RuneCountInString(section)
		if used+sectionRunes > maxSkillsTotalRunes {
			sb.WriteString("... (skills truncated)\n")
			break
		}
		sb.WriteString(section)
		used += sectionRunes
	}

	return strings.TrimSpace(sb.String())
}
