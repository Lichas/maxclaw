package tools

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"
)

// dangerousPatterns 危险的命令模式
var dangerousPatterns = []string{
	`rm\s+-rf\s+/`,         // 删除根目录
	`rm\s+-rf\s+/\s`,       // 删除根目录（带空格）
	`mkfs\.`,               // 格式化文件系统
	`dd\s+if=.*of=/dev/`,   // 直接写入设备
	`:(){ :|:& };:`,        // fork 炸弹
	`>\s*/dev/sda`,         // 覆盖磁盘
	`>\s*/dev/hda`,         // 覆盖磁盘
	`chmod\s+-R\s+000\s+/`, // 递归修改根目录权限
}

// isDangerousCommand 检查命令是否危险
func isDangerousCommand(command string) error {
	lowerCmd := strings.ToLower(command)

	for _, pattern := range dangerousPatterns {
		matched, err := regexp.MatchString(pattern, lowerCmd)
		if err != nil {
			continue
		}
		if matched {
			return fmt.Errorf("dangerous command detected: pattern '%s' matched", pattern)
		}
	}

	return nil
}

// ExecTool Shell 执行工具
type ExecTool struct {
	BaseTool
	WorkingDir          string
	Timeout             time.Duration
	RestrictToWorkspace bool
}

// NewExecTool 创建 Shell 执行工具
func NewExecTool(workingDir string, timeout int, restrictToWorkspace bool) *ExecTool {
	if timeout <= 0 {
		timeout = 60
	}

	tool := &ExecTool{
		BaseTool: BaseTool{
			name:        "exec",
			description: "Execute shell commands. Use for running code, managing files, or system operations. Command timeout is enforced.",
			parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{
						"type":        "string",
						"description": "The shell command to execute",
					},
					"timeout": map[string]interface{}{
						"type":        "integer",
						"description": "Timeout in seconds (optional, default: 60)",
						"minimum":     1,
						"maximum":     300,
					},
				},
				"required": []string{"command"},
			},
		},
		WorkingDir:          workingDir,
		Timeout:             time.Duration(timeout) * time.Second,
		RestrictToWorkspace: restrictToWorkspace,
	}

	return tool
}

// Execute 执行 Shell 命令
func (t *ExecTool) Execute(ctx context.Context, params map[string]interface{}) (string, error) {
	command, _ := params["command"].(string)
	if command == "" {
		return "", fmt.Errorf("command is required")
	}

	// 检查危险命令
	if err := isDangerousCommand(command); err != nil {
		return "", err
	}

	// 确定工作目录
	workDir := t.WorkingDir
	if t.RestrictToWorkspace && workDir != "" {
		workspace, err := cleanAbsPath(workDir)
		if err != nil {
			return "", fmt.Errorf("invalid workspace: %w", err)
		}

		if err := validateCommandInWorkspace(command, workspace); err != nil {
			return "", err
		}
	}
	if t.RestrictToWorkspace && workDir == "" {
		return "", fmt.Errorf("restrictToWorkspace enabled but working directory is empty")
	}

	// 确定超时时间
	timeout := t.Timeout
	if v, ok := params["timeout"].(float64); ok {
		t := int(v)
		if t > 0 {
			timeout = time.Duration(t) * time.Second
		}
	}

	// 创建带超时的 context
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 执行命令
	cmd := exec.CommandContext(execCtx, "sh", "-c", command)
	if workDir != "" {
		cmd.Dir = workDir
	}

	output, err := cmd.CombinedOutput()

	// 截断输出（限制 10KB）
	maxOutputSize := 10 * 1024
	outputStr := string(output)
	if len(outputStr) > maxOutputSize {
		outputStr = outputStr[:maxOutputSize] + "\n... (output truncated)"
	}

	if err != nil {
		if execCtx.Err() == context.DeadlineExceeded {
			return fmt.Sprintf("Command timed out after %v\nPartial output:\n%s", timeout, outputStr), nil
		}
		return fmt.Sprintf("Command failed: %v\nOutput:\n%s", err, outputStr), nil
	}

	return outputStr, nil
}

