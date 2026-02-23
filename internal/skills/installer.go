package skills

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

// 镜像源列表（按优先级排序）
var mirrorSources = []struct {
	name string
	url  string
}{
	{"GitHub", "https://github.com/%s/archive/refs/heads/%s.zip"},
	{"FastGit", "https://hub.fastgit.xyz/%s/archive/refs/heads/%s.zip"},
	{"GhProxy", "https://ghproxy.com/https://github.com/%s/archive/refs/heads/%s.zip"},
	{"GhProxy-CN", "https://ghproxy.cn/https://github.com/%s/archive/refs/heads/%s.zip"},
	{"Moeyy", "https://github.moeyy.xyz/https://github.com/%s/archive/refs/heads/%s.zip"},
}

// Installer 负责安装官方 skills
type Installer struct {
	workspace  string
	httpClient *http.Client
}

// NewInstaller 创建 skills 安装器
func NewInstaller(workspace string) *Installer {
	return &Installer{
		workspace: workspace,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment, // 支持系统代理
			},
		},
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

// InstallOfficialSkills 从 GitHub 或镜像下载并安装官方 skills
// 支持自动 fallback 到可用镜像
func (i *Installer) InstallOfficialSkills() error {
	skillsDir := filepath.Join(i.workspace, "skills")

	// 确保 skills 目录存在
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		return fmt.Errorf("failed to create skills directory: %w", err)
	}

	// 尝试从多个源下载
	zipPath := filepath.Join(i.workspace, ".tmp_skills.zip")
	defer os.Remove(zipPath)

	var lastErr error
	for _, source := range mirrorSources {
		zipURL := fmt.Sprintf(source.url, AnthricSkillsRepo, DefaultSkillsBranch)
		fmt.Printf("📦 Trying %s...\n", source.name)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		err := i.downloadFileWithContext(ctx, zipURL, zipPath)
		cancel()

		if err == nil {
			fmt.Printf("  ✓ Downloaded from %s\n", source.name)
			break
		}

		lastErr = err
		// 检查是否是网络连接问题
		if isNetworkError(err) {
			fmt.Printf("  ✗ %s unavailable, trying next mirror...\n", source.name)
			continue
		}
		// 其他错误直接返回
		return fmt.Errorf("download failed from %s: %w", source.name, err)
	}

	// 检查是否下载成功
	if _, err := os.Stat(zipPath); os.IsNotExist(err) {
		return &NetworkError{
			Message: "failed to download skills from all mirrors",
			Cause:   lastErr,
		}
	}

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

// NetworkError 网络错误类型
type NetworkError struct {
	Message string
	Cause   error
}

func (e *NetworkError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

func (e *NetworkError) Unwrap() error {
	return e.Cause
}

// IsNetworkError 检查是否是网络错误
func isNetworkError(err error) bool {
	if err == nil {
		return false
	}

	// 检查 URL 错误
	if urlErr, ok := err.(*url.Error); ok {
		// 超时或临时错误
		if urlErr.Timeout() || urlErr.Temporary() {
			return true
		}
	}

	// 检查错误消息
	errStr := err.Error()
	networkKeywords := []string{
		"connection refused",
		"no such host",
		"timeout",
		"i/o timeout",
		"temporary failure",
		"connection reset",
		"EOF",
	}

	for _, keyword := range networkKeywords {
		if strings.Contains(errStr, keyword) {
			return true
		}
	}

	return false
}

// downloadFileWithContext 带上下文的文件下载
func (i *Installer) downloadFileWithContext(ctx context.Context, url, filepath string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	// 设置请求头，模拟浏览器
	req.Header.Set("User-Agent", "maxclaw-skills-installer/1.0")

	resp, err := i.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	// 创建目标文件
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

	var skillsList []string
	for _, entry := range entries {
		name := entry.Name()
		if name == "README.md" || name == SkillsInstallMarker || strings.HasPrefix(name, ".") {
			continue
		}

		// 检查是否是有效的 skill（包含 SKILL.md 或 .md 文件）
		if entry.IsDir() {
			skillFile := filepath.Join(skillsDir, name, "SKILL.md")
			if _, err := os.Stat(skillFile); err == nil {
				skillsList = append(skillsList, name)
				continue
			}
			// 也检查目录下是否有 .md 文件
			if hasMarkdownFiles(filepath.Join(skillsDir, name)) {
				skillsList = append(skillsList, name)
			}
		} else if strings.HasSuffix(name, ".md") {
			skillName := strings.TrimSuffix(name, ".md")
			skillsList = append(skillsList, skillName)
		}
	}

	return skillsList, nil
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

// GetInstallHelpMessage 获取安装帮助信息（网络失败时显示）
func GetInstallHelpMessage() string {
	return `
Skills installation failed due to network issues.

Options:
  1. Configure proxy and retry:
     export HTTPS_PROXY=http://127.0.0.1:7890
     maxclaw skills install --official

  2. Manual download:
     - Download: https://github.com/anthropics/skills/archive/refs/heads/main.zip
     - Extract the 'skills' folder to: ~/.maxclaw/workspace/skills/

  3. Use a mirror:
     The installer already tried multiple mirrors (FastGit, GhProxy, etc.)
     If all failed, you may need a system-wide VPN/proxy.

  4. Skip for now:
     maxclaw works without official skills. You can add your own skills
     to ~/.maxclaw/workspace/skills/ later.
`
}
