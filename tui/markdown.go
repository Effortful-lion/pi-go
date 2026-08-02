//go:build !windows

package tui

import (
	"regexp"
	"strings"
)

// 代码块标记正则
var codeBlockRe = regexp.MustCompile("^```(\\w*)")

// RenderMarkdown 将 Markdown 文本渲染为 ANSI 终端样式文本。
// 支持: 标题、粗体、斜体、行内代码、代码块、无序列表、引用、分隔线。
func RenderMarkdown(text string) string {
	lines := strings.Split(text, "\n")

	// 状态
	inCodeBlock := false
	codeLang := ""
	var codeLines []string
	var result []string

	flushCodeBlock := func() {
		if len(codeLines) > 0 {
			code := strings.Join(codeLines, "\n")
			if codeLang != "" {
				code = Highlight(code, codeLang)
			}
			result = append(result, code)
			codeLines = nil
			codeLang = ""
		}
	}

	for _, line := range lines {
		// 代码块
		if m := codeBlockRe.FindStringSubmatch(line); m != nil {
			if inCodeBlock {
				flushCodeBlock()
				inCodeBlock = false
			} else {
				inCodeBlock = true
				codeLang = m[1]
				codeLines = nil
			}
			continue
		}

		if inCodeBlock {
			codeLines = append(codeLines, line)
			continue
		}

		// 内联样式的行级处理
		result = append(result, renderInline(line))
	}

	// 未关闭的代码块
	flushCodeBlock()

	return strings.Join(result, "\n")
}

// renderInline 处理一行内的内联样式。
func renderInline(line string) string {
	trimmed := strings.TrimSpace(line)

	// 分隔线
	if trimmed == "---" || trimmed == "***" || trimmed == "___" {
		return Dim(strings.Repeat("─", 80))
	}

	// 标题
	if strings.HasPrefix(trimmed, "#### ") {
		return Bold(strings.TrimPrefix(trimmed, "#### "))
	}
	if strings.HasPrefix(trimmed, "### ") {
		return Bold(strings.TrimPrefix(trimmed, "### "))
	}
	if strings.HasPrefix(trimmed, "## ") {
		return Bold(strings.TrimPrefix(trimmed, "## "))
	}
	if strings.HasPrefix(trimmed, "# ") {
		return Bold(strings.TrimPrefix(trimmed, "# "))
	}

	// 无序列表
	if isListItem(trimmed) {
		indent := ""
		for _, ch := range line {
			if ch == ' ' {
				indent += " "
			} else {
				break
			}
		}
		content := strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(trimmed, "- "), "* "), "+ ")
		bullet := "•"
		if len(line) > len(trimmed) {
			return indent + bullet + " " + formatBoldItalicCode(content)
		}
		return Green(bullet) + " " + formatBoldItalicCode(content)
	}

	// 引用
	if strings.HasPrefix(trimmed, "> ") {
		content := strings.TrimPrefix(trimmed, "> ")
		return Dim("│ ") + formatBoldItalicCode(content)
	}

	// 表格分隔行
	if isTableSeparator(trimmed) {
		return Dim(trimmed)
	}

	return formatBoldItalicCode(line)
}

// formatBoldItalicCode 处理行内 **粗体**、*斜体*、`代码`。
func formatBoldItalicCode(text string) string {
	// 先处理行内代码
	text = replaceInlineCode(text)
	// 再处理粗体
	text = replaceBold(text)
	// 最后处理斜体（避免与粗体混淆）
	text = replaceItalic(text)
	return text
}

// replaceInlineCode 替换 `code` 为 CYAN + 反引号保留。
func replaceInlineCode(text string) string {
	var b strings.Builder
	b.Grow(len(text) * 2)
	i := 0
	for i < len(text) {
		if text[i] == '`' {
			j := i + 1
			for j < len(text) && text[j] != '`' {
				j++
			}
			if j < len(text) {
				b.WriteString(Cyan(text[i : j+1]))
				i = j + 1
				continue
			}
		}
		b.WriteByte(text[i])
		i++
	}
	return b.String()
}

// replaceBold 替换 **text** 为粗体。
func replaceBold(text string) string {
	var b strings.Builder
	b.Grow(len(text) * 2)
	i := 0
	for i < len(text) {
		if i+1 < len(text) && text[i] == '*' && text[i+1] == '*' {
			j := i + 2
			for j+1 < len(text) && !(text[j] == '*' && text[j+1] == '*') {
				j++
			}
			if j+1 < len(text) {
				b.WriteString(Bold(text[i+2 : j]))
				i = j + 2
				continue
			}
		}
		b.WriteByte(text[i])
		i++
	}
	return b.String()
}

// replaceItalic 替换 *text* 为斜体（但不匹配 **）。
func replaceItalic(text string) string {
	var b strings.Builder
	b.Grow(len(text) * 2)
	i := 0
	for i < len(text) {
		if text[i] == '*' && (i == 0 || text[i-1] != '*') && (i+1 >= len(text) || text[i+1] != '*') {
			j := i + 1
			for j < len(text) && text[j] != '*' {
				j++
			}
			if j < len(text) && (j+1 >= len(text) || text[j+1] != '*') {
				b.WriteString(Italic(text[i+1 : j]))
				i = j + 1
				continue
			}
		}
		b.WriteByte(text[i])
		i++
	}
	return b.String()
}

