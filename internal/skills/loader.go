package skills

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var (
	atSkillPattern     = regexp.MustCompile(`(?i)@skill:([a-z0-9_.-]+)`)
	dollarSkillPattern = regexp.MustCompile(`\$([a-zA-Z][a-zA-Z0-9_.-]*)`)
)

// Entry is a single skill document discovered from workspace/skills.
type Entry struct {
	Name        string
	DisplayName string
	Path        string
	Body        string
}

// Discover loads markdown skill files from skillsDir.
// Supported layouts:
//   - skills/<name>.md
//   - skills/<name>/SKILL.md
func Discover(skillsDir string) ([]Entry, error) {
	info, err := os.Stat(skillsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("skills path is not a directory: %s", skillsDir)
	}

	var entries []Entry
	err = filepath.WalkDir(skillsDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if path == skillsDir {
				return nil
			}
			if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
				return filepath.SkipDir
			}
			return nil
		}

		if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
			return nil
		}
		lowerName := strings.ToLower(name)
		parentDir := filepath.Dir(path)
		isTopLevelMarkdown := parentDir == skillsDir && strings.HasSuffix(lowerName, ".md") && lowerName != "readme.md"
		isSkillDoc := strings.EqualFold(name, "SKILL.md")
		if !isTopLevelMarkdown && !isSkillDoc {
			return nil
		}

		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}

		skillName := inferSkillName(path)
		title, body := extractTitleAndBody(string(content))
		if title == "" {
			title = skillName
		}

		body = strings.TrimSpace(body)
		if body == "" {
			body = "(empty skill)"
		}

		entries = append(entries, Entry{
			Name:        strings.ToLower(skillName),
			DisplayName: title,
			Path:        path,
			Body:        body,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})
	return entries, nil
}

// FilterByMessage returns selected skills based on references in message.
// Supported selectors:
//   - @skill:<name>
//   - $<name>
//
// Special selectors:
//   - @skill:all / $all
//   - @skill:none / $none
//
// If no selector is present, all skills are returned.
func FilterByMessage(entries []Entry, message string) []Entry {
	refs := extractRefs(message)
	if len(refs) == 0 {
		return entries
	}

	for _, ref := range refs {
		switch ref {
		case "all":
			return entries
		case "none":
			return nil
		}
	}

	directIndex := make(map[string][]int)
	canonicalIndex := make(map[string][]int)
	for idx, entry := range entries {
		for _, key := range entryKeys(entry) {
			if key == "" {
				continue
			}
			directIndex[key] = append(directIndex[key], idx)
			canonical := canonicalName(key)
			if canonical != "" {
				canonicalIndex[canonical] = append(canonicalIndex[canonical], idx)
			}
		}
	}

	seen := make(map[int]struct{})
	filtered := make([]Entry, 0, len(refs))
	for _, ref := range refs {
		indices := directIndex[ref]
		if len(indices) == 0 {
			indices = canonicalIndex[canonicalName(ref)]
		}
		for _, idx := range indices {
			if _, ok := seen[idx]; ok {
				continue
			}
			filtered = append(filtered, entries[idx])
			seen[idx] = struct{}{}
		}
	}

	return filtered
}

func inferSkillName(path string) string {
	base := filepath.Base(path)
	if strings.EqualFold(base, "SKILL.md") {
		return filepath.Base(filepath.Dir(path))
	}
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func extractTitleAndBody(content string) (string, string) {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		trim := strings.TrimSpace(line)
		if trim == "" {
			continue
		}
		if strings.HasPrefix(trim, "# ") {
			title := strings.TrimSpace(strings.TrimPrefix(trim, "# "))
			return title, strings.Join(lines[i+1:], "\n")
		}
		break
	}
	return "", content
}

// extractRefs 从用户消息中提取技能引用。
// 输入: message 用户消息字符串（包含 @skill:<name> 或 $<name> 的引用）。
// 输出: refs 已标准化为小写的技能名切片；若无引用则返回空切片。
func extractRefs(message string) []string {
	var refs []string
	for _, match := range atSkillPattern.FindAllStringSubmatch(message, -1) {
		if len(match) < 2 {
			continue
		}
		ref := strings.ToLower(strings.TrimSpace(match[1]))
		if ref != "" {
			refs = append(refs, ref)
		}
	}
	for _, match := range dollarSkillPattern.FindAllStringSubmatch(message, -1) {
		if len(match) < 2 {
			continue
		}
		ref := strings.ToLower(strings.TrimSpace(match[1]))
		if ref != "" {
			refs = append(refs, ref)
		}
	}
	return refs
}

func entryKeys(entry Entry) []string {
	keys := []string{
		strings.ToLower(strings.TrimSpace(entry.Name)),
		strings.ToLower(strings.TrimSpace(entry.DisplayName)),
	}
	seen := make(map[string]struct{}, len(keys))
	uniq := make([]string, 0, len(keys))
	for _, key := range keys {
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		uniq = append(uniq, key)
	}
	return uniq
}

func canonicalName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}
