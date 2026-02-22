package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Lichas/maxclaw/internal/config"
	workspaceSkills "github.com/Lichas/maxclaw/internal/skills"
	"github.com/spf13/cobra"
)

func init() {
	skillsCmd.AddCommand(skillsListCmd)
	skillsCmd.AddCommand(skillsShowCmd)
	skillsCmd.AddCommand(skillsValidateCmd)
	skillsCmd.AddCommand(skillsAddCmd)
}

var skillsCmd = &cobra.Command{
	Use:   "skills",
	Short: "Manage workspace skills",
}

var skillsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List discovered skills in workspace",
	RunE: func(cmd *cobra.Command, args []string) error {
		workspace, entries, err := discoverWorkspaceSkills()
		if err != nil {
			return err
		}

		if len(entries) == 0 {
			fmt.Printf("No skills found in %s\n", filepath.Join(workspace, "skills"))
			return nil
		}

		fmt.Printf("Skills in %s:\n\n", filepath.Join(workspace, "skills"))
		for _, entry := range entries {
			relPath := entry.Path
			if rel, relErr := filepath.Rel(workspace, entry.Path); relErr == nil {
				relPath = rel
			}
			fmt.Printf("- %s (%s)\n", entry.Name, relPath)
		}
		fmt.Printf("\nTotal: %d\n", len(entries))
		return nil
	},
}

var skillsShowCmd = &cobra.Command{
	Use:   "show [skill-name]",
	Short: "Show one skill content",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		_, entries, err := discoverWorkspaceSkills()
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			return fmt.Errorf("no skills found")
		}

		selected := resolveSkill(entries, args[0])
		if selected == nil {
			return fmt.Errorf("skill not found: %s", args[0])
		}

		fmt.Printf("# %s\n\n", selected.DisplayName)
		fmt.Printf("Name: %s\n", selected.Name)
		fmt.Printf("Path: %s\n\n", selected.Path)
		fmt.Println(selected.Body)
		return nil
	},
}

var skillsValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate skills naming collisions",
	RunE: func(cmd *cobra.Command, args []string) error {
		workspace, entries, err := discoverWorkspaceSkills()
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			fmt.Printf("No skills found in %s\n", filepath.Join(workspace, "skills"))
			return nil
		}

		seen := map[string]string{}
		seenCanonical := map[string]string{}
		var issues []string
		for _, entry := range entries {
			if prevPath, ok := seen[entry.Name]; ok {
				issues = append(issues, fmt.Sprintf("duplicate skill name %q: %s and %s", entry.Name, prevPath, entry.Path))
			} else {
				seen[entry.Name] = entry.Path
			}

			canon := canonicalSkillName(entry.Name)
			if canon == "" {
				continue
			}
			if prevPath, ok := seenCanonical[canon]; ok && prevPath != entry.Path {
				issues = append(issues, fmt.Sprintf("canonical name collision %q: %s and %s", canon, prevPath, entry.Path))
			} else {
				seenCanonical[canon] = entry.Path
			}
		}

		if len(issues) > 0 {
			fmt.Println("Skill validation failed:")
			for _, issue := range issues {
				fmt.Printf("- %s\n", issue)
			}
			return fmt.Errorf("found %d skill issue(s)", len(issues))
		}

		fmt.Printf("Skills validation passed (%d skills)\n", len(entries))
		return nil
	},
}

func discoverWorkspaceSkills() (string, []workspaceSkills.Entry, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return "", nil, fmt.Errorf("failed to load config: %w", err)
	}

	workspace := cfg.Agents.Defaults.Workspace
	entries, err := workspaceSkills.Discover(filepath.Join(workspace, "skills"))
	if err != nil {
		return workspace, nil, err
	}
	return workspace, entries, nil
}

func resolveSkill(entries []workspaceSkills.Entry, ref string) *workspaceSkills.Entry {
	ref = strings.ToLower(strings.TrimSpace(ref))
	if ref == "" {
		return nil
	}

	for i := range entries {
		if entries[i].Name == ref {
			return &entries[i]
		}
	}

	refCanonical := canonicalSkillName(ref)
	for i := range entries {
		if canonicalSkillName(entries[i].Name) == refCanonical {
			return &entries[i]
		}
	}
	return nil
}

func canonicalSkillName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}
