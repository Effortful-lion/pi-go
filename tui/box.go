//go:build !windows

package tui

import (
	"strings"
)

// Box 在终端中用 Unicode 框线字符绘制文本框，类似 Claude Code 风格。
//
// 用法：
//
//	box := NewBox(60)                              // 宽度 60 列
//	box.Line("第一行")                              // 框内行
//	box.Render()                                    // 输出完整框线
//
//	// 链式调用
//	NewBox(60).Line("标题").Gap().Line("内容").Render()
type Box struct {
	width int
	lines []string
	out   strings.Builder
}

// NewBox 创建框线构建器。
// width 为框线内容区宽度（不含左右边框和 padding）。
func NewBox(width int) *Box {
	if width < 20 {
		width = 20
	}
	return &Box{width: width}
}

// Line 添加一行文本到框内。
// 文本超过宽度会自动换行。
func (b *Box) Line(text string) *Box {
	for len(text) > b.width {
		b.lines = append(b.lines, text[:b.width])
		text = text[b.width:]
	}
	b.lines = append(b.lines, text)
	return b
}

// Gap 添加一个空行。
func (b *Box) Gap() *Box {
	b.lines = append(b.lines, "")
	return b
}

// Lines 添加多行文本（按 \n 分割）。
func (b *Box) Lines(text string) *Box {
	for _, line := range strings.Split(text, "\n") {
		b.Line(line)
	}
	return b
}

// Render 返回完整的框线字符串。
// 框线样式：
//
//	┌────────────────┐
//	│ 内容行1        │
//	│ 内容行2        │
//	└────────────────┘
func (b *Box) Render() string {
	b.out.Reset()

	totalWidth := b.width + 2 // +2 为左右各一个空格 padding

	// 上边框
	b.out.WriteString("┌")
	b.out.WriteString(strings.Repeat("─", totalWidth))
	b.out.WriteString("┐\n")

	// 内容行
	for _, line := range b.lines {
		b.out.WriteString("│")
		padding := totalWidth - len(StripANSI(line))
		if padding < 0 {
			padding = 0
		}
		b.out.WriteString(" ")
		b.out.WriteString(line)
		b.out.WriteString(strings.Repeat(" ", padding-1))
		b.out.WriteString(" │\n")
	}

	// 下边框
	b.out.WriteString("└")
	b.out.WriteString(strings.Repeat("─", totalWidth))
	b.out.WriteString("┘")

	return b.out.String()
}

// RenderTo 将框线字符串写入提供的 builder。
func (b *Box) RenderTo(sb *strings.Builder) {
	sb.WriteString(b.Render())
}