// isListItem 判断是否是列表项。
func isListItem(line string) bool {
	return strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") || strings.HasPrefix(line, "+ ")
}

// isTableSeparator 判断是否是表格分隔行。
func isTableSeparator(line string) bool {
	if len(line) < 3 {
		return false
	}
	s := strings.TrimSpace(line)
	return strings.HasPrefix(s, "|") && strings.Contains(s, "---")
}

// MarkdownLineRenderer 是有状态的逐行 Markdown 渲染器。
// 用于流式实时渲染场景，能正确跟踪跨行的 ``` 代码块状态。
// 零值可用，通过 NewMarkdownLineRenderer() 创建。
type MarkdownLineRenderer struct {
	inCodeBlock bool
	codeLang    string
	codeLines   []string
}

// NewMarkdownLineRenderer 创建新的逐行 Markdown 渲染器。
func NewMarkdownLineRenderer() *MarkdownLineRenderer {
	return &MarkdownLineRenderer{}
}

// RenderLine 渲染一行 Markdown。
// 当检测到代码块边界 ``` 时切换状态；
// 处于代码块内部时，行不经过 Markdown 块级渲染，直接保留原样；
// 代码块结束时，一次性用 Highlight 进行语法高亮。
func (r *MarkdownLineRenderer) RenderLine(line string) string {
	if m := codeBlockRe.FindStringSubmatch(line); m != nil {
		if r.inCodeBlock {
			// 代码块结束：flush 累积的代码行并高亮
			rendered := r.flushCodeBlock()
			r.inCodeBlock = false
			return rendered
		}
		// 代码块开始
		r.inCodeBlock = true
		r.codeLang = m[1]
		r.codeLines = nil
		return Dim(line)
	}

	if r.inCodeBlock {
		r.codeLines = append(r.codeLines, line)
		return line
	}

	return renderInline(line)
}

// flushCodeBlock 将累积的代码行用 Highlight 渲染并返回。
func (r *MarkdownLineRenderer) flushCodeBlock() string {
	if len(r.codeLines) == 0 {
		return ""
	}
	code := strings.Join(r.codeLines, "\n")
	if r.codeLang != "" {
		code = Highlight(code, r.codeLang)
	}
	r.codeLines = nil
	r.codeLang = ""
	return code
}

// Reset 重置渲染器状态（用于新的对话轮次）。
func (r *MarkdownLineRenderer) Reset() {
	r.inCodeBlock = false
	r.codeLang = ""
	r.codeLines = nil
}

// MarkdownLine 将一行 Markdown 渲染为 ANSI（无状态版本，向后兼容）。
// 已废弃：新代码应使用 MarkdownLineRenderer.RenderLine() 以获得正确的代码块状态跟踪。
// 用于不需要状态跟踪的逐行渲染场景。
func MarkdownLine(line string) string {
	if codeBlockRe.MatchString(line) {
		return Dim(line)
	}
	return renderInline(line)
}

// EscapeMarkdown 将文本中的 escape 序列还原为普通文本，用于需要纯文本输出时不干扰终端。
func EscapeMarkdown(text string) string {
	// 移除所有 ANSI escape 序列
	re := regexp.MustCompile(`\033\[[0-9;]*m`)
	return re.ReplaceAllString(text, "")
}

// PadRight 右侧填充到指定宽度（用于表格渲染）。
func PadRight(s string, width int) string {
	// 计算去除 ANSI escape 后的视觉宽度
	visualLen := visualWidth(s)
	if visualLen >= width {
		return s
	}
	return s + strings.Repeat(" ", width-visualLen)
}

// visualWidth 计算去除 ANSI escape 序列后的显示宽度。
func visualWidth(s string) int {
	re := regexp.MustCompile(`\033\[[0-9;]*m`)
	cleaned := re.ReplaceAllString(s, "")
	return len(cleaned)
}

// RenderSimple 最简 Markdown 渲染：仅处理代码块高亮。
// 用于不需要完整 Markdown 支持但想要代码高亮的场景。
func RenderSimple(text string) string {
	lines := strings.Split(text, "\n")
	inCode := false
	codeLang := ""
	var codeLines []string
	var out []string

	for _, line := range lines {
		if m := codeBlockRe.FindStringSubmatch(line); m != nil {
			if inCode {
				code := strings.Join(codeLines, "\n")
				if codeLang != "" {
					code = Highlight(code, codeLang)
				}
				out = append(out, code)
				codeLines = nil
				codeLang = ""
				inCode = false
			} else {
				inCode = true
				codeLang = m[1]
			}
			continue
		}
		if inCode {
			codeLines = append(codeLines, line)
		} else {
			out = append(out, line)
		}
	}

	if len(codeLines) > 0 {
		code := strings.Join(codeLines, "\n")
		if codeLang != "" {
			code = Highlight(code, codeLang)
		}
		out = append(out, code)
	}

	return strings.Join(out, "\n")
}


