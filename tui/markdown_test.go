//go:build !windows

package tui

import (
	"strings"
	"testing"
)

func TestMarkdownLineRenderer_PlainText(t *testing.T) {
	r := NewMarkdownLineRenderer()
	result := r.RenderLine("Hello World")
	if result != "Hello World" {
		t.Errorf("expected plain text, got %q", result)
	}
}

func TestMarkdownLineRenderer_Heading(t *testing.T) {
	r := NewMarkdownLineRenderer()
	result := r.RenderLine("# Title")
	if !strings.Contains(result, "\033[1m") {
		t.Error("heading should be bold")
	}
	if !strings.Contains(result, "Title") {
		t.Error("heading should contain text")
	}
}

func TestMarkdownLineRenderer_InlineCode(t *testing.T) {
	r := NewMarkdownLineRenderer()
	result := r.RenderLine("use `fmt.Println()` to print")
	if !strings.Contains(result, "\033[36m") {
		t.Error("inline code should be cyan")
	}
}

func TestMarkdownLineRenderer_CodeBlockStateMachine(t *testing.T) {
	r := NewMarkdownLineRenderer()

	// 代码块开始
	result := r.RenderLine("```go")
	if !strings.Contains(result, "\033[2m") {
		t.Error("code block start should be dimmed")
	}
	if !r.inCodeBlock {
		t.Error("should be in code block after opening fence")
	}
	if r.codeLang != "go" {
		t.Errorf("code language should be 'go', got %q", r.codeLang)
	}

	// 代码块内部：不应被 Markdown 渲染
	result = r.RenderLine("func main() {")
	if result != "func main() {" {
		t.Errorf("code inside block should be raw, got %q", result)
	}

	// 代码块内部的 # 注释不应被渲染为标题
	result = r.RenderLine("# this is a comment, not a heading")
	if strings.Contains(result, "\033[1m") {
		t.Error("code inside block should not be rendered as heading")
	}

	// 代码块内部的行内代码也不应被渲染
	result = r.RenderLine(`fmt.Println("hello")`)
	if strings.Contains(result, "\033[36m") {
		t.Error("code inside block should not render inline code")
	}

	// 代码块结束：应该输出高亮后的代码
	result = r.RenderLine("```")
	if r.inCodeBlock {
		t.Error("should not be in code block after closing fence")
	}
	// 高亮后的代码包含 ANSI 转义码，验证包含关键字和注释
	if !strings.Contains(StripANSI(result), "func main() {") {
		t.Error("highlighted code should contain code lines")
	}
	if !strings.Contains(StripANSI(result), "this is a comment") {
		t.Error("highlighted code should contain comment line")
	}
}

func TestMarkdownLineRenderer_CodeBlockWithoutLang(t *testing.T) {
	r := NewMarkdownLineRenderer()

	r.RenderLine("```")
	if !r.inCodeBlock {
		t.Error("should be in code block after fence without lang")
	}
	if r.codeLang != "" {
		t.Errorf("code language should be empty, got %q", r.codeLang)
	}

	r.RenderLine("plain code line")
	result := r.RenderLine("```")
	if r.inCodeBlock {
		t.Error("should exit code block")
	}
	// 无语言标识时，代码不经过 Highlight，直接原样输出
	if !strings.Contains(result, "plain code line") {
		t.Error("code block content should be in output")
	}
}

func TestMarkdownLineRenderer_NestedFence(t *testing.T) {
	// 连续两个代码块
	r := NewMarkdownLineRenderer()

	r.RenderLine("```python")
	r.RenderLine("print('hello')")
	r.RenderLine("```")

	if r.inCodeBlock {
		t.Error("should exit first code block")
	}

	// 第二个代码块
	r.RenderLine("```go")
	if !r.inCodeBlock {
		t.Error("should enter second code block")
	}
	r.RenderLine("package main")
	r.RenderLine("```")

	if r.inCodeBlock {
		t.Error("should exit second code block")
	}
}

func TestMarkdownLineRenderer_UnclosedCodeBlock(t *testing.T) {
	r := NewMarkdownLineRenderer()

	r.RenderLine("```go")
	r.RenderLine("func main() {")
	r.RenderLine("fmt.Println(\"unclosed\")")
	// 没有关闭的 ```

	if !r.inCodeBlock {
		t.Error("should still be in code block")
	}
	if len(r.codeLines) != 2 {
		t.Errorf("should have 2 code lines, got %d", len(r.codeLines))
	}
}

func TestMarkdownLineRenderer_Reset(t *testing.T) {
	r := NewMarkdownLineRenderer()

	r.RenderLine("```go")
	r.RenderLine("code")
	if !r.inCodeBlock {
		t.Error("should be in code block")
	}

	r.Reset()
	if r.inCodeBlock {
		t.Error("should not be in code block after reset")
	}
	if r.codeLang != "" {
		t.Error("codeLang should be empty after reset")
	}
	if len(r.codeLines) != 0 {
		t.Error("codeLines should be empty after reset")
	}

	// Reset 后可以正常使用
	result := r.RenderLine("normal text")
	if result != "normal text" {
		t.Errorf("should render normally after reset, got %q", result)
	}
}

func TestMarkdownLineRenderer_MixedContent(t *testing.T) {
	r := NewMarkdownLineRenderer()

	// 普通 Markdown 文本
	r1 := r.RenderLine("# Heading")
	if !strings.Contains(r1, "\033[1m") {
		t.Error("heading should be bold")
	}

	// 进入代码块
	r.RenderLine("```go")
	r.RenderLine("// This is a comment")
	r.RenderLine("var x = 42")
	r.RenderLine("```")

	// 退出代码块后，普通 Markdown 应该正常工作
	r2 := r.RenderLine("- list item")
	if !strings.Contains(r2, "\033[32m") {
		t.Error("list item should be green after code block")
	}
}

func TestMarkdownLine_BackwardCompatible(t *testing.T) {
	// 旧的无状态函数仍然可用
	result := MarkdownLine("```go")
	if !strings.Contains(result, "\033[2m") {
		t.Error("code fence should be dimmed")
	}

	result = MarkdownLine("normal text")
	if result != "normal text" {
		t.Errorf("normal text should be unchanged, got %q", result)
	}

	result = MarkdownLine("# heading")
	if !strings.Contains(result, "\033[1m") {
		t.Error("heading should be bold")
	}
}
