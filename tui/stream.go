//go:build !windows

package tui

import (
	"fmt"
	"strings"

	"github.com/Effortful-lion/pi-go/agent"
	tea "github.com/charmbracelet/bubbletea"
)

// streamMsg 封装 agent.Event 作为 Bubbletea 消息。
type streamMsg agent.Event

// streamDoneMsg 表示 Agent 流结束。
type streamDoneMsg struct{}

// waitForStream 将 agent.Stream (channel) 适配为 Bubbletea Cmd。
// 从 channel 读取下一个事件并包装为 streamMsg；channel 关闭时发送 streamDoneMsg。
func waitForStream(stream agent.Stream) tea.Cmd {
	return func() tea.Msg {
		evt, ok := <-stream
		if !ok {
			return streamDoneMsg{}
		}
		return streamMsg(evt)
	}
}

// handleStream 处理 Agent 流式事件，累积渲染输出。
// 返回 nil cmd 时表示流已结束。
func (m *ChatModel) handleStream(msg streamMsg, stream agent.Stream) (tea.Model, tea.Cmd) {
	evt := agent.Event(msg)
	botPrefix := m.emojiResolver.Resolve(emojiSlotAssistant)

	switch evt.Type {
	case agent.EventTextDelta:
		// 流式文本按行累积，遇到 \n 实时追加到 output
		for _, ch := range evt.Text {
			m.lineBuf.WriteRune(ch)
			if ch == '\n' {
				m.flushLine(botPrefix)
			}
		}

	case agent.EventToolCall:
		m.flushLine(botPrefix)
		toolPrefix := m.emojiResolver.Resolve(emojiSlotToolCall)
		fmt.Fprintf(&m.output, "%s\n", Dim(fmt.Sprintf("  %s %s", toolPrefix, evt.ToolCall.Name)))

	case agent.EventToolResult:
		m.flushLine(botPrefix)
		truncated := truncate(evt.ToolResult, 100)
		resultPrefix := m.emojiResolver.Resolve(emojiSlotToolResult)
		fmt.Fprintf(&m.output, "%s\n", Dim(fmt.Sprintf("  %s %s", resultPrefix, truncated)))

	case agent.EventStepEnd:
		m.flushInline(botPrefix)
		if evt.Usage != nil && evt.Usage.TotalTokens > 0 {
			total := evt.Usage.TotalTokens
			val := fmt.Sprintf("%d", total)
			if total >= 1000 {
				val = fmt.Sprintf("%.1fk", float64(total)/1000)
			}
			fmt.Fprintf(&m.output, "%s\n", Dim(fmt.Sprintf("  [%s tokens]", val)))
		}

	case agent.EventError:
		m.flushLine(botPrefix)
		errorPrefix := m.emojiResolver.Resolve(emojiSlotError)
		fmt.Fprintf(&m.output, "%s\n", Red(fmt.Sprintf("  %s %v", errorPrefix, evt.Err)))
		// 错误后停止流
		m.running = false
		m.stream = nil
		m.textarea.Focus()
		m.textarea.Reset()
		return m, nil

	case agent.EventDone:
		// 流结束：刷新剩余缓冲
		m.flushLine(botPrefix)
		m.running = false
		m.stream = nil
		m.textarea.Focus()
		m.textarea.Reset()
		return m, nil
	}

	// 继续等待下一个事件
	return m, waitForStream(stream)
}

// flushLine 刷新行缓冲到 output，遇到 \n 结尾时输出整行。
func (m *ChatModel) flushLine(botPrefix string) {
	if m.lineBuf.Len() == 0 {
		return
	}
	line := strings.TrimSuffix(m.lineBuf.String(), "\n")
	m.lineBuf.Reset()
	rendered := MarkdownLine(line)
	if m.firstLine {
		m.output.WriteString("\n") // 回复前空行
		fmt.Fprintf(&m.output, "%s\n", Green(botPrefix+" ")+rendered)
		m.firstLine = false
	} else {
		fmt.Fprintln(&m.output, rendered)
	}
}

// flushInline 刷新行缓冲（不换行场景，如 StepEnd）。
func (m *ChatModel) flushInline(botPrefix string) {
	if m.lineBuf.Len() == 0 {
		return
	}
	line := strings.TrimSuffix(m.lineBuf.String(), "\n")
	m.lineBuf.Reset()
	rendered := MarkdownLine(line)
	if m.firstLine {
		m.output.WriteString("\n")
		fmt.Fprintf(&m.output, "%s\n", Green(botPrefix+" ")+rendered)
		m.firstLine = false
	} else {
		fmt.Fprintln(&m.output, rendered)
	}
}

// emoji slot 常量（避免循环依赖，直接定义在此文件）。
const (
	emojiSlotAssistant  = "assistant"
	emojiSlotUser       = "user"
	emojiSlotToolCall   = "tool_call"
	emojiSlotToolResult = "tool_result"
	emojiSlotError      = "error"
)
