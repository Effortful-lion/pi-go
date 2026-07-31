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
	"github.com/Effortful-lion/pi-go/lg"
)

var logger = lg.Module("[tui]")

// ChatUI 交互式对话界面，基于终端 I/O 驱动 Agent 对话。
type ChatUI struct {
	agent        *agent.Agent
	prompt       string // 输入提示符
	out          *os.File
	emojiResolver *emoji.Resolver // emoji 主题解析器
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

// NewChatUI 创建对话 UI。
func NewChatUI(ag *agent.Agent, opts ...ChatUIOption) *ChatUI {
	ui := &ChatUI{
		agent:         ag,
		prompt:        "> ",
		out:           os.Stdout,
		emojiResolver: emoji.DefaultResolver, // 使用默认 emoji 主题
	}
	for _, o := range opts {
		o(ui)
	}
	return ui
}

// Run 启动交互式对话循环。阻塞直到用户退出。
//
// UI 风格参考 Codex 卡片风格：
//   - 用户输入通过 LineEditor 实时回显（提示符 > + 输入内容），不再额外显示
//   - AI 回复用浅灰背景色卡片包裹，整体视觉分明
//   - AI 回复内容通过 MarkdownLine 渲染，去掉裸 Markdown 语法（如 ###）
//   - 卡片前后留空行，整体间距美观
func (ui *ChatUI) Run(ctx context.Context) error {
	restore, err := EnterRawMode()
	if err != nil {
		return fmt.Errorf("enter raw mode: %w", err)
	}
	defer restore()

	ui.printWelcome()

	le := NewLineEditor(ui.prompt)

	for {
		// 显示提示符，读取输入（可能多行）
		input, cancelled := le.ReadLine()
		if cancelled {
			ui.printGoodbye()
			return nil
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		// 命令处理
		if strings.HasPrefix(input, "/") {
			ui.handleCommand(ctx, input, le)
			continue
		}

		// 添加到历史
		le.AddHistory(input)

		// 启动 Agent 对话
		stream := ui.agent.Run(ctx, input)

		// 消费事件流并渲染（Codex 卡片风格 + Markdown 行级渲染）
		ui.renderAgentStream(stream)

		// 对话结束，空行分隔（下一个提示符前）
		fmt.Fprintln(ui.out)
	}
}

// handleCommand 处理斜杠命令。
func (ui *ChatUI) handleCommand(ctx context.Context, input string, le *LineEditor) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return
	}
	switch parts[0] {
	case "/clear":
		ClearScreen()
		ui.printWelcome()
	case "/reset":
		ui.agent.Reset()
		le.AddHistory(input)
		fmt.Fprintln(ui.out, Dim("对话已重置"))
	case "/export":
		path := "conversation.md"
		if len(parts) > 1 {
			path = parts[1]
		}
		if err := ui.ExportConversation(path); err != nil {
			fmt.Fprintln(ui.out, Red(fmt.Sprintf("导出失败: %v", err)))
		} else {
			fmt.Fprintln(ui.out, Green(fmt.Sprintf("对话已导出到 %s", path)))
		}
	case "/help":
		ui.printHelp()
	default:
		fmt.Fprintln(ui.out, Dim(fmt.Sprintf("未知命令: %s (使用 /help 查看命令列表)", parts[0])))
	}
	le.AddHistory(input)
	fmt.Fprintln(ui.out)
}

// printHelp 打印命令帮助。
func (ui *ChatUI) printHelp() {
	fmt.Fprintln(ui.out)
	fmt.Fprintln(ui.out, Bold("可用命令:"))
	fmt.Fprintln(ui.out, Dim("  /clear      清屏"))
	fmt.Fprintln(ui.out, Dim("  /reset      重置对话"))
	fmt.Fprintln(ui.out, Dim("  /export     导出对话到文件"))
	fmt.Fprintln(ui.out, Dim("  /help       显示帮助"))
	fmt.Fprintln(ui.out, Dim(""))
	fmt.Fprintln(ui.out, Dim("快捷键:"))
	fmt.Fprintln(ui.out, Dim("  ↑↓          浏览历史记录"))
	fmt.Fprintln(ui.out, Dim("  Alt+Enter   多行输入"))
	fmt.Fprintln(ui.out, Dim("  Tab         路径补全"))
	fmt.Fprintln(ui.out, Dim("  Ctrl+L      清屏"))
	fmt.Fprintln(ui.out, Dim("  Ctrl+C      取消输入"))
	fmt.Fprintln(ui.out, Dim("  Ctrl+D      退出"))
}

