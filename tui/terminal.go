//go:build !windows

// Package tui 提供终端 UI 能力——ANSI 颜色/样式、清屏。
//
// 本文件提供终端底层能力：ANSI escape 常量、颜色样式、清屏。
// 交互式输入已迁移至 Bubbletea + Bubbles/Textarea (参见 model.go / chat.go)。
package tui

import (
	"fmt"
	"strings"
)

// ============================================================================
// ANSI Escape 基础
// ============================================================================

// ANSI escape 前缀。
const esc = "\033"

// ============================================================================
// 清屏
// ============================================================================

// ClearScreen 清屏并将光标移到左上角。
func ClearScreen() {
	fmt.Printf("%s[2J", esc)
}

// ============================================================================
// 颜色与样式
// ============================================================================

// Color ANSI 标准色号。
type Color int

const (
	ColorDefault Color = 39
	ColorBlack   Color = 30
	ColorRed     Color = 31
	ColorGreen   Color = 32
	ColorYellow  Color = 33
	ColorBlue    Color = 34
	ColorMagenta Color = 35
	ColorCyan    Color = 36
	ColorWhite   Color = 37
	ColorGray    Color = 90
)

// Style 文本样式。
type Style struct {
	Fg        Color
	Bold      bool
	Dim       bool
	Italic    bool
	Underline bool
}

// StyleText 用指定样式包裹文本，返回包含 ANSI escape 的字符串。
// 用法：fmt.Print(tui.StyleText("hello", tui.Style{Fg: tui.ColorRed, Bold: true}))
func StyleText(text string, s Style) string {
	codes := make([]string, 0, 4)

	if s.Bold {
		codes = append(codes, "1")
	}
	if s.Dim {
		codes = append(codes, "2")
	}
	if s.Italic {
		codes = append(codes, "3")
	}
	if s.Underline {
		codes = append(codes, "4")
	}
	if s.Fg != 0 && s.Fg != ColorDefault {
		codes = append(codes, fmt.Sprintf("%d", s.Fg))
	}

	if len(codes) == 0 {
		return text
	}

	ansi := esc + "["
	for i, c := range codes {
		if i > 0 {
			ansi += ";"
		}
		ansi += c
	}
	ansi += "m"

	return ansi + text + esc + "[0m"
}

// 便捷样式函数
func Bold(text string) string      { return StyleText(text, Style{Bold: true}) }
func Dim(text string) string       { return StyleText(text, Style{Dim: true}) }
func Italic(text string) string    { return StyleText(text, Style{Italic: true}) }
func Underline(text string) string { return StyleText(text, Style{Underline: true}) }
func Red(text string) string       { return StyleText(text, Style{Fg: ColorRed}) }
func Green(text string) string     { return StyleText(text, Style{Fg: ColorGreen}) }
func Blue(text string) string      { return StyleText(text, Style{Fg: ColorBlue}) }
func Cyan(text string) string      { return StyleText(text, Style{Fg: ColorCyan}) }
func Gray(text string) string      { return StyleText(text, Style{Fg: ColorGray}) }
func Yellow(text string) string    { return StyleText(text, Style{Fg: ColorYellow}) }
func Magenta(text string) string   { return StyleText(text, Style{Fg: ColorMagenta}) }

// StripANSI 去除字符串中的 ANSI escape 序列，返回纯文本。
// 用于计算框线宽度等场景。
func StripANSI(s string) string {
	var b strings.Builder
	inEscape := false
	for i := 0; i < len(s); i++ {
		if s[i] == '\033' {
			inEscape = true
			continue
		}
		if inEscape {
			if s[i] == 'm' {
				inEscape = false
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// maxHistory 最大历史记录条数。
const maxHistory = 1000
