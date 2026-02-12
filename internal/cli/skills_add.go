package cli

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Lichas/nanobot-go/internal/config"
	"github.com/spf13/cobra"
)

var (
	skillsAddItems []string
	skillsAddRef   string
	skillsAddPath  string
	skillsAddName  string
)

func init() {
	skillsAddCmd.Flags().StringArrayVar(&skillsAddItems, "skill", nil, "Skill name/path to install (repeatable)")
	skillsAddCmd.Flags().StringVar(&skillsAddRef, "ref", "", "Git ref (branch/tag/commit), default main")
	skillsAddCmd.Flags().StringVar(&skillsAddPath, "path", "", "Base path in repo where skills are located")
	skillsAddCmd.Flags().StringVar(&skillsAddName, "name", "", "Destination skill name (single skill only)")
}

var skillsAddCmd = &cobra.Command{
	Use:   "add [github-repo-or-url]",
	Short: "Install one or more skills from a GitHub repository",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(skillsAddItems) == 0 {
			return fmt.Errorf("at least one --skill is required")
		}
		if skillsAddName != "" && len(skillsAddItems) != 1 {
			return fmt.Errorf("--name can only be used with exactly one --skill")
		}

		src, err := parseGitHubSkillSource(args[0])
		if err != nil {
			return err
		}
		if skillsAddRef != "" {
			src.Ref = skillsAddRef
		}
		if skillsAddPath != "" {
			src.BasePath = cleanRepoSubPath(skillsAddPath)
		}

		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		skillsRoot := filepath.Join(cfg.Agents.Defaults.Workspace, "skills")
		if err := os.MkdirAll(skillsRoot, 0755); err != nil {
			return fmt.Errorf("failed to create skills directory: %w", err)
		}

		repoDir, err := cloneRepo(src)
		if err != nil {
			return err
		}
		defer os.RemoveAll(repoDir)

		fmt.Printf("Repository: %s (ref=%s)\n", src.OwnerRepo, src.Ref)
		if src.BasePath != "" {
			fmt.Printf("Base path: %s\n", src.BasePath)
		}

		for idx, skillRef := range skillsAddItems {
			match, matchErr := findSkillInRepo(repoDir, src.BasePath, skillRef)
			if matchErr != nil {
				return matchErr
			}

			destName := inferDestinationSkillName(match.Path)
			if skillsAddName != "" && idx == 0 {
				destName = strings.TrimSpace(skillsAddName)
			}
			if destName == "" {
				return fmt.Errorf("invalid destination name for skill %q", skillRef)
			}

			var destPath string
			if match.IsDir {
				destPath = filepath.Join(skillsRoot, destName)
			} else {
				destPath = filepath.Join(skillsRoot, destName+".md")
			}
			if _, statErr := os.Stat(destPath); statErr == nil {
				return fmt.Errorf("destination already exists: %s", destPath)
			} else if !os.IsNotExist(statErr) {
				return fmt.Errorf("failed to stat destination: %w", statErr)
			}

			if match.IsDir {
				if err := copyDir(match.Path, destPath); err != nil {
					return fmt.Errorf("failed to install %q: %w", skillRef, err)
				}
			} else {
				if err := copyFile(match.Path, destPath); err != nil {
					return fmt.Errorf("failed to install %q: %w", skillRef, err)
				}
			}
			fmt.Printf("Installed: %s -> %s\n", skillRef, destPath)
		}

		return nil
	},
}

type githubSkillSource struct {
	OwnerRepo string
	Ref       string
	BasePath  string
}

type skillSourceMatch struct {
	Path  string
	IsDir bool
}

var ownerRepoPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)

func parseGitHubSkillSource(raw string) (*githubSkillSource, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("source is required")
	}

	// owner/repo shorthand
	if ownerRepoPattern.MatchString(raw) {
		return &githubSkillSource{
			OwnerRepo: raw,
			Ref:       "main",
		}, nil
	}

	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return nil, fmt.Errorf("invalid source %q, expected owner/repo or GitHub URL", raw)
	}
	if !strings.EqualFold(u.Hostname(), "github.com") {
		return nil, fmt.Errorf("unsupported host %q, only github.com is supported", u.Hostname())
	}

	parts := splitPathSegments(u.Path)
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid github url: %s", raw)
	}

	owner := parts[0]
	repo := strings.TrimSuffix(parts[1], ".git")
	ownerRepo := owner + "/" + repo
	if !ownerRepoPattern.MatchString(ownerRepo) {
		return nil, fmt.Errorf("invalid owner/repo in url: %s", raw)
	}

	src := &githubSkillSource{
		OwnerRepo: ownerRepo,
		Ref:       "main",
	}

	// https://github.com/<owner>/<repo>/tree/<ref>/<path...>
	if len(parts) >= 4 && parts[2] == "tree" {
		src.Ref = parts[3]
		if len(parts) > 4 {
			src.BasePath = cleanRepoSubPath(strings.Join(parts[4:], "/"))
		}
	}
	return src, nil
}

func splitPathSegments(path string) []string {
	raw := strings.Split(path, "/")
	out := make([]string, 0, len(raw))
	for _, segment := range raw {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			continue
		}
		out = append(out, segment)
	}
	return out
}

func cleanRepoSubPath(path string) string {
	path = strings.ReplaceAll(path, "\\", "/")
	path = strings.TrimSpace(path)
	path = strings.TrimPrefix(path, "/")
	path = strings.TrimSuffix(path, "/")
	clean := filepath.Clean(path)
	if clean == "." || clean == "/" {
		return ""
	}
	if strings.HasPrefix(clean, "..") {
		return ""
	}
	return filepath.ToSlash(clean)
}