func validateCommandInWorkspace(command, workspace string) error {
	if workspace == "" {
		return fmt.Errorf("workspace is required")
	}

	// Reject directory-changing commands in restricted mode.
	// Since cmd.Dir is already set to the workspace, cd has no legitimate use.
	if hasCDCommand(command) {
		return fmt.Errorf("'cd' is not allowed in restricted workspace mode")
	}

	// Reject default-value expansions that can bypass path checks.
	if strings.Contains(command, "${") && strings.Contains(command, ":-") {
		return fmt.Errorf("default-value shell expansion is not allowed in restricted mode")
	}

	tokens := splitShellWords(command)
	for _, token := range tokens {
		if token == "" {
			continue
		}

		if isShellSeparator(token) {
			continue
		}

		// Disallow shell expansions that can hide paths
		if strings.Contains(token, "$(") || strings.Contains(token, "`") {
			return fmt.Errorf("command contains unsupported shell expansion in restricted mode")
		}

		// Handle redirection tokens like >file or <file
		trimmed := strings.TrimLeft(token, "><")
		if trimmed == "" {
			continue
		}

		// Handle key=value assignments (e.g. CONFIG=/path)
		if strings.Contains(trimmed, "=") {
			parts := strings.SplitN(trimmed, "=", 2)
			if len(parts) == 2 && looksLikePath(parts[1]) {
				if err := ensurePathWithinWorkspace(parts[1], workspace); err != nil {
					return err
				}
				continue
			}
		}

		if looksLikePath(trimmed) {
			if err := ensurePathWithinWorkspace(trimmed, workspace); err != nil {
				return err
			}
		}
	}

	return nil
}

// hasCDCommand checks whether the command contains a cd sub-command.
// It looks for 'cd' as a standalone command (not part of another word).
func hasCDCommand(command string) bool {
	// Simple token-based check: if any token is exactly "cd" after splitting.
	tokens := splitShellWords(command)
	for i, token := range tokens {
		if token == "cd" {
			// Verify it's not something like "abcde" by checking the previous separator.
			if i == 0 {
				return true
			}
			prev := tokens[i-1]
			if isShellSeparator(prev) || prev == ";" || prev == "|" || prev == "&" || prev == "&&" || prev == "||" {
				return true
			}
		}
	}
	return false
}

func looksLikePath(token string) bool {
	if token == "" {
		return false
	}
	if strings.HasPrefix(token, "~") {
		return true
	}
	if strings.Contains(token, "$HOME") || strings.Contains(token, "${HOME}") || strings.Contains(token, "$USERPROFILE") {
		return true
	}
	if strings.HasPrefix(token, ".") || strings.Contains(token, "/") || strings.Contains(token, "\\") {
		return true
	}
	return false
}

func ensurePathWithinWorkspace(pathToken, workspace string) error {
	if strings.HasPrefix(pathToken, "~") || strings.Contains(pathToken, "$HOME") || strings.Contains(pathToken, "${HOME}") || strings.Contains(pathToken, "$USERPROFILE") {
		return fmt.Errorf("path '%s' is not allowed in restricted mode", pathToken)
	}

	ws, err := cleanAbsPath(workspace)
	if err != nil {
		return fmt.Errorf("invalid workspace: %w", err)
	}

	var target string
	if filepath.IsAbs(pathToken) {
		target, err = cleanAbsPath(pathToken)
		if err != nil {
			return fmt.Errorf("invalid path '%s': %w", pathToken, err)
		}
	} else {
		target, err = cleanAbsPath(filepath.Join(ws, pathToken))
		if err != nil {
			return fmt.Errorf("invalid path '%s': %w", pathToken, err)
		}
	}

	if !isWithin(ws, target) {
		return fmt.Errorf("path '%s' is outside workspace", pathToken)
	}
	return nil
}

func isWithin(base, target string) bool {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".."
}

func cleanAbsPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved, nil
	}
	return abs, nil
}

func splitShellWords(input string) []string {
	var tokens []string
	var buf strings.Builder
	inSingle := false
	inDouble := false
	escape := false

	flush := func() {
		if buf.Len() > 0 {
			tokens = append(tokens, buf.String())
			buf.Reset()
		}
	}

	for _, r := range input {
		if escape {
			buf.WriteRune(r)
			escape = false
			continue
		}

		if r == '\\' && !inSingle {
			escape = true
			continue
		}

		if r == '\'' && !inDouble {
			inSingle = !inSingle
			continue
		}
		if r == '"' && !inSingle {
			inDouble = !inDouble
			continue
		}

		if unicode.IsSpace(r) && !inSingle && !inDouble {
			flush()
			continue
		}

		buf.WriteRune(r)
	}

	flush()
	return tokens
}

func isShellSeparator(token string) bool {
	switch token {
	case "|", "||", "&", "&&", ";":
		return true
	default:
		return false
	}
}

// Name 返回工具名称
func (t *ExecTool) Name() string {
	return t.name
}

// Description 返回工具描述
func (t *ExecTool) Description() string {
	return t.description
}

// Parameters 返回参数定义
func (t *ExecTool) Parameters() map[string]interface{} {
	return t.parameters
}