// ExportConversation 将对话历史导出为 Markdown 文件。
func (ui *ChatUI) ExportConversation(path string) error {
	messages := ui.agent.Messages()

	userPrefix := ui.emojiResolver.Resolve(emoji.SlotUser)
	assistantPrefix := ui.emojiResolver.Resolve(emoji.SlotAssistant)

	var b strings.Builder
	b.WriteString(fmt.Sprintf("# Pi-Go Agent 对话记录\n\n"))
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

// printWelcome 打印欢迎信息。
func (ui *ChatUI) printWelcome() {
	fmt.Fprintln(ui.out)
	fmt.Fprintln(ui.out, Bold("Pi-Go Agent"), Dim("— AI Coding Assistant"))
	fmt.Fprintln(ui.out, Dim("Type your question below. Ctrl+C to cancel, Ctrl+D to exit."))
	fmt.Fprintln(ui.out)
}

// printGoodbye 打印告别信息。
func (ui *ChatUI) printGoodbye() {
	fmt.Fprintln(ui.out)
	fmt.Fprintln(ui.out, Dim("Goodbye!"))
}

// renderAgentStream 消费 agent.Event 流并渲染到终端。
//
// 渲染策略：
//   - 保持流式输出：每行实时输出，不累积
//   - 无背景色，简洁输出
//   - 文本按行累积，每行通过 MarkdownLine 转换为 ANSI 样式（去掉裸 Markdown 语法）
//   - 第一行前加 🤖 标识
//   - 工具调用/结果以 dim 样式显示
//   - 前后留空行，整体间距美观
func (ui *ChatUI) renderAgentStream(stream agent.Stream) {
	botPrefix := ui.emojiResolver.Resolve(emoji.SlotAssistant)

	// 行缓冲：用于按行处理 Markdown 渲染
	var lineBuf strings.Builder
	firstLine := true

	flushLine := func() {
		if lineBuf.Len() == 0 {
			return
		}
		line := strings.TrimSuffix(lineBuf.String(), "\n")
		lineBuf.Reset()
		rendered := MarkdownLine(line)
		if firstLine {
			fmt.Fprintln(ui.out) // 回复前空行
			fmt.Fprintln(ui.out, Green(botPrefix+" ")+rendered)
			firstLine = false
		} else {
			fmt.Fprintln(ui.out, rendered)
		}
	}

	flushInline := func() {
		// 用于 StepEnd 等不换行的场景
		if lineBuf.Len() == 0 {
			return
		}
		line := strings.TrimSuffix(lineBuf.String(), "\n")
		lineBuf.Reset()
		rendered := MarkdownLine(line)
		if firstLine {
			fmt.Fprintln(ui.out) // 回复前空行
			fmt.Fprintln(ui.out, Green(botPrefix+" ")+rendered)
			firstLine = false
		} else {
			fmt.Fprintln(ui.out, rendered)
		}
	}

	for evt := range stream {
		switch evt.Type {
		case agent.EventTextDelta:
			// 流式文本按行累积，遇到 \n 实时输出
			for _, ch := range evt.Text {
				lineBuf.WriteRune(ch)
				if ch == '\n' {
					flushLine()
				}
			}

		case agent.EventToolCall:
			flushLine()
			toolPrefix := ui.emojiResolver.Resolve(emoji.SlotToolCall)
			fmt.Fprintln(ui.out, Dim(fmt.Sprintf("  %s %s", toolPrefix, evt.ToolCall.Name)))

		case agent.EventToolResult:
			flushLine()
			truncated := truncate(evt.ToolResult, 100)
			resultPrefix := ui.emojiResolver.Resolve(emoji.SlotToolResult)
			fmt.Fprintln(ui.out, Dim(fmt.Sprintf("  %s %s", resultPrefix, truncated)))

		case agent.EventStepEnd:
			flushInline()
			if evt.Usage != nil && evt.Usage.TotalTokens > 0 {
				total := evt.Usage.TotalTokens
				val := fmt.Sprintf("%d", total)
				if total >= 1000 {
					val = fmt.Sprintf("%.1fk", float64(total)/1000)
				}
				fmt.Fprintln(ui.out, Dim(fmt.Sprintf("  [%s tokens]", val)))
			}

		case agent.EventError:
			flushLine()
			errorPrefix := ui.emojiResolver.Resolve(emoji.SlotError)
			fmt.Fprintln(ui.out, Red(fmt.Sprintf("  %s %v", errorPrefix, evt.Err)))
		}
	}

	// 流结束，刷新剩余缓冲
	flushLine()
}

// truncate 截断字符串到 maxLen 个字符。
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "…"
}
