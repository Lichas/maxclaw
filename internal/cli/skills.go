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

  # Install entire repository
  maxclaw skills install github.com/username/repo

  # Install specific skill (subdirectory)
  maxclaw skills install github.com/openclaw/skills/tree/main/skills/steipete/weather

  # Install single skill file
  maxclaw skills install github.com/openclaw/skills/blob/main/skills/weather/SKILL.md

  # Install from local directory
  maxclaw skills install /path/to/skills`,
	RunE: func(cmd *cobra.Command, args []string) error {
		official, _ := cmd.Flags().GetBool("official")

		workspace := config.GetWorkspacePath()
		installer := skills.NewInstaller(workspace)

		if official {
			fmt.Println("📦 Installing official skills...")
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
		return installFromSource(installer, workspace, source)
	},
}

// installFromSource 智能识别源类型并安装
func installFromSource(installer *skills.Installer, workspace, source string) error {
	// 标准化 source（去掉 https:// 前缀）
	source = strings.TrimPrefix(source, "https://")
	source = strings.TrimPrefix(source, "http://")
	source = strings.TrimSuffix(source, "/")

	// 判断是否为 GitHub URL
	if isGitHubURL(source) {
		return installFromGitHub(installer, workspace, source)
	}

	// 本地路径
	if _, err := os.Stat(source); err == nil {
		fmt.Printf("📦 Installing from local path: %s\n", source)
		return installFromLocal(workspace, source)
	}

	return fmt.Errorf("unsupported source: %s", source)
}

// isGitHubURL 判断是否为 GitHub URL
func isGitHubURL(source string) bool {
	return strings.HasPrefix(source, "github.com/") ||
		strings.HasPrefix(source, "raw.githubusercontent.com/")
}

// installFromGitHub 从 GitHub 智能安装
func installFromGitHub(installer *skills.Installer, workspace, source string) error {
	repoInfo := parseGitHubURL(source)

	switch repoInfo.Type {
	case "file":
		// 单文件（blob 链接）
		fmt.Printf("📦 Installing single skill file from %s/%s...\n", repoInfo.Owner, repoInfo.Repo)
		return installer.InstallSingleFile(skills.GitHubSource(repoInfo))

	case "dir":
		// 子目录（tree 链接）
		fmt.Printf("📦 Installing skill directory from %s/%s/%s...\n", repoInfo.Owner, repoInfo.Repo, repoInfo.Path)
		return installer.InstallSubPath(skills.GitHubSource(repoInfo))

	case "repo":
		// 整个仓库
		fmt.Printf("📦 Installing skills from %s/%s...\n", repoInfo.Owner, repoInfo.Repo)
		repo := skills.OfficialRepo{
			Name: fmt.Sprintf("%s/%s", repoInfo.Owner, repoInfo.Repo),
			Repo: fmt.Sprintf("%s/%s", repoInfo.Owner, repoInfo.Repo),
		}
		_, err := installer.InstallRepoSkills(repo)
		return err

	default:
		return fmt.Errorf("unrecognized GitHub URL format: %s", source)
	}
}

// GitHubRepoInfo 解析后的 GitHub 信息（内部使用，与 skills.GitHubSource 结构一致）
type GitHubRepoInfo struct {
	Owner  string
	Repo   string
	Branch string
	Path   string
	Type   string // "file", "dir", "repo"
}

// parseGitHubURL 解析 GitHub URL
// 支持格式：
// - github.com/owner/repo
// - github.com/owner/repo/tree/branch/path
// - github.com/owner/repo/blob/branch/path/file.md
// - raw.githubusercontent.com/owner/repo/branch/path/file.md
func parseGitHubURL(source string) GitHubRepoInfo {
	info := GitHubRepoInfo{Type: "repo"}

	// 处理 raw.githubusercontent.com
	if strings.HasPrefix(source, "raw.githubusercontent.com/") {
		parts := strings.SplitN(strings.TrimPrefix(source, "raw.githubusercontent.com/"), "/", 5)
		if len(parts) >= 4 {
			info.Owner = parts[0]
			info.Repo = parts[1]
			info.Branch = parts[2]
			info.Path = parts[3]
			info.Type = "file"
		}
		return info
	}

	// 处理 github.com
	parts := strings.SplitN(strings.TrimPrefix(source, "github.com/"), "/", 5)
	if len(parts) < 2 {
		return info
	}

	info.Owner = parts[0]
	info.Repo = parts[1]

	if len(parts) < 4 {
		return info
	}

	// 判断是 tree 还是 blob
	mode := parts[2]
	info.Branch = parts[3]

	if len(parts) >= 5 {
		info.Path = parts[4]
	}

	if mode == "blob" {
		info.Type = "file"
	} else if mode == "tree" {
		info.Type = "dir"
	}

	return info
}

// installFromLocal 从本地路径安装
func installFromLocal(workspace, source string) error {
	info, err := os.Stat(source)
	if err != nil {
		return err
	}

	targetDir := filepath.Join(workspace, "skills")

	if info.IsDir() {
		// 如果是目录，复制所有 .md 文件
		entries, err := os.ReadDir(source)
		if err != nil {
			return err
		}

		count := 0
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
				srcPath := filepath.Join(source, entry.Name())
				dstPath := filepath.Join(targetDir, entry.Name())
				data, err := os.ReadFile(srcPath)
				if err != nil {
					continue
				}
				if err := os.WriteFile(dstPath, data, 0644); err != nil {
					continue
				}
				count++
			}
		}
		fmt.Printf("✓ Installed %d skills\n", count)
		return nil
	}

	// 单文件
	if strings.HasSuffix(source, ".md") {
		data, err := os.ReadFile(source)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(targetDir, filepath.Base(source))
		if err := os.WriteFile(dstPath, data, 0644); err != nil {
			return err
		}
		fmt.Printf("✓ Installed %s\n", filepath.Base(source))
		return nil
	}

	return fmt.Errorf("unsupported file type: %s", source)
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