func cloneRepo(src *githubSkillSource) (string, error) {
	tmpDir, err := os.MkdirTemp("", "nanobot-skills-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}

	repoURL := fmt.Sprintf("https://github.com/%s.git", src.OwnerRepo)
	args := []string{"clone", "--depth", "1", "--branch", src.Ref, repoURL, tmpDir}
	if out, cloneErr := exec.Command("git", args...).CombinedOutput(); cloneErr == nil {
		return tmpDir, nil
	} else {
		// fallback to master when ref is implicit main
		if src.Ref == "main" {
			argsMaster := []string{"clone", "--depth", "1", "--branch", "master", repoURL, tmpDir}
			if outMaster, masterErr := exec.Command("git", argsMaster...).CombinedOutput(); masterErr == nil {
				src.Ref = "master"
				return tmpDir, nil
			} else {
				os.RemoveAll(tmpDir)
				return "", fmt.Errorf("failed to clone repository: %v\n%s\n%s", cloneErr, string(out), string(outMaster))
			}
		}
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("failed to clone repository: %v\n%s", cloneErr, string(out))
	}
}

func findSkillInRepo(repoDir, basePath, skillRef string) (*skillSourceMatch, error) {
	skillRef = strings.TrimSpace(skillRef)
	if skillRef == "" {
		return nil, fmt.Errorf("empty skill reference")
	}

	var prefixes []string
	prefixes = append(prefixes, cleanRepoSubPath(basePath))
	prefixes = append(prefixes, "")
	if basePath != "" && !strings.Contains(cleanRepoSubPath(basePath), "skills") {
		prefixes = append(prefixes, cleanRepoSubPath(filepath.ToSlash(filepath.Join(basePath, "skills"))))
	}
	prefixes = append(prefixes, "skills")
	prefixes = dedupeStrings(prefixes)

	var candidates []string
	for _, prefix := range prefixes {
		candidates = append(candidates,
			joinRepoSubPath(prefix, skillRef),
			joinRepoSubPath(prefix, skillRef+".md"),
		)
	}
	candidates = dedupeStrings(candidates)

	for _, candidate := range candidates {
		abs := safeRepoPath(repoDir, candidate)
		if abs == "" {
			continue
		}
		info, err := os.Stat(abs)
		if err != nil {
			continue
		}
		if info.IsDir() {
			if _, err := os.Stat(filepath.Join(abs, "SKILL.md")); err == nil {
				return &skillSourceMatch{Path: abs, IsDir: true}, nil
			}
			continue
		}
		if strings.HasSuffix(strings.ToLower(info.Name()), ".md") {
			return &skillSourceMatch{Path: abs, IsDir: false}, nil
		}
	}

	// fallback search by basename
	var matched *skillSourceMatch
	wantName := strings.ToLower(skillRef)
	walkErr := filepath.WalkDir(repoDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			if strings.EqualFold(d.Name(), skillRef) {
				if _, statErr := os.Stat(filepath.Join(path, "SKILL.md")); statErr == nil {
					matched = &skillSourceMatch{Path: path, IsDir: true}
					return errors.New("found")
				}
			}
			return nil
		}

		name := strings.ToLower(d.Name())
		if name == wantName+".md" {
			matched = &skillSourceMatch{Path: path, IsDir: false}
			return errors.New("found")
		}
		return nil
	})
	if walkErr != nil && walkErr.Error() != "found" {
		return nil, walkErr
	}
	if matched != nil {
		return matched, nil
	}

	return nil, fmt.Errorf("skill %q not found in %s", skillRef, repoDir)
}

func safeRepoPath(root, sub string) string {
	sub = cleanRepoSubPath(sub)
	if sub == "" {
		return root
	}
	abs := filepath.Join(root, filepath.FromSlash(sub))
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return ""
	}
	if strings.HasPrefix(rel, "..") {
		return ""
	}
	return abs
}

func dedupeStrings(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = cleanRepoSubPath(item)
		if item == "" {
			// keep one empty prefix for root path checks
			if _, ok := seen["."]; ok {
				continue
			}
			seen["."] = struct{}{}
			out = append(out, "")
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

func joinRepoSubPath(prefix, suffix string) string {
	suffix = cleanRepoSubPath(suffix)
	prefix = cleanRepoSubPath(prefix)
	if prefix == "" {
		return suffix
	}
	if suffix == "" {
		return prefix
	}
	return filepath.ToSlash(filepath.Join(prefix, suffix))
}

func inferDestinationSkillName(srcPath string) string {
	base := filepath.Base(srcPath)
	if strings.EqualFold(base, "SKILL.md") {
		return strings.TrimSpace(filepath.Base(filepath.Dir(srcPath)))
	}
	if strings.HasSuffix(strings.ToLower(base), ".md") {
		return strings.TrimSuffix(base, filepath.Ext(base))
	}
	return strings.TrimSpace(base)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return nil
}

func copyDir(srcDir, dstDir string) error {
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return err
	}

	return filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}

		dstPath := filepath.Join(dstDir, rel)
		if d.IsDir() {
			info, infoErr := d.Info()
			mode := fs.FileMode(0755)
			if infoErr == nil {
				mode = info.Mode().Perm()
			}
			return os.MkdirAll(dstPath, mode)
		}
		return copyFile(path, dstPath)
	})
}
