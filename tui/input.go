//go:build !windows

package tui

import tea "github.com/charmbracelet/bubbletea"

// InputStrategy 定义输入交互策略：如何区分"换行"与"提交"。
//
// 使用者可以实现此接口来定制 TUI 的输入行为，例如：
//   - Enter 换行 + Esc 提交（默认）
//   - Enter 换行 + Ctrl+Enter 提交
//   - Enter 直接提交（传统单行模式）
//
// 注意：Bracketed Paste（粘贴）由框架层统一处理，不经过 InputStrategy。
type InputStrategy interface {
	// ShouldSubmit 判断当前按键是否应该触发提交。
	// 返回 true 时，ChatModel 会调用 handleSubmit。
	ShouldSubmit(msg tea.KeyMsg, textareaValue string, running bool) bool

	// ShouldInsertNewline 判断当前按键是否应该插入换行。
	// 返回 true 时，ChatModel 会将按键透传给 textarea。
	ShouldInsertNewline(msg tea.KeyMsg, running bool) bool

	// HelpText 返回输入帮助提示文字（显示在输入框下方）。
	HelpText() string
}

// EscSubmitStrategy Enter 换行 + Esc 提交（默认策略）。
type EscSubmitStrategy struct{}

func (s EscSubmitStrategy) ShouldSubmit(msg tea.KeyMsg, textareaValue string, running bool) bool {
	if running {
		return false
	}
	return msg.Type == tea.KeyEsc
}

func (s EscSubmitStrategy) ShouldInsertNewline(msg tea.KeyMsg, running bool) bool {
	if running {
		return false
	}
	return msg.Type == tea.KeyEnter
}

func (s EscSubmitStrategy) HelpText() string {
	return Dim("Enter: newline | Esc: send | Ctrl+C: quit")
}

// 编译期验证 EscSubmitStrategy 实现了 InputStrategy
var _ InputStrategy = EscSubmitStrategy{}
