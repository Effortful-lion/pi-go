// Package builtin 提供 Agent 内置工具：文件操作、Shell 执行等。
package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Effortful-lion/pi-go/ai"
	"github.com/Effortful-lion/pi-go/tool"
)

const defaultMaxReadSize = 1 << 20 // 1MB

// FileConfig 文件工具安全配置。
type FileConfig struct {
	AllowedRoots []string // 允许访问的根目录，空=不限制
	MaxReadSize  int64    // 最大读取字节数，≤0 使用 defaultMaxReadSize
}

// maxReadSize 返回有效的最大读取大小。
func (c FileConfig) maxReadSize() int64 {
	if c.MaxReadSize <= 0 {
		return defaultMaxReadSize
	}
	return c.MaxReadSize
}

// readFileTool 读取文件内容。
type readFileTool struct {
	cfg FileConfig
}

// writeFileTool 写入文件。
type writeFileTool struct {
	cfg FileConfig
}

// listDirTool 列出目录内容。
type listDirTool struct {
	cfg FileConfig
}

// searchFileTool 根据文件名模式搜索文件。
type searchFileTool struct {
	cfg FileConfig
}

// NewFileTools 返回 4 个文件操作工具。
func NewFileTools(cfg FileConfig) []tool.Tool {
	return []tool.Tool{
		&readFileTool{cfg: cfg},
		&writeFileTool{cfg: cfg},
		&listDirTool{cfg: cfg},
		&searchFileTool{cfg: cfg},
	}
}

// --- read_file ---

func (t *readFileTool) Name() string { return "read_file" }
func (t *readFileTool) Definition() ai.ToolDefinition {
	return ai.ToolDefinition{
		Name:        "read_file",
		Description: "读取指定文件的内容，支持分页（offset/limit）。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":   map[string]any{"type": "string", "description": "文件路径"},
				"offset": map[string]any{"type": "integer", "description": "起始行号（0-based），默认 0"},
				"limit":  map[string]any{"type": "integer", "description": "最大行数，默认读取全部"},
			},
			"required": []string{"path"},
		},
	}
}
func (t *readFileTool) Execute(ctx context.Context, args string) (string, error) {
	var p struct {
		Path   string `json:"path"`
		Offset int    `json:"offset"`
		Limit  int    `json:"limit"`
	}
	if err := json.Unmarshal([]byte(args), &p); err != nil {
		return "", fmt.Errorf("read_file: invalid args: %w", err)
	}
	abs, err := resolvePath(p.Path)
	if err != nil {
		return "", err
	}
	if err := checkAllowed(t.cfg.AllowedRoots, abs); err != nil {
		return "", err
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return "", fmt.Errorf("read_file: %w", err)
	}
	return formatLines(data, p.Offset, p.Limit, t.cfg.maxReadSize()), nil
}

// --- write_file ---

func (t *writeFileTool) Name() string { return "write_file" }
func (t *writeFileTool) Definition() ai.ToolDefinition {
	return ai.ToolDefinition{
		Name:        "write_file",
		Description: "创建或覆盖写入文件内容。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":    map[string]any{"type": "string", "description": "文件路径"},
				"content": map[string]any{"type": "string", "description": "要写入的文本内容"},
			},
			"required": []string{"path", "content"},
		},
	}
}
func (t *writeFileTool) Execute(ctx context.Context, args string) (string, error) {
	var p struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(args), &p); err != nil {
		return "", fmt.Errorf("write_file: invalid args: %w", err)
	}
	abs, err := resolvePath(p.Path)
	if err != nil {
		return "", err
	}
	if err := checkAllowed(t.cfg.AllowedRoots, abs); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0755); err != nil {
		return "", fmt.Errorf("write_file: create parent dirs: %w", err)
	}
	if err := os.WriteFile(abs, []byte(p.Content), 0644); err != nil {
		return "", fmt.Errorf("write_file: %w", err)
	}
	return fmt.Sprintf("wrote %d bytes to %s", len(p.Content), abs), nil
}

// --- list_dir ---

