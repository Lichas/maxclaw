package skills

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// AnthricSkillsRepo 官方 skills 仓库
	AnthricSkillsRepo = "anthropics/skills"
	// DefaultSkillsBranch 默认分支
	DefaultSkillsBranch = "main"
	// SkillsInstallMarker 安装标记文件
	SkillsInstallMarker = ".official_skills_installed"
)

// Installer 负责安装官方 skills
type Installer struct {
	workspace   string
	httpClient  *http.Client
}

// NewInstaller 创建 skills 安装器
func NewInstaller(workspace string) *Installer {
	return &Installer{
		workspace:  workspace,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// IsFirstRun 检查是否是首次运行（skills 目录为空或没有官方 skills）
func (i *Installer) IsFirstRun() bool {
	skillsDir := filepath.Join(i.workspace, "skills")

	// 检查标记文件
	markerPath := filepath.Join(skillsDir, SkillsInstallMarker)
	if _, err := os.Stat(markerPath); err == nil {
		return false
	}

	// 检查 skills 目录是否存在且有内容
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		// 目录不存在，需要安装
		return true
	}

	// 如果 skills 目录为空或只有 README.md，视为首次运行
	for _, entry := range entries {
		name := entry.Name()
		if name != "README.md" && !strings.HasPrefix(name, ".") {
			// 有非 README 文件，说明用户已添加自己的 skills
			return false
		}
	}

	return true
}

// InstallOfficialSkills 从 GitHub 下载并安装官方 skills
func (i *Installer) InstallOfficialSkills() error {
	skillsDir := filepath.Join(i.workspace, "skills")

	// 确保 skills 目录存在
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		return fmt.Errorf("failed to create skills directory: %w", err)
	}

	// 下载官方 skills
	fmt.Println("📦 Downloading official skills from anthropics/skills...")

	zipURL := fmt.Sprintf("https://github.com/%s/archive/refs/heads/%s.zip", AnthricSkillsRepo, DefaultSkillsBranch)
	zipPath := filepath.Join(i.workspace, ".tmp_skills.zip")

	// 下载 zip 文件
	if err := i.downloadFile(zipURL, zipPath); err != nil {
		return fmt.Errorf("failed to download skills: %w", err)
	}
	defer os.Remove(zipPath)

	// 解压并安装
	if err := i.extractSkills(zipPath, skillsDir); err != nil {
		return fmt.Errorf("failed to extract skills: %w", err)
	}

	// 创建安装标记
	markerPath := filepath.Join(skillsDir, SkillsInstallMarker)
	markerContent := fmt.Sprintf("Official skills installed at: %s\nSource: https://github.com/%s\n",
		time.Now().Format(time.RFC3339), AnthricSkillsRepo)
	if err := os.WriteFile(markerPath, []byte(markerContent), 0644); err != nil {
		return fmt.Errorf("failed to create install marker: %w", err)
	}

	fmt.Println("✓ Official skills installed successfully!")
	return nil
}

// downloadFile 下载文件到指定路径
func (i *Installer) downloadFile(url, filepath string) error {
	resp, err := i.httpClient.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	// 创建临时目录
	if err := os.MkdirAll(os.TempDir(), 0755); err != nil {
		return err
	}

	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

// extractSkills 解压 zip 文件中的 skills 到目标目录
func (i *Installer) extractSkills(zipPath, targetDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	// 找到 skills 目录的前缀
	skillsPrefix := ""
	for _, f := range r.File {
		if strings.Contains(f.Name, "/skills/") {
			parts := strings.Split(f.Name, "/")
			for i, part := range parts {
				if part == "skills" && i > 0 {
					skillsPrefix = strings.Join(parts[:i+1], "/") + "/"
					break
				}
			}
			break
		}
	}

	if skillsPrefix == "" {
		return fmt.Errorf("could not find skills directory in archive")
	}

	installedCount := 0
	for _, f := range r.File {
		// 只处理 skills 目录下的文件
		if !strings.HasPrefix(f.Name, skillsPrefix) {
			continue
		}

		// 跳过根目录和特殊文件
		relPath := strings.TrimPrefix(f.Name, skillsPrefix)
		if relPath == "" || strings.HasPrefix(relPath, ".") {
			continue
		}

		targetPath := filepath.Join(targetDir, relPath)

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, f.Mode()); err != nil {
				return err
			}
			continue
		}

		// 创建文件
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}

		out, err := os.Create(targetPath)
		if err != nil {
			rc.Close()
			return err
		}

		_, err = io.Copy(out, rc)
		out.Close()
		rc.Close()

		if err != nil {
			return err
		}

		installedCount++
	}

	fmt.Printf("  Installed %d skill files\n", installedCount)
	return nil
}

// InstallIfNeeded 如果需要则安装官方 skills（用于自动检测）
func (i *Installer) InstallIfNeeded() error {
	if !i.IsFirstRun() {
		return nil
	}

	return i.InstallOfficialSkills()
}

// ListInstalledSkills 列出已安装的官方 skills
func (i *Installer) ListInstalledSkills() ([]string, error) {
	skillsDir := filepath.Join(i.workspace, "skills")

	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return nil, err
	}

	var skills []string
	for _, entry := range entries {
		name := entry.Name()
		if name == "README.md" || name == SkillsInstallMarker || strings.HasPrefix(name, ".") {
			continue
		}

		// 检查是否是有效的 skill（包含 SKILL.md 或 .md 文件）
		if entry.IsDir() {
			skillFile := filepath.Join(skillsDir, name, "SKILL.md")
			if _, err := os.Stat(skillFile); err == nil {
				skills = append(skills, name)
				continue
			}
			// 也检查目录下是否有 .md 文件
			if hasMarkdownFiles(filepath.Join(skillsDir, name)) {
				skills = append(skills, name)
			}
		} else if strings.HasSuffix(name, ".md") {
			skillName := strings.TrimSuffix(name, ".md")
			skills = append(skills, skillName)
		}
	}

	return skills, nil
}

// hasMarkdownFiles 检查目录下是否有 markdown 文件
func hasMarkdownFiles(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			return true
		}
	}
	return false
}
