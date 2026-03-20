package agent

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/Lichas/maxclaw/internal/skills"
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

func (b *ContextBuilder) buildSkillsSection(currentMessage string, explicitSkillRefs []string) string {
	selected := b.resolveSkillEntries(currentMessage, explicitSkillRefs)
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

func (b *ContextBuilder) resolveSkillEntries(currentMessage string, explicitSkillRefs []string) []skills.Entry {
	skillsDir := filepath.Join(b.workspace, "skills")
	entries, err := skills.DiscoverAll(skillsDir, b.enableGlobalSkills)
	if err != nil || len(entries) == 0 {
		return nil
	}

	// Filter out disabled skills.
	stateMgr := skills.NewStateManager(filepath.Join(b.workspace, ".skills_state.json"))
	entries = stateMgr.FilterEnabled(entries)
	if len(entries) == 0 {
		return nil
	}

	if len(explicitSkillRefs) > 0 {
		selectorMessage := strings.TrimSpace(strings.Join(skillRefsToSelectors(explicitSkillRefs), " "))
		return skills.FilterByMessage(entries, selectorMessage)
	}
	return skills.FilterByMessage(entries, currentMessage)
}

func skillRefsToSelectors(refs []string) []string {
	selectors := make([]string, 0, len(refs))
	for _, ref := range refs {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			continue
		}
		selectors = append(selectors, "@skill:"+ref)
	}
	return selectors
}