func (t *listDirTool) Name() string { return "list_dir" }
func (t *listDirTool) Definition() ai.ToolDefinition {
	return ai.ToolDefinition{
		Name:        "list_dir",
		Description: "列出指定目录下的文件和子目录。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{"type": "string", "description": "目录路径，默认为当前目录"},
			},
			"required": []string{},
		},
	}
}
func (t *listDirTool) Execute(ctx context.Context, args string) (string, error) {
	var p struct {
		Path string `json:"path"`
	}
	_ = json.Unmarshal([]byte(args), &p) // 允许 path 缺失

	base := p.Path
	if base == "" {
		base = "."
	}
	abs, err := resolvePath(base)
	if err != nil {
		return "", err
	}
	if err := checkAllowed(t.cfg.AllowedRoots, abs); err != nil {
		return "", err
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		return "", fmt.Errorf("list_dir: %w", err)
	}
	var b strings.Builder
	for _, e := range entries {
		prefix := "  "
		if e.IsDir() {
			prefix = "d "
		}
		b.WriteString(prefix)
		b.WriteString(e.Name())
		b.WriteByte('\n')
	}
	return b.String(), nil
}

// --- search_file ---

func (t *searchFileTool) Name() string { return "search_file" }
func (t *searchFileTool) Definition() ai.ToolDefinition {
	return ai.ToolDefinition{
		Name:        "search_file",
		Description: "按文件名模式递归搜索文件（支持通配符 *）。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pattern":   map[string]any{"type": "string", "description": "文件名匹配模式，如 *.go"},
				"recursive": map[string]any{"type": "boolean", "description": "是否递归搜索子目录，默认 true"},
				"path":      map[string]any{"type": "string", "description": "搜索起始目录，默认为当前目录"},
			},
			"required": []string{"pattern"},
		},
	}
}
func (t *searchFileTool) Execute(ctx context.Context, args string) (string, error) {
	var p struct {
		Pattern   string `json:"pattern"`
		Recursive *bool  `json:"recursive"`
		Path      string `json:"path"`
	}
	if err := json.Unmarshal([]byte(args), &p); err != nil {
		return "", fmt.Errorf("search_file: invalid args: %w", err)
	}
	base := p.Path
	if base == "" {
		base = "."
	}
	abs, err := resolvePath(base)
	if err != nil {
		return "", err
	}
	if err := checkAllowed(t.cfg.AllowedRoots, abs); err != nil {
		return "", err
	}

	recursive := true
	if p.Recursive != nil {
		recursive = *p.Recursive
	}

	var results []string
	if recursive {
		err = filepath.Walk(abs, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // 跳过无权限的目录
			}
			if info.IsDir() {
				return nil
			}
			if match, _ := filepath.Match(p.Pattern, info.Name()); match {
				results = append(results, path)
			}
			return nil
		})
	} else {
		entries, readErr := os.ReadDir(abs)
		if readErr != nil {
			return "", fmt.Errorf("search_file: %w", readErr)
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			if match, _ := filepath.Match(p.Pattern, e.Name()); match {
				results = append(results, filepath.Join(abs, e.Name()))
			}
		}
	}
	if err != nil {
		return "", fmt.Errorf("search_file: %w", err)
	}
	if len(results) == 0 {
		return "no matches", nil
	}
	return strings.Join(results, "\n"), nil
}

// --- 共享工具函数 ---

// resolvePath 将相对路径转为绝对路径。
func resolvePath(raw string) (string, error) {
	abs, err := filepath.Abs(raw)
	if err != nil {
		return "", fmt.Errorf("resolve path %q: %w", raw, err)
	}
	// 解析符号链接
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		// 文件可能不存在（如 write_file 新建），使用原始绝对路径
		return abs, nil
	}
	return resolved, nil
}

// checkAllowed 检查目标路径是否在允许的根目录范围内。
func checkAllowed(roots []string, target string) error {
	if len(roots) == 0 {
		return nil // 不限制
	}
	for _, root := range roots {
		absRoot, _ := filepath.Abs(root)
		if strings.HasPrefix(target, absRoot+string(os.PathSeparator)) || target == absRoot {
			return nil
		}
	}
	return fmt.Errorf("access denied: path %q is outside allowed roots", target)
}

// formatLines 读取文件并按行格式化输出（带行号、分页）。
func formatLines(data []byte, offset, limit int, maxSize int64) string {
	if int64(len(data)) > maxSize {
		data = data[:maxSize]
	}
	lines := strings.Split(string(data), "\n")
	if offset >= len(lines) {
		return ""
	}
	lines = lines[offset:]
	if limit > 0 && limit < len(lines) {
		lines = lines[:limit]
	}
	var b strings.Builder
	for i, line := range lines {
		fmt.Fprintf(&b, "%6d: %s\n", offset+i+1, line)
	}
	return b.String()
}
