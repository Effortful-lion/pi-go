package tui

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/Effortful-lion/pi-go/agent"
	"github.com/Effortful-lion/pi-go/ai"
)

// ============================================================================
// Mock Agent（用于 ChatUI 测试）
// ============================================================================

// mockAgent 按调用顺序返回预设的 agent.Event 序列。
type mockAgent struct {
	calls   [][]agent.Event
	callNum int
}

func (m *mockAgent) Run(ctx context.Context, userInput string) agent.Stream {
	ch := make(chan agent.Event, 16)
	go func() {
		defer close(ch)
		if m.callNum >= len(m.calls) {
			ch <- agent.Event{Type: agent.EventError, Err: errors.New("no more mock calls")}
			return
		}
		events := m.calls[m.callNum]
		m.callNum++
		for _, e := range events {
			select {
			case <-ctx.Done():
				return
			case ch <- e:
			}
		}
	}()
	return ch
}

// agentInterface ChatUI 依赖的 Agent 接口。
// 定义在这里以避免循环导入，实际使用 *agent.Agent。
type agentInterface interface {
	Run(ctx context.Context, userInput string) agent.Stream
}

var _ agentInterface = (*agent.Agent)(nil)
var _ agentInterface = (*mockAgent)(nil)

// ============================================================================
// 辅助函数
// ============================================================================

// makeTextAgentEvents 创建纯文本的 Agent 事件序列。
func makeTextAgentEvents(text string) []agent.Event {
	return []agent.Event{
		{Type: agent.EventStepStart, Step: 1},
		{Type: agent.EventTextDelta, Text: text, Step: 1},
		{Type: agent.EventStepEnd, Step: 1, Usage: &ai.Usage{TotalTokens: 100}},
		{Type: agent.EventDone, Step: 1},
	}
}

// makeToolCallAgentEvents 创建包含工具调用的 Agent 事件序列（两轮）。
func makeToolCallAgentEvents(toolName, toolResult, finalText string) []agent.Event {
	return []agent.Event{
		// 第一轮：工具调用
		{Type: agent.EventStepStart, Step: 1},
		{Type: agent.EventToolCall, Step: 1, ToolCall: &ai.ToolCall{Name: toolName}},
		{Type: agent.EventToolResult, Step: 1, ToolResult: toolResult},
		{Type: agent.EventStepEnd, Step: 1, Usage: &ai.Usage{TotalTokens: 500}},
		// 第二轮：最终回答
		{Type: agent.EventStepStart, Step: 2},
		{Type: agent.EventTextDelta, Text: finalText, Step: 2},
		{Type: agent.EventStepEnd, Step: 2, Usage: &ai.Usage{TotalTokens: 300}},
		{Type: agent.EventDone, Step: 2},
	}
}

// captureOutput 重定向 stdout 到 buffer，返回恢复函数。
func captureOutput() (buf *bytes.Buffer, restore func()) {
	buf = new(bytes.Buffer)

	// 创建 pipe
	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w

	restore = func() {
		w.Close()
		buf.ReadFrom(r)
		os.Stdout = oldStdout
	}
	return buf, restore
}

// ============================================================================
// 测试
// ============================================================================

func TestChatUI_TextResponse(t *testing.T) {
	ma := &mockAgent{
		calls: [][]agent.Event{
			makeTextAgentEvents("Hello, this is a test response."),
		},
	}

	buf, restore := captureOutput()
	defer restore()

	ui := NewChatUI(&agent.Agent{}, WithWriter(os.Stdout)) // 构造 ChatUI，但我们不调用 Run

	// 直接渲染 mock 的 stream（避免进入 raw mode 循环）
	stream := ma.Run(context.Background(), "Hello")
	ui.renderAgentStream(stream)

	// 读取捕获的输出
	restore()
	output := buf.String()

	// 应该有绿色的 🤖 标记
	if !strings.Contains(output, "Hello") {
		t.Error("output should contain the response text")
	}
}

func TestChatUI_ToolCallResponse(t *testing.T) {
	ma := &mockAgent{
		calls: [][]agent.Event{
			makeToolCallAgentEvents("get_weather", `{"city":"Beijing","temp":25}`, "Beijing is 25°C"),
		},
	}

	buf, restore := captureOutput()
	defer restore()

	ui := NewChatUI(&agent.Agent{})
	stream := ma.Run(context.Background(), "What's the weather?")
	ui.renderAgentStream(stream)

	restore()
	output := buf.String()

	// 应该包含工具调用信息
	if !strings.Contains(output, "get_weather") {
		t.Error("output should contain tool call name")
	}
	// 应该包含最终回答
	if !strings.Contains(output, "Beijing is 25°C") {
		t.Error("output should contain final response")
	}
}

func TestChatUI_ErrorResponse(t *testing.T) {
	ma := &mockAgent{
		calls: [][]agent.Event{
			{
				{Type: agent.EventStepStart, Step: 1},
				{Type: agent.EventError, Step: 1, Err: errors.New("connection failed")},
			},
		},
	}

	buf, restore := captureOutput()
	defer restore()

	ui := NewChatUI(&agent.Agent{})
	stream := ma.Run(context.Background(), "error")
	ui.renderAgentStream(stream)

	restore()
	output := buf.String()

	if !strings.Contains(output, "connection failed") {
		t.Errorf("output should contain error message, got: %q", output)
	}
}

func TestChatUI_UsageDisplay(t *testing.T) {
	ma := &mockAgent{
		calls: [][]agent.Event{
			makeTextAgentEvents("Short reply."),
		},
	}

	buf, restore := captureOutput()
	defer restore()

	ui := NewChatUI(&agent.Agent{})
	stream := ma.Run(context.Background(), "hi")
	ui.renderAgentStream(stream)

	restore()
	output := buf.String()

	// 100 tokens < 1000，应该显示为 "100 tokens"
	if !strings.Contains(output, "100") {
		t.Errorf("output should contain token count, got: %q", output)
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"short", 100, "short"},
		{"hello world", 5, "hello…"},
		{"abc", 3, "abc"},
		{"abcd", 4, "abcd"},
		{"abcde", 4, "abcd…"},
		{"", 10, ""},
	}

	for _, tt := range tests {
		result := truncate(tt.input, tt.maxLen)
		if result != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, result, tt.want)
		}
	}
}
