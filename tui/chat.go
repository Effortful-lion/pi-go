//go:build !windows

package tui

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Effortful-lion/pi-go/agent"
	"github.com/Effortful-lion/pi-go/ai"
	"github.com/Effortful-lion/pi-go/emoji"
	tea "github.com/charmbracelet/bubbletea"
)

// ChatUI 交互式对话界面，基于 Bubbletea 驱动 Agent 对话。
type ChatUI struct {
	agent         *agent.Agent
	prompt        string // 输入提示符（保留兼容）
	out           *os.File
	emojiResolver *emoji.Resolver // emoji 主题解析器
	inputStrategy InputStrategy   // 输入交互策略（nil 使用默认 EscSubmitStrategy）
}

// ChatUIOption ChatUI 配置选项。
type ChatUIOption func(*ChatUI)

// WithWriter 设置输出目标（默认 os.Stdout），用于测试。
func WithWriter(w *os.File) ChatUIOption {
	return func(ui *ChatUI) {
		ui.out = w
	}
}

// WithEmojiResolver 设置 emoji 主题解析器
func WithEmojiResolver(resolver *emoji.Resolver) ChatUIOption {
	return func(ui *ChatUI) {
		ui.emojiResolver = resolver
	}
}

// WithInputStrategy 设置输入交互策略。
// 传 nil 使用默认的 EscSubmitStrategy（Enter 换行 + Esc 提交）。
func WithInputStrategy(s InputStrategy) ChatUIOption {
	return func(ui *ChatUI) {
		ui.inputStrategy = s
	}
}

// NewChatUI 创建对话 UI。
func NewChatUI(ag *agent.Agent, opts ...ChatUIOption) *ChatUI {
	ui := &ChatUI{
		agent:         ag,
		prompt:        "> ",
		out:           os.Stdout,
		emojiResolver: emoji.DefaultResolver, // 使用默认 emoji 主题
		inputStrategy: EscSubmitStrategy{},    // 默认策略
	}
	for _, o := range opts {
		o(ui)
	}
	return ui
}

// Run 启动交互式对话循环。阻塞直到用户退出。
//
// 使用 Bubbletea 框架，支持：
//   - 可插拔输入策略（InputStrategy 接口，默认 Enter 换行 + Esc 提交）
//   - Bracketed Paste 防粘贴误触发
//   - 多行文本编辑（Bubbles/Textarea）
//   - 历史记录浏览（↑↓）
//   - 流式 AI 回复渲染
func (ui *ChatUI) Run(ctx context.Context) error {
	model := NewChatModel(ui.agent, ui.emojiResolver, ui.inputStrategy)
	model.ctx = ctx

	p := tea.NewProgram(
		model,
		tea.WithInput(os.Stdin),
		tea.WithOutput(ui.out),
	)

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("bubbletea: %w", err)
	}
	return nil
}

// ExportConversation 将对话历史导出为 Markdown 文件。
func (ui *ChatUI) ExportConversation(path string) error {
	messages := ui.agent.Messages()

	userPrefix := ui.emojiResolver.Resolve(emoji.SlotUser)
	assistantPrefix := ui.emojiResolver.Resolve(emoji.SlotAssistant)

	var b strings.Builder
	b.WriteString("# Pi-Go Agent 对话记录\n\n")
	b.WriteString(fmt.Sprintf("导出时间: %s\n\n", time.Now().Format("2006-01-02 15:04:05")))
	b.WriteString("---\n\n")

	for _, msg := range messages {
		switch msg.Role {
		case ai.RoleSystem:
			b.WriteString("## 系统提示\n\n```\n")
			b.WriteString(msg.Content)
			b.WriteString("\n```\n\n")
		case ai.RoleUser:
			b.WriteString(fmt.Sprintf("## %s 用户\n\n", userPrefix))
			b.WriteString(msg.Content)
			b.WriteString("\n\n")
		case ai.RoleAssistant:
			b.WriteString(fmt.Sprintf("## %s 助手\n\n", assistantPrefix))
			b.WriteString(msg.Content)
			for _, block := range msg.Blocks {
				if block.Type == ai.BlockToolCall && block.ToolCall != nil {
					b.WriteString(fmt.Sprintf("\n\n**工具调用:** `%s`\n\n```json\n%s\n```\n",
						block.ToolCall.Name, block.ToolCall.Arguments))
				}
			}
			b.WriteString("\n")
		case ai.RoleTool:
			b.WriteString("### 工具结果\n\n```\n")
			b.WriteString(msg.Content)
			b.WriteString("\n```\n\n")
		}
	}

	return os.WriteFile(path, []byte(b.String()), 0644)
}

// truncate 截断字符串到 maxLen 个字符。
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "…"
}
