package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Lichas/maxclaw/internal/config"
	"github.com/Lichas/maxclaw/internal/skills"
	workspaceSkills "github.com/Lichas/maxclaw/internal/skills"
	"github.com/spf13/cobra"
)

func init() {
	skillsCmd.AddCommand(skillsListCmd)
	skillsCmd.AddCommand(skillsShowCmd)
	skillsCmd.AddCommand(skillsValidateCmd)
	skillsCmd.AddCommand(skillsAddCmd)
	skillsCmd.AddCommand(skillsInstallCmd)
	skillsCmd.AddCommand(skillsUpdateCmd)

	skillsInstallCmd.Flags().Bool("official", false, "Install official skills from anthropics/skills")
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

var skillsInstallCmd = &cobra.Command{
	Use:   "install [source]",
	Short: "Install skills from a source",
	Long: `Install skills from various sources.

Examples:
  # Install official skills from Anthropic
  maxclaw skills install --official

  # Install from a GitHub repository
  maxclaw skills install github.com/username/repo

  # Install from local directory
  maxclaw skills install /path/to/skills`,
	RunE: func(cmd *cobra.Command, args []string) error {
		official, _ := cmd.Flags().GetBool("official")

		workspace := config.GetWorkspacePath()
		installer := skills.NewInstaller(workspace)

		if official {
			fmt.Println("📦 Installing official skills from anthropics/skills...")
			if err := installer.InstallOfficialSkills(); err != nil {
				return fmt.Errorf("failed to install official skills: %w", err)
			}

			installedSkills, err := installer.ListInstalledSkills()
			if err != nil {
				return err
			}

			if len(installedSkills) > 0 {
				fmt.Printf("\n✓ Installed %d official skills:\n", len(installedSkills))
				for _, skill := range installedSkills {
					fmt.Printf("  - %s\n", skill)
				}
			}
			return nil
		}

		if len(args) == 0 {
			return fmt.Errorf("please specify a source or use --official flag\n\nUsage:\n  maxclaw skills install --official\n  maxclaw skills install github.com/username/repo")
		}

		source := args[0]
		fmt.Printf("📦 Installing skills from %s...\n", source)
		return fmt.Errorf("custom source installation not yet implemented")
	},
}

var skillsUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update official skills to latest version",
	RunE: func(cmd *cobra.Command, args []string) error {
		workspace := config.GetWorkspacePath()
		installer := skills.NewInstaller(workspace)

		markerPath := filepath.Join(workspace, "skills", skills.SkillsInstallMarker)
		_ = os.Remove(markerPath)

		fmt.Println("📦 Updating official skills...")
		if err := installer.InstallOfficialSkills(); err != nil {
			return fmt.Errorf("failed to update skills: %w", err)
		}

		fmt.Println("\n✓ Skills updated successfully!")
		return nil
	},
}
