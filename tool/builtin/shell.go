package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/Effortful-lion/pi-go/ai"
	"github.com/Effortful-lion/pi-go/tool"
)

const defaultCmdTimeout = 30 * time.Second

// 默认的危险命令黑名单。
var defaultBlockedPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\brm\s+-rf\b`),
	regexp.MustCompile(`(?i)\bsudo\b`),
	regexp.MustCompile(`(?i)\bchmod\s+777\b`),
	regexp.MustCompile(`(?i)\bcurl\s+.*\|\s*sh\b`),
	regexp.MustCompile(`(?i)\bwget\s+.*\|.*sh\b`),
}

// ShellConfig Shell 工具配置。
type ShellConfig struct {
	AllowedCommands []string // 允许的命令白名单，空=允许所有
	BlockedPatterns []*regexp.Regexp // 危险模式黑名单，nil/空=使用默认黑名单
	CmdTimeout      time.Duration   // 命令超时，≤0 使用默认值 30s
}

// effectiveTimeout 返回有效超时值。
func (c ShellConfig) effectiveTimeout() time.Duration {
	if c.CmdTimeout <= 0 {
		return defaultCmdTimeout
	}
	return c.CmdTimeout
}

// effectiveBlocked 返回有效黑名单。
func (c ShellConfig) effectiveBlocked() []*regexp.Regexp {
	if len(c.BlockedPatterns) > 0 {
		return c.BlockedPatterns
	}
	return defaultBlockedPatterns
}

// shellTool 执行 Shell 命令。
type shellTool struct {
	cfg ShellConfig
}

// NewShellTool 创建 Shell 工具。
func NewShellTool(cfg ShellConfig) tool.Tool {
	return &shellTool{cfg: cfg}
}

func (t *shellTool) Name() string { return "shell" }

func (t *shellTool) Definition() ai.ToolDefinition {
	return ai.ToolDefinition{
		Name: "shell",
		Description: `在当前工作目录下执行 Shell 命令，返回 stdout 和 stderr 组合输出。
命令会在新的子进程中执行，工作目录通过 cwd 参数指定。
命令执行有超时限制，超时后会被终止。`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command": map[string]any{"type": "string", "description": "要执行的 Shell 命令"},
				"cwd":     map[string]any{"type": "string", "description": "命令执行的工作目录（可选）"},
			},
			"required": []string{"command"},
		},
	}
}

func (t *shellTool) Execute(ctx context.Context, args string) (string, error) {
	var p struct {
		Command string `json:"command"`
		CWD     string `json:"cwd"`
	}
	if err := json.Unmarshal([]byte(args), &p); err != nil {
		return "", fmt.Errorf("shell: invalid args: %w", err)
	}
	if p.Command == "" {
		return "", fmt.Errorf("shell: command is required")
	}

	// 安全检查
	trimmed := strings.TrimSpace(p.Command)
	if err := t.checkCommand(trimmed); err != nil {
		return "", err
	}

	timeout := t.cfg.effectiveTimeout()
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(execCtx, "sh", "-c", trimmed)
	if p.CWD != "" {
		cmd.Dir = p.CWD
	}

	output, err := cmd.CombinedOutput()
	if execCtx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("shell: command timed out after %v", timeout)
	}
	if err != nil {
		return fmt.Sprintf("exit code: %d\n%s", cmd.ProcessState.ExitCode(), string(output)), nil
	}
	return string(output), nil
}

// checkCommand 执行安全检查。
func (t *shellTool) checkCommand(cmd string) error {
	// 白名单检查
	allowed := t.cfg.AllowedCommands
	if len(allowed) > 0 {
		ok := false
		for _, a := range allowed {
			if strings.HasPrefix(cmd, a+" ") || cmd == a {
				ok = true
				break
			}
		}
		if !ok {
			return fmt.Errorf("shell: command %q is not in the allowed list", shellPreview(cmd))
		}
	}

	// 黑名单检查
	for _, p := range t.cfg.effectiveBlocked() {
		if p.MatchString(cmd) {
			return fmt.Errorf("shell: dangerous command blocked: %q", shellPreview(cmd))
		}
	}

	return nil
}

// shellPreview 截取命令的前 80 字符用于安全提示。
func shellPreview(cmd string) string {
	if len(cmd) > 80 {
		return cmd[:80] + "..."
	}
	return cmd
}
