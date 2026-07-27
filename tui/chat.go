package tui

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/Effortful-lion/pi-go/agent"
	"github.com/Effortful-lion/pi-go/lg"
)

var logger = lg.Module("[tui]")

// ChatUI 交互式对话界面，基于终端 I/O 驱动 Agent 对话。
type ChatUI struct {
	agent  *agent.Agent
	prompt string // 输入提示符
	out    *os.File
}

// ChatUIOption ChatUI 配置选项。
type ChatUIOption func(*ChatUI)

// WithWriter 设置输出目标（默认 os.Stdout），用于测试。
func WithWriter(w *os.File) ChatUIOption {
	return func(ui *ChatUI) {
		ui.out = w
	}
}

// NewChatUI 创建对话 UI。
func NewChatUI(ag *agent.Agent, opts ...ChatUIOption) *ChatUI {
	ui := &ChatUI{
		agent:  ag,
		prompt: "> ",
		out:    os.Stdout,
	}
	for _, o := range opts {
		o(ui)
	}
	return ui
}

// Run 启动交互式对话循环。阻塞直到用户退出。
func (ui *ChatUI) Run(ctx context.Context) error {
	restore, err := EnterRawMode()
	if err != nil {
		return fmt.Errorf("enter raw mode: %w", err)
	}
	defer restore()

	ui.printWelcome()

	for {
		// 显示提示符，读取输入
		fmt.Fprint(ui.out, ui.prompt)
		input, cancelled := EditLine(ui.prompt)
		if cancelled {
			ui.printGoodbye()
			return nil
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		// 回显用户输入
		ui.printUserInput(input)

		// 启动 Agent 对话
		stream := ui.agent.Run(ctx, input)

		// 消费事件流并渲染
		ui.renderAgentStream(stream)

		// 对话结束，空行分隔
		fmt.Fprintln(ui.out)
	}
}

// printWelcome 打印欢迎信息。
func (ui *ChatUI) printWelcome() {
	fmt.Fprintln(ui.out)
	fmt.Fprintln(ui.out, Bold("Pi Agent"), Dim("— AI Coding Assistant"))
	fmt.Fprintln(ui.out, Dim("Type your question below. Ctrl+C to cancel, Ctrl+D to exit."))
	fmt.Fprintln(ui.out)
}

// printGoodbye 打印告别信息。
func (ui *ChatUI) printGoodbye() {
	fmt.Fprintln(ui.out)
	fmt.Fprintln(ui.out, Dim("Goodbye!"))
}

// printUserInput 回显用户输入。
func (ui *ChatUI) printUserInput(input string) {
	fmt.Fprintln(ui.out)
	fmt.Fprint(ui.out, Cyan("💬 "))
	fmt.Fprintln(ui.out, input)
	fmt.Fprintln(ui.out)
}

// renderAgentStream 消费 agent.Event 流并渲染到终端。
func (ui *ChatUI) renderAgentStream(stream agent.Stream) {
	botPrefix := Green("🤖 ")
	fmt.Fprint(ui.out, botPrefix)

	var lineLen int // 当前行已输出的字符数（不含 ANSI）
	firstTextDelta := true

	for evt := range stream {
		switch evt.Type {
		case agent.EventTextDelta:
			// 流式文本增量，直接写入终端
			if firstTextDelta {
				firstTextDelta = false
			}
			fmt.Fprint(ui.out, evt.Text)
			lineLen += len(evt.Text)

		case agent.EventToolCall:
			// 工具调用通知
			if !firstTextDelta {
				fmt.Fprintln(ui.out)
				lineLen = 0
			}
			fmt.Fprint(ui.out, Dim(fmt.Sprintf("[调用工具 %s]", evt.ToolCall.Name)))
			fmt.Fprintln(ui.out)
			lineLen = 0

		case agent.EventToolResult:
			// 工具结果（截断显示）
			truncated := truncate(evt.ToolResult, 100)
			fmt.Fprint(ui.out, Dim(fmt.Sprintf("[工具结果] %s", truncated)))
			fmt.Fprintln(ui.out)
			lineLen = 0

		case agent.EventStepEnd:
			// Usage 统计
			if evt.Usage != nil && evt.Usage.TotalTokens > 0 {
				if lineLen > 0 {
					fmt.Fprint(ui.out, "  ")
				}
				total := evt.Usage.TotalTokens
				suffix := "tokens"
				val := fmt.Sprintf("%d", total)
				if total >= 1000 {
					val = fmt.Sprintf("%.1fk", float64(total)/1000)
				}
				fmt.Fprint(ui.out, Dim(fmt.Sprintf("[%s %s]", val, suffix)))
			}
			fmt.Fprintln(ui.out)
			lineLen = 0

		case agent.EventError:
			fmt.Fprintln(ui.out)
			fmt.Fprintln(ui.out, Red(fmt.Sprintf("✖ %v", evt.Err)))
			lineLen = 0
		}
	}
}

// truncate 截断字符串到 maxLen 个字符。
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "…"
}
